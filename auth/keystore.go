package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	identityv1 "github.com/paper-board/proto/gen/go/identity/v1"
)

// Keystore caches GetPublicKey responses with TTL. Concurrent-safe.
type Keystore struct {
	client identityv1.AuthServiceClient
	ttl    time.Duration
	mu     sync.RWMutex
	cache  map[string]keyEntry
}

type keyEntry struct {
	pubKey  []byte
	algo    string
	expires time.Time
}

// NewKeystore returns a Keystore with the given client and default 5min TTL.
func NewKeystore(client identityv1.AuthServiceClient) *Keystore {
	return &Keystore{
		client: client,
		ttl:    5 * time.Minute,
		cache:  make(map[string]keyEntry),
	}
}

// NewKeystoreWithTTL returns a Keystore with a custom TTL.
func NewKeystoreWithTTL(client identityv1.AuthServiceClient, ttl time.Duration) *Keystore {
	return &Keystore{
		client: client,
		ttl:    ttl,
		cache:  make(map[string]keyEntry),
	}
}

// Get returns the cached public key for kid, fetching from identity on miss or expiry.
func (k *Keystore) Get(ctx context.Context, kid string) ([]byte, string, error) {
	k.mu.RLock()
	if e, ok := k.cache[kid]; ok && time.Now().Before(e.expires) {
		k.mu.RUnlock()
		return e.pubKey, e.algo, nil
	}
	k.mu.RUnlock()
	return k.refresh(ctx, kid)
}

// Refresh forces a re-fetch from identity, bypassing the TTL.
func (k *Keystore) Refresh(ctx context.Context, kid string) ([]byte, string, error) {
	return k.refresh(ctx, kid)
}

func (k *Keystore) refresh(ctx context.Context, kid string) ([]byte, string, error) {
	resp, err := k.client.GetPublicKey(ctx, &identityv1.GetPublicKeyRequest{Kid: kid})
	if err != nil {
		return nil, "", fmt.Errorf("auth: keystore refresh kid=%s: %w", kid, err)
	}
	k.mu.Lock()
	k.cache[kid] = keyEntry{
		pubKey:  resp.PublicKey,
		algo:    resp.Algorithm,
		expires: time.Now().Add(k.ttl),
	}
	k.mu.Unlock()
	return resp.PublicKey, resp.Algorithm, nil
}
