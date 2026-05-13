package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	identityv1 "github.com/paper-board/proto/gen/go/identity/v1"
	"github.com/paper-board/sdk/auth/mock"
	"google.golang.org/grpc"
)

const (
	testAPIKey = "pbk_live_ABCDEFGHJKMNPQRSTVWXYZ23456789AB"
)

// okHandler records that it was called.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func newAPIKeyMock(userID, orgID, keyID uuid.UUID, env identityv1.Env, role identityv1.Role) *mock.AuthClient {
	return &mock.AuthClient{
		VerifyAPIKeyFn: func(_ context.Context, _ *identityv1.VerifyAPIKeyRequest, _ ...grpc.CallOption) (*identityv1.VerifyAPIKeyResponse, error) {
			return &identityv1.VerifyAPIKeyResponse{
				Ctx: &identityv1.AuthContext{
					UserId:   userID.String(),
					OrgId:    orgID.String(),
					ApiKeyId: keyID.String(),
					Env:      env,
					Role:     role,
				},
			}, nil
		},
	}
}

func TestRequire_no_header_401(t *testing.T) {
	cfg := New(WithClient(&mock.AuthClient{}))
	mw := cfg.Require(JWT, APIKey)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", rec.Code)
	}
}

func TestRequire_non_bearer_scheme_401(t *testing.T) {
	cfg := New(WithClient(&mock.AuthClient{}))
	mw := cfg.Require(JWT, APIKey)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	mw(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", rec.Code)
	}
}

func TestRequire_apikey_format_routes_to_apikey_verify(t *testing.T) {
	uid := uuid.New()
	oid := uuid.New()
	kid := uuid.New()

	mc := newAPIKeyMock(uid, oid, kid, identityv1.Env_ENV_LIVE, identityv1.Role_ROLE_MEMBER)
	cfg := New(WithClient(mc))
	mw := cfg.Require(APIKey)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rec := httptest.NewRecorder()

	var capturedCtx AuthCtx
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rec.Code)
	}
	if capturedCtx.Mode != APIKey {
		t.Errorf("Mode = %v; want APIKey", capturedCtx.Mode)
	}
	if capturedCtx.UserID != uid {
		t.Errorf("UserID = %v; want %v", capturedCtx.UserID, uid)
	}
}

func TestRequire_jwt_format_routes_to_jwt_verify(t *testing.T) {
	priv, pubPEM := testRSAKey(t)
	kid := uuid.New().String()
	userID := uuid.New()
	orgID := uuid.New()

	mc := &mock.AuthClient{
		GetPublicKeyFn: func(_ context.Context, req *identityv1.GetPublicKeyRequest, _ ...grpc.CallOption) (*identityv1.GetPublicKeyResponse, error) {
			return &identityv1.GetPublicKeyResponse{Kid: req.Kid, PublicKey: pubPEM, Algorithm: "RS256"}, nil
		},
	}
	cfg := New(WithClient(mc))
	mw := cfg.Require(JWT)

	raw := signJWT(t, priv, kid, userID, orgID, "live", time.Hour)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()

	var capturedCtx AuthCtx
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx, _ = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rec.Code)
	}
	if capturedCtx.Mode != JWT {
		t.Errorf("Mode = %v; want JWT", capturedCtx.Mode)
	}
	if capturedCtx.UserID != userID {
		t.Errorf("UserID = %v; want %v", capturedCtx.UserID, userID)
	}
}

func TestRequire_mode_mismatch_401(t *testing.T) {
	// Route accepts only JWT but request has API key format.
	cfg := New(WithClient(&mock.AuthClient{}))
	mw := cfg.Require(JWT) // only JWT allowed

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey) // APIKey format
	rec := httptest.NewRecorder()
	mw(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401 for mode mismatch", rec.Code)
	}
}

func TestRequire_apikey_verify_failure_401(t *testing.T) {
	mc := &mock.AuthClient{
		VerifyAPIKeyFn: func(_ context.Context, _ *identityv1.VerifyAPIKeyRequest, _ ...grpc.CallOption) (*identityv1.VerifyAPIKeyResponse, error) {
			return nil, errors.New("bad key")
		},
	}
	cfg := New(WithClient(mc))
	mw := cfg.Require(APIKey)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rec := httptest.NewRecorder()
	mw(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401", rec.Code)
	}
}

func TestRequireRole_owner_passes(t *testing.T) {
	uid := uuid.New()
	oid := uuid.New()
	kid := uuid.New()

	mc := newAPIKeyMock(uid, oid, kid, identityv1.Env_ENV_LIVE, identityv1.Role_ROLE_OWNER)
	cfg := New(WithClient(mc))
	chain := cfg.Require(APIKey)(cfg.RequireRole(Owner)(okHandler))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want 200 for owner", rec.Code)
	}
}

func TestRequireRole_member_denied_403(t *testing.T) {
	uid := uuid.New()
	oid := uuid.New()
	kid := uuid.New()

	mc := newAPIKeyMock(uid, oid, kid, identityv1.Env_ENV_LIVE, identityv1.Role_ROLE_MEMBER)
	cfg := New(WithClient(mc))
	chain := cfg.Require(APIKey)(cfg.RequireRole(Owner)(okHandler))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want 403 for member on owner-only route", rec.Code)
	}
}

func TestRequireRole_no_auth_ctx_401(t *testing.T) {
	cfg := New(WithClient(&mock.AuthClient{}))
	mw := cfg.RequireRole(Owner)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want 401 when no auth ctx", rec.Code)
	}
}

func TestRequire_both_modes_accepted(t *testing.T) {
	// JWT mode.
	priv, pubPEM := testRSAKey(t)
	kid := uuid.New().String()
	userID := uuid.New()
	orgID := uuid.New()

	mc := &mock.AuthClient{
		GetPublicKeyFn: func(_ context.Context, req *identityv1.GetPublicKeyRequest, _ ...grpc.CallOption) (*identityv1.GetPublicKeyResponse, error) {
			return &identityv1.GetPublicKeyResponse{Kid: req.Kid, PublicKey: pubPEM, Algorithm: "RS256"}, nil
		},
	}
	cfg := New(WithClient(mc))
	mw := cfg.Require(JWT, APIKey)

	raw := signJWT(t, priv, kid, userID, orgID, "live", time.Hour)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	mw(okHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("JWT with both modes accepted: status = %d; want 200", rec.Code)
	}
}

func TestNew_with_keystore_ttl(t *testing.T) {
	mc := &mock.AuthClient{}
	cfg := New(WithClient(mc), WithKeystoreTTL(10*time.Minute))
	if cfg.store == nil {
		t.Error("store should not be nil after New with WithClient + WithKeystoreTTL")
	}
	if cfg.store.ttl != 10*time.Minute {
		t.Errorf("store.ttl = %v; want 10m", cfg.store.ttl)
	}
}

func TestNew_with_custom_keystore(t *testing.T) {
	mc := &mock.AuthClient{}
	customStore := NewKeystore(mc)
	cfg := New(WithKeystore(customStore))
	if cfg.store != customStore {
		t.Error("WithKeystore did not inject custom store")
	}
}

func TestWriteAuthErr_escapes_quotes(t *testing.T) {
	rec := httptest.NewRecorder()
	writeAuthErr(rec, http.StatusUnauthorized, `contains "quoted" value`)

	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("body is not valid JSON: %v — body: %s", err, rec.Body.String())
	}
	want := `contains "quoted" value`
	if parsed.Error.Message != want {
		t.Errorf("message = %q; want %q", parsed.Error.Message, want)
	}
}
