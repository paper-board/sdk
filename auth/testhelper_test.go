package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// testRSAKey generates an RSA key pair for testing.
func testRSAKey(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	return priv, pubPEM
}

// signJWT signs a JWT with the given key, kid, and claims.
func signJWT(t *testing.T, priv *rsa.PrivateKey, kid string, userID, orgID uuid.UUID, env string, ttl time.Duration) string {
	t.Helper()
	claims := gojwt.MapClaims{
		"sub":    userID.String(),
		"org_id": orgID.String(),
		"env":    env,
		"exp":    time.Now().Add(ttl).Unix(),
		"iat":    time.Now().Unix(),
		"jti":    uuid.New().String(),
	}
	tok := gojwt.NewWithClaims(gojwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return signed
}
