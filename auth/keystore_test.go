package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	identityv1 "github.com/paper-board/proto/gen/go/identity/v1"
	"github.com/paper-board/sdk/auth/mock"
	"google.golang.org/grpc"
)

func TestKeystore_caches_within_ttl(t *testing.T) {
	callCount := 0
	mc := &mock.AuthClient{
		GetPublicKeyFn: func(_ context.Context, req *identityv1.GetPublicKeyRequest, _ ...grpc.CallOption) (*identityv1.GetPublicKeyResponse, error) {
			callCount++
			return &identityv1.GetPublicKeyResponse{
				Kid:       req.Kid,
				PublicKey: []byte("my-pub-key"),
				Algorithm: "RS256",
			}, nil
		},
	}

	ks := NewKeystoreWithTTL(mc, 5*time.Minute)

	// First call — fetches.
	pub1, algo1, err := ks.Get(context.Background(), "kid1")
	if err != nil {
		t.Fatal(err)
	}
	if string(pub1) != "my-pub-key" {
		t.Errorf("got %s; want my-pub-key", pub1)
	}
	if algo1 != "RS256" {
		t.Errorf("got algo %s; want RS256", algo1)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call; got %d", callCount)
	}

	// Second call within TTL — cache hit, no fetch.
	_, _, err = ks.Get(context.Background(), "kid1")
	if err != nil {
		t.Fatal(err)
	}
	if callCount != 1 {
		t.Errorf("expected still 1 call after cache hit; got %d", callCount)
	}
}

func TestKeystore_refresh_bypasses_cache(t *testing.T) {
	callCount := 0
	mc := &mock.AuthClient{
		GetPublicKeyFn: func(_ context.Context, req *identityv1.GetPublicKeyRequest, _ ...grpc.CallOption) (*identityv1.GetPublicKeyResponse, error) {
			callCount++
			return &identityv1.GetPublicKeyResponse{Kid: req.Kid, PublicKey: []byte("refreshed"), Algorithm: "RS256"}, nil
		},
	}

	ks := NewKeystore(mc)

	// Populate cache.
	_, _, _ = ks.Get(context.Background(), "kid-x")
	if callCount != 1 {
		t.Fatalf("expected 1 call after Get; got %d", callCount)
	}

	// Refresh should call identity again even if cache is hot.
	pub, _, err := ks.Refresh(context.Background(), "kid-x")
	if err != nil {
		t.Fatal(err)
	}
	if string(pub) != "refreshed" {
		t.Errorf("got %s; want refreshed", pub)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls after Refresh; got %d", callCount)
	}
}

func TestKeystore_expired_entry_refetches(t *testing.T) {
	callCount := 0
	mc := &mock.AuthClient{
		GetPublicKeyFn: func(_ context.Context, req *identityv1.GetPublicKeyRequest, _ ...grpc.CallOption) (*identityv1.GetPublicKeyResponse, error) {
			callCount++
			return &identityv1.GetPublicKeyResponse{Kid: req.Kid, PublicKey: []byte("pem"), Algorithm: "RS256"}, nil
		},
	}

	// 1 nanosecond TTL — expires immediately.
	ks := NewKeystoreWithTTL(mc, time.Nanosecond)

	_, _, _ = ks.Get(context.Background(), "kid-y")
	time.Sleep(2 * time.Millisecond) // ensure expiry
	_, _, _ = ks.Get(context.Background(), "kid-y")

	if callCount != 2 {
		t.Errorf("expected 2 calls (expired); got %d", callCount)
	}
}

func TestKeystore_identity_error_propagates(t *testing.T) {
	mc := &mock.AuthClient{
		GetPublicKeyFn: func(_ context.Context, _ *identityv1.GetPublicKeyRequest, _ ...grpc.CallOption) (*identityv1.GetPublicKeyResponse, error) {
			return nil, errors.New("identity down")
		},
	}
	ks := NewKeystore(mc)
	_, _, err := ks.Get(context.Background(), "kid-z")
	if err == nil {
		t.Fatal("expected error; got nil")
	}
}
