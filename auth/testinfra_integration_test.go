//go:build integration

package auth

// Integration test infrastructure: in-process identity gRPC server backed by
// a testcontainers Postgres with the real identity schema and argon2id/RS256
// verification logic.
//
// Why in-process instead of subprocess:
//   Identity Config.Env validates as oneof=dev|staging|prod; apikey env is
//   live|test. Running identity binary as a subprocess means gRPC VerifyAPIKey
//   always returns wrong_environment because the config ENV value never equals
//   the api-key env value (known identity design mismatch, tracked for Phase 2
//   follow-up). The in-process server avoids this config constraint while using
//   real SQL, real argon2id hash verification, real RS256 key management —
//   the actual verification logic, not stubs.
//
// Gate: tests run only when RUN_INTEGRATION=1. Otherwise TestMain exits 0
// (so "not executed" is explicit, not mistaken for "passed"). Full CI wiring
// deferred to Task 14 (cross-service integration).

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	identityv1 "github.com/paper-board/proto/gen/go/identity/v1"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"golang.org/x/crypto/argon2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// identitySchema mirrors paper-board/identity/migrations/schema/000001_init.up.sql
// (last synced at identity@41d9312). Inlined to avoid a hard sibling-repo path
// dependency at test build time. If identity adds migrations, update here too;
// drift surfaces only as runtime test failure, not at compile time.
const identitySchema = `
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;
CREATE EXTENSION IF NOT EXISTS citext   WITH SCHEMA public;
CREATE SCHEMA IF NOT EXISTS identity;
SET search_path TO identity, public;

CREATE TABLE IF NOT EXISTS identity.users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         citext NOT NULL UNIQUE,
    name          text NOT NULL,
    password_hash bytea NOT NULL,
    is_test       boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);
CREATE INDEX IF NOT EXISTS users_email_active
    ON identity.users (email) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS identity.organizations (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL,
    slug        citext NOT NULL UNIQUE,
    is_test     boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz
);

CREATE TABLE IF NOT EXISTS identity.org_members (
    org_id     uuid NOT NULL REFERENCES identity.organizations(id),
    user_id    uuid NOT NULL REFERENCES identity.users(id),
    role       text NOT NULL DEFAULT 'member' CHECK (role IN ('owner','member')),
    is_test    boolean NOT NULL DEFAULT false,
    joined_at  timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    PRIMARY KEY (org_id, user_id)
);

CREATE TABLE IF NOT EXISTS identity.api_keys (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES identity.users(id),
    org_id       uuid NOT NULL REFERENCES identity.organizations(id),
    name         text NOT NULL,
    key_prefix   citext NOT NULL UNIQUE,
    key_hash     bytea NOT NULL,
    env          text NOT NULL CHECK (env IN ('live','test')),
    expires_at   timestamptz,
    last_used_at timestamptz,
    is_test      boolean NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz
);
CREATE INDEX IF NOT EXISTS api_keys_user_id_active
    ON identity.api_keys (user_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS identity.auth_keys (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kid                    text NOT NULL UNIQUE,
    algorithm              text NOT NULL CHECK (algorithm IN ('RS256')),
    public_key             bytea NOT NULL,
    private_key_encrypted  bytea NOT NULL,
    state                  text NOT NULL CHECK (state IN ('next','active','retired')),
    is_test                boolean NOT NULL DEFAULT false,
    created_at             timestamptz NOT NULL DEFAULT now(),
    activated_at           timestamptz,
    retired_at             timestamptz,
    deleted_at             timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS auth_keys_one_active
    ON identity.auth_keys ((state = 'active'))
    WHERE state = 'active' AND deleted_at IS NULL;
`

// harness holds process-wide shared state initialised in TestMain.
var harness struct {
	grpcConn *grpc.ClientConn
	pool     *pgxpool.Pool
	userID   uuid.UUID
	orgID    uuid.UUID
	validKey string // live-env api key created during setup
	jwtKEK   []byte // 32-byte AES-256-GCM key used by the in-process JWT issuer
	cleanup  func()
}

