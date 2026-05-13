package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// verifyJWT parses and validates a JWT string against the Keystore.
// On key parse failure it refreshes the Keystore entry once and retries.
// Errors:
//
//	bad format / sig          → ErrUnauthenticated
//	kid retired after refresh → ErrKeyRetired
//	missing required claims   → ErrUnauthenticated
func verifyJWT(ctx context.Context, store *Keystore, raw string) (AuthCtx, error) {
	parser := gojwt.NewParser(
		gojwt.WithValidMethods([]string{"RS256"}),
	)

	// ParseUnverified extracts header/claims without signature check, so we can read kid.
	unverified, _, err := parser.ParseUnverified(raw, gojwt.MapClaims{})
	if err != nil {
		return AuthCtx{}, fmt.Errorf("%w: malformed token", ErrUnauthenticated)
	}
	kid, _ := unverified.Header["kid"].(string)
	if kid == "" {
		return AuthCtx{}, fmt.Errorf("%w: missing kid", ErrUnauthenticated)
	}

	ac, keyErr, verifyErr := parseAndVerify(ctx, store, parser, raw, kid, false)
	if keyErr != nil {
		// Key lookup or parse failed — refresh once and retry before giving up.
		ac2, keyErr2, verifyErr2 := parseAndVerify(ctx, store, parser, raw, kid, true)
		if keyErr2 != nil {
			return AuthCtx{}, ErrKeyRetired
		}
		if verifyErr2 != nil {
			return AuthCtx{}, ErrKeyRetired
		}
		return ac2, nil
	}
	if verifyErr != nil {
		return AuthCtx{}, verifyErr
	}
	return ac, nil
}

// parseAndVerify fetches the public key and verifies the JWT.
// Returns (result, keyErr, verifyErr) where keyErr is non-nil if key fetch/parse failed
// (safe to retry after Refresh) and verifyErr is non-nil for signature/claim failures (not retryable).
func parseAndVerify(ctx context.Context, store *Keystore, parser *gojwt.Parser, raw, kid string, forceRefresh bool) (AuthCtx, error, error) {
	var (
		pubKey []byte
		err    error
	)
	if forceRefresh {
		pubKey, _, err = store.Refresh(ctx, kid)
	} else {
		pubKey, _, err = store.Get(ctx, kid)
	}
	if err != nil {
		return AuthCtx{}, fmt.Errorf("key lookup: %w", err), nil
	}

	rsaPub, err := parseRSAPublicKey(pubKey)
	if err != nil {
		return AuthCtx{}, fmt.Errorf("key parse: %w", err), nil
	}

	parsed, err := parser.Parse(raw, func(t *gojwt.Token) (any, error) {
		return rsaPub, nil
	})
	if err != nil {
		return AuthCtx{}, nil, fmt.Errorf("%w: %v", ErrUnauthenticated, err)
	}

	mc, ok := parsed.Claims.(gojwt.MapClaims)
	if !ok {
		return AuthCtx{}, nil, ErrUnauthenticated
	}

	sub, _ := mc["sub"].(string)
	orgStr, _ := mc["org_id"].(string)
	env, _ := mc["env"].(string)

	userID, parseErr := uuid.Parse(sub)
	if parseErr != nil {
		return AuthCtx{}, nil, fmt.Errorf("%w: invalid sub", ErrUnauthenticated)
	}
	orgID, parseErr := uuid.Parse(orgStr)
	if parseErr != nil {
		return AuthCtx{}, nil, fmt.Errorf("%w: invalid org_id", ErrUnauthenticated)
	}

	kidUUID, parseErr := uuid.Parse(kid)
	var authKeyID *uuid.UUID
	if parseErr == nil {
		authKeyID = &kidUUID
	}

	return AuthCtx{
		UserID:    userID,
		OrgID:     orgID,
		Mode:      JWT,
		AuthKeyID: authKeyID,
		Method:    "jwt",
		Env:       Env(env),
		Role:      Member, // JWT does not carry role; identity's VerifyJWT RPC provides it on Phase 2+
	}, nil, nil
}

// parseRSAPublicKey accepts either raw PKIX DER bytes or PEM-encoded bytes and returns an RSA public key.
// Identity returns raw PKIX DER bytes per proto contract. We attempt PEM-decode
// first as a defensive fallback for wire formats that pre-encode the DER as PEM
// (none currently do in paper-board's deployment, but cheap insurance).
func parseRSAPublicKey(keyBytes []byte) (*rsa.PublicKey, error) {
	der := keyBytes
	if block, _ := pem.Decode(keyBytes); block != nil {
		der = block.Bytes
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}
	return rsaPub, nil
}
