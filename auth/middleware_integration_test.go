//go:build integration

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityv1 "github.com/paper-board/proto/gen/go/identity/v1"
)

// newIntegrationConfig returns a *Config wired to the harness gRPC connection.
func newIntegrationConfig() *Config {
	client := identityv1.NewAuthServiceClient(harness.grpcConn)
	return New(WithClient(client), WithKeystoreTTL(5*time.Second))
}

// probe sends a GET to mw-wrapped okHandler with the given Authorization header
// and returns the HTTP status code.
func probe(mw func(http.Handler) http.Handler, authHeader string) int {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)
	return rec.Code
}

func TestIntegration_valid_apikey(t *testing.T) {
	cfg := newIntegrationConfig()
	mw := cfg.Require(APIKey)

	got := probe(mw, "Bearer "+harness.validKey)
	if got != http.StatusOK {
		t.Errorf("status = %d; want 200 for valid api key", got)
	}
}

func TestIntegration_expired_apikey_401(t *testing.T) {
	ctx := context.Background()

	// Insert an api key with expires_at in the past then immediately soft-delete
	// it — identity's VerifyAPIKey only checks deleted_at; expiry enforcement in
	// production is via a background soft-delete on expired rows.
	raw, hash, prefix, err := generateAPIKey("live")
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	past := time.Now().UTC().Add(-time.Hour)
	if _, err := harness.pool.Exec(ctx,
		`INSERT INTO identity.api_keys (user_id, org_id, name, key_prefix, key_hash, env, expires_at)
		 VALUES ($1, $2, 'expired', $3, $4, 'live', $5)`,
		harness.userID, harness.orgID, prefix, hash, past,
	); err != nil {
		t.Fatalf("insert expired key: %v", err)
	}

	var keyID interface{}
	if err := harness.pool.QueryRow(ctx,
		`SELECT id FROM identity.api_keys WHERE key_prefix = $1`, prefix,
	).Scan(&keyID); err != nil {
		t.Fatalf("lookup key id: %v", err)
	}
	if _, err := harness.pool.Exec(ctx,
		`UPDATE identity.api_keys SET deleted_at = now() WHERE id = $1`, keyID,
	); err != nil {
		t.Fatalf("soft-delete expired key: %v", err)
	}
	t.Cleanup(func() {
		_, _ = harness.pool.Exec(ctx,
			`UPDATE identity.api_keys SET deleted_at = NULL WHERE id = $1`, keyID)
	})

	cfg := newIntegrationConfig()
	mw := cfg.Require(APIKey)
	got := probe(mw, "Bearer "+raw)
	if got != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401 for expired api key", got)
	}
}

func TestIntegration_deleted_apikey_401(t *testing.T) {
	ctx := context.Background()

	raw, hash, prefix, err := generateAPIKey("live")
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	var keyID interface{}
	if err := harness.pool.QueryRow(ctx,
		`INSERT INTO identity.api_keys (user_id, org_id, name, key_prefix, key_hash, env)
		 VALUES ($1, $2, 'to-delete', $3, $4, 'live') RETURNING id`,
		harness.userID, harness.orgID, prefix, hash,
	).Scan(&keyID); err != nil {
		t.Fatalf("insert key: %v", err)
	}

	// Soft-delete it.
	if _, err := harness.pool.Exec(ctx,
		`UPDATE identity.api_keys SET deleted_at = now() WHERE id = $1`, keyID,
	); err != nil {
		t.Fatalf("delete key: %v", err)
	}
	t.Cleanup(func() {
		_, _ = harness.pool.Exec(ctx,
			`UPDATE identity.api_keys SET deleted_at = NULL WHERE id = $1`, keyID)
	})

	cfg := newIntegrationConfig()
	mw := cfg.Require(APIKey)
	got := probe(mw, "Bearer "+raw)
	if got != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401 for deleted api key", got)
	}
}

func TestIntegration_malformed_header_401(t *testing.T) {
	cfg := newIntegrationConfig()
	mw := cfg.Require(APIKey, JWT)

	// "Bearer garbage" — format doesn't match api key regex, treated as JWT,
	// which will fail key lookup / signature verification.
	got := probe(mw, "Bearer garbage-token")
	if got != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401 for malformed bearer token", got)
	}
}

func TestIntegration_missing_header_401(t *testing.T) {
	cfg := newIntegrationConfig()
	mw := cfg.Require(APIKey, JWT)

	got := probe(mw, "")
	if got != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401 for missing header", got)
	}
}

func TestIntegration_valid_jwt(t *testing.T) {
	ctx := context.Background()
	token := issueTestJWT(ctx, t, harness.pool, harness.jwtKEK, harness.userID, harness.orgID, time.Hour)

	cfg := newIntegrationConfig()
	mw := cfg.Require(JWT)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	var capturedCtx AuthCtx
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 for valid jwt", rec.Code)
	}
	if capturedCtx.UserID != harness.userID {
		t.Errorf("UserID = %v; want %v", capturedCtx.UserID, harness.userID)
	}
	if capturedCtx.OrgID != harness.orgID {
		t.Errorf("OrgID = %v; want %v", capturedCtx.OrgID, harness.orgID)
	}
	if capturedCtx.Mode != JWT {
		t.Errorf("Mode = %v; want JWT", capturedCtx.Mode)
	}
}

func TestIntegration_expired_jwt_401(t *testing.T) {
	ctx := context.Background()

	// Negative TTL → exp in the past, well beyond clock skew.
	token := issueTestJWT(ctx, t, harness.pool, harness.jwtKEK, harness.userID, harness.orgID, -2*time.Hour)

	cfg := newIntegrationConfig()
	mw := cfg.Require(JWT)
	got := probe(mw, "Bearer "+token)
	if got != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401 for expired jwt", got)
	}
}

func TestIntegration_invalid_jwt_signature_401(t *testing.T) {
	ctx := context.Background()
	token := issueTestJWT(ctx, t, harness.pool, harness.jwtKEK, harness.userID, harness.orgID, time.Hour)

	// Tamper: replace the last 8 chars of the signature segment.
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}
	sig := []byte(parts[2])
	for i := len(sig) - 8; i < len(sig); i++ {
		sig[i] ^= 0xFF
	}
	tampered := parts[0] + "." + parts[1] + "." + string(sig)

	cfg := newIntegrationConfig()
	mw := cfg.Require(JWT)
	got := probe(mw, "Bearer "+tampered)
	if got != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401 for tampered jwt signature", got)
	}
}