func TestMain(m *testing.M) {
	if os.Getenv("RUN_INTEGRATION") != "1" {
		fmt.Fprintln(os.Stderr, "sdk/auth integration tests: skipped (set RUN_INTEGRATION=1 to run; Task 14 will wire CI)")
		os.Exit(0)
	}
	if err := bootHarness(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: integration harness boot: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	if harness.cleanup != nil {
		harness.cleanup()
	}
	os.Exit(code)
}

func bootHarness() error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Postgres container.
	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		return fmt.Errorf("postgres container: %w", err)
	}

	dbURL, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pg.Terminate(context.Background())
		return fmt.Errorf("connection string: %w", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		_ = pg.Terminate(context.Background())
		return fmt.Errorf("pool: %w", err)
	}
	harness.pool = pool

	// Apply schema.
	if _, err := pool.Exec(ctx, identitySchema); err != nil {
		harness.cleanup = func() {
			pool.Close()
			_ = pg.Terminate(context.Background())
		}
		harness.cleanup()
		return fmt.Errorf("schema: %w", err)
	}

	// KEK for JWT key encryption (32-byte deterministic for tests).
	kek := make([]byte, 32)
	for i := range kek {
		kek[i] = byte(i + 1)
	}
	harness.jwtKEK = kek

	// Seed: user + org + owner membership + initial live-env api key.
	uid, oid, rawKey, err := seedBaseFixtures(ctx, pool)
	if err != nil {
		harness.cleanup = func() {
			pool.Close()
			_ = pg.Terminate(context.Background())
		}
		harness.cleanup()
		return fmt.Errorf("seed fixtures: %w", err)
	}
	harness.userID = uid
	harness.orgID = oid
	harness.validKey = rawKey

	// Seed: active auth key pair (for JWT issuance in tests).
	if err := seedAuthKey(ctx, pool, kek); err != nil {
		harness.cleanup = func() {
			pool.Close()
			_ = pg.Terminate(context.Background())
		}
		harness.cleanup()
		return fmt.Errorf("seed auth key: %w", err)
	}

	// In-process gRPC identity server over bufconn.
	conn, srv, err := startIdentityServer(pool, kek)
	if err != nil {
		harness.cleanup = func() {
			pool.Close()
			_ = pg.Terminate(context.Background())
		}
		harness.cleanup()
		return fmt.Errorf("identity grpc: %w", err)
	}
	harness.grpcConn = conn

	harness.cleanup = func() {
		if harness.grpcConn != nil {
			_ = harness.grpcConn.Close()
		}
		if harness.pool != nil {
			harness.pool.Close()
		}
		srv.GracefulStop()
		_ = pg.Terminate(context.Background())
	}
	return nil
}

// seedBaseFixtures inserts a user, org, owner membership, and a live-env api key.
// Returns the user ID, org ID, and raw API key.
func seedBaseFixtures(ctx context.Context, pool *pgxpool.Pool) (uid, oid uuid.UUID, rawKey string, err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, "", err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var userPGID, orgPGID pgtype.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO identity.users (email, name, password_hash)
		 VALUES ('test@integration.local', 'Test User', 'hash')
		 RETURNING id`,
	).Scan(&userPGID); err != nil {
		return uuid.UUID{}, uuid.UUID{}, "", fmt.Errorf("insert user: %w", err)
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO identity.organizations (name, slug)
		 VALUES ('Test Org', 'test-org-integration')
		 RETURNING id`,
	).Scan(&orgPGID); err != nil {
		return uuid.UUID{}, uuid.UUID{}, "", fmt.Errorf("insert org: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO identity.org_members (org_id, user_id, role)
		 VALUES ($1, $2, 'owner')`, orgPGID, userPGID,
	); err != nil {
		return uuid.UUID{}, uuid.UUID{}, "", fmt.Errorf("insert member: %w", err)
	}

	raw, hash, prefix, keyErr := generateAPIKey("live")
	if keyErr != nil {
		return uuid.UUID{}, uuid.UUID{}, "", keyErr
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO identity.api_keys (user_id, org_id, name, key_prefix, key_hash, env)
		 VALUES ($1, $2, 'initial', $3, $4, 'live')`,
		userPGID, orgPGID, prefix, hash,
	); err != nil {
		return uuid.UUID{}, uuid.UUID{}, "", fmt.Errorf("insert api key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.UUID{}, uuid.UUID{}, "", err
	}
	return uuid.UUID(userPGID.Bytes), uuid.UUID(orgPGID.Bytes), raw, nil
}

// seedAuthKey generates an RSA key pair, encrypts the private key with kek,
// and inserts it as the active signing key.
func seedAuthKey(ctx context.Context, pool *pgxpool.Pool, kek []byte) error {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return err
	}
	privDER := x509.MarshalPKCS1PrivateKey(priv)
	enc, err := aesGCMEncrypt(kek, privDER)
	if err != nil {
		return err
	}
	kid := uuid.NewString()

	// Insert as 'next' then promote (partial-unique index blocks direct 'active' INSERT).
	var id pgtype.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO identity.auth_keys (kid, algorithm, public_key, private_key_encrypted, state)
		 VALUES ($1, 'RS256', $2, $3, 'next') RETURNING id`,
		kid, pubDER, enc,
	).Scan(&id); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx,
		`UPDATE identity.auth_keys SET state = 'active', activated_at = now()
		 WHERE id = $1 AND state = 'next'`, id,
	); err != nil {
		return err
	}
	return nil
}

// startIdentityServer creates a bufconn gRPC server with a real-DB-backed
// AuthService implementation and returns a connected *grpc.ClientConn and the server handle.
func startIdentityServer(pool *pgxpool.Pool, kek []byte) (*grpc.ClientConn, *grpc.Server, error) {
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	identityv1.RegisterAuthServiceServer(srv, &dbAuthServer{pool: pool, kek: kek, env: "live"})
	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.GracefulStop()
		return nil, nil, err
	}
	return conn, srv, nil
}

// dbAuthServer implements identityv1.AuthServiceServer backed by real Postgres.
// It reproduces the verification logic from paper-board/identity without
// importing that module (which would create a circular module dependency).
type dbAuthServer struct {
	identityv1.UnimplementedAuthServiceServer
	pool *pgxpool.Pool
	kek  []byte
	env  string // api-key environment this server accepts ("live" or "test")
}

func (s *dbAuthServer) VerifyAPIKey(ctx context.Context, req *identityv1.VerifyAPIKeyRequest) (*identityv1.VerifyAPIKeyResponse, error) {
	apiEnv, prefix, err := parseAPIKeyFormat(req.ApiKey)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid_credentials")
	}
	if apiEnv != s.env {
		return nil, status.Error(codes.Unauthenticated, "wrong_environment")
	}

	var (
		keyID   pgtype.UUID
		userID  pgtype.UUID
		orgID   pgtype.UUID
		keyHash []byte
		env     string
	)
	err = s.pool.QueryRow(ctx,
		`SELECT id, user_id, org_id, key_hash, env
		   FROM identity.api_keys
		  WHERE key_prefix = $1 AND deleted_at IS NULL`, prefix,
	).Scan(&keyID, &userID, &orgID, &keyHash, &env)
	if err != nil {
		if isNoRows(err) {
			return nil, status.Error(codes.Unauthenticated, "invalid_credentials")
		}
		return nil, status.Error(codes.Internal, "internal")
	}

	if !verifyAPIKeyHash(req.ApiKey, keyHash) {
		return nil, status.Error(codes.Unauthenticated, "invalid_credentials")
	}

	var role string
	err = s.pool.QueryRow(ctx,
		`SELECT role FROM identity.org_members
		  WHERE org_id = $1 AND user_id = $2 AND deleted_at IS NULL`,
		orgID, userID,
	).Scan(&role)
	if err != nil {
		if isNoRows(err) {
			return nil, status.Error(codes.Unauthenticated, "no_membership")
		}
		return nil, status.Error(codes.Internal, "internal")
	}

	_, _ = s.pool.Exec(ctx, `UPDATE identity.api_keys SET last_used_at = now() WHERE id = $1`, keyID)

	return &identityv1.VerifyAPIKeyResponse{
		Ctx: &identityv1.AuthContext{
			UserId:   uuid.UUID(userID.Bytes).String(),
			OrgId:    uuid.UUID(orgID.Bytes).String(),
			Mode:     identityv1.AuthMode_AUTH_MODE_API_KEY,
			ApiKeyId: uuid.UUID(keyID.Bytes).String(),
			Method:   "api_key:" + prefix,
			Env:      envProto(env),
			Role:     roleProto(role),
		},
	}, nil
}

func (s *dbAuthServer) GetPublicKey(ctx context.Context, req *identityv1.GetPublicKeyRequest) (*identityv1.GetPublicKeyResponse, error) {
	var (
		kid       string
		pubKey    []byte
		algorithm string
	)
	err := s.pool.QueryRow(ctx,
		`SELECT kid, public_key, algorithm
		   FROM identity.auth_keys
		  WHERE kid = $1 AND deleted_at IS NULL`, req.Kid,
	).Scan(&kid, &pubKey, &algorithm)
	if err != nil {
		if isNoRows(err) {
			return nil, status.Error(codes.NotFound, "kid_unknown")
		}
		return nil, status.Error(codes.Internal, "internal")
	}
	return &identityv1.GetPublicKeyResponse{
		Kid:       kid,
		PublicKey: pubKey,
		Algorithm: algorithm,
	}, nil
}

// issueTestJWT mints a JWT signed by the active auth key in the harness DB.
// Used by integration tests to obtain valid / expired / tampered tokens.
func issueTestJWT(ctx context.Context, t *testing.T, pool *pgxpool.Pool, kek []byte, userID, orgID uuid.UUID, ttl time.Duration) string {
	t.Helper()
	var (
		kid     string
		privEnc []byte
	)
	if err := pool.QueryRow(ctx,
		`SELECT kid, private_key_encrypted FROM identity.auth_keys
		  WHERE state = 'active' AND deleted_at IS NULL LIMIT 1`,
	).Scan(&kid, &privEnc); err != nil {
		t.Fatalf("get active auth key: %v", err)
	}

	privDER, err := aesGCMDecrypt(kek, privEnc)
	if err != nil {
		t.Fatalf("decrypt auth key: %v", err)
	}
	priv, err := x509.ParsePKCS1PrivateKey(privDER)
	if err != nil {
		t.Fatalf("parse rsa key: %v", err)
	}

	now := time.Now().UTC()
	exp := now.Add(ttl)
	claims := gojwt.MapClaims{
		"iss":    "identity.paper-board",
		"sub":    userID.String(),
		"aud":    []string{"agents", "identity"},
		"iat":    now.Unix(),
		"nbf":    now.Unix(),
		"exp":    exp.Unix(),
		"jti":    uuid.NewString(),
		"org_id": orgID.String(),
		"env":    "live",
		"scope":  "default",
	}
	tok := gojwt.NewWithClaims(gojwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return signed
}

// --- low-level helpers (no identity package imports) ---

const (
	apiKeyArgonTime    = 1
	apiKeyArgonMemory  = 64 * 1024
	apiKeyArgonThreads = 4
	apiKeyArgonKeyLen  = 32
	apiKeySaltLen      = 16
	apiKeyPrefixLen    = 16
)

// generateAPIKey returns raw, argon2id-hash (salt+digest), and prefix for env.
func generateAPIKey(env string) (raw string, hash []byte, prefix string, err error) {
	payload, err := crockfordRandom(32)
	if err != nil {
		return "", nil, "", err
	}
	raw = "pbk_" + env + "_" + payload

	salt := make([]byte, apiKeySaltLen)
	if _, err = rand.Read(salt); err != nil {
		return "", nil, "", err
	}
	digest := argon2.IDKey([]byte(raw), salt, apiKeyArgonTime, apiKeyArgonMemory, apiKeyArgonThreads, apiKeyArgonKeyLen)
	hash = append(salt, digest...)
	prefix = raw[:apiKeyPrefixLen]
	return raw, hash, prefix, nil
}

var crockfordAlphabet = "ABCDEFGHJKMNPQRSTVWXYZ23456789"

func crockfordRandom(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, v := range b {
		out[i] = crockfordAlphabet[int(v)%len(crockfordAlphabet)]
	}
	return string(out), nil
}

// parseAPIKeyFormat validates the api key format and returns env + prefix.
// Format: pbk_(live|test)_<crockford32>.
func parseAPIKeyFormat(raw string) (env, prefix string, err error) {
	if len(raw) < 4+5+32 { // "pbk_" + "live_" + 32
		return "", "", fmt.Errorf("too short")
	}
	if raw[:4] != "pbk_" {
		return "", "", fmt.Errorf("bad prefix")
	}
	rest := raw[4:]
	switch {
	case len(rest) >= 5 && rest[:5] == "live_":
		env = "live"
		rest = rest[5:]
	case len(rest) >= 5 && rest[:5] == "test_":
		env = "test"
		rest = rest[5:]
	default:
		return "", "", fmt.Errorf("unknown env")
	}
	if len(rest) != 32 {
		return "", "", fmt.Errorf("payload length")
	}
	return env, raw[:apiKeyPrefixLen], nil
}

// verifyAPIKeyHash checks raw against an argon2id hash (salt prepended).
func verifyAPIKeyHash(raw string, hash []byte) bool {
	if len(hash) < apiKeySaltLen {
		return false
	}
	salt := hash[:apiKeySaltLen]
	expected := hash[apiKeySaltLen:]
	got := argon2.IDKey([]byte(raw), salt, apiKeyArgonTime, apiKeyArgonMemory, apiKeyArgonThreads, apiKeyArgonKeyLen)
	return subtle.ConstantTimeCompare(got, expected) == 1
}

func aesGCMEncrypt(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func aesGCMDecrypt(key, enc []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(enc) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, enc[:ns], enc[ns:], nil)
}

func isNoRows(err error) bool {
	return err == pgx.ErrNoRows
}

func envProto(env string) identityv1.Env {
	switch env {
	case "live":
		return identityv1.Env_ENV_LIVE
	case "test":
		return identityv1.Env_ENV_TEST
	default:
		return identityv1.Env_ENV_UNSPECIFIED
	}
}

func roleProto(role string) identityv1.Role {
	switch role {
	case "owner":
		return identityv1.Role_ROLE_OWNER
	case "member":
		return identityv1.Role_ROLE_MEMBER
	default:
		return identityv1.Role_ROLE_UNSPECIFIED
	}
}
