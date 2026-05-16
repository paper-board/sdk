package idempotency

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// MemoryStore is an in-memory Store for testing only.
type MemoryStore struct {
	mu   sync.Mutex
	recs map[string]*Record
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{recs: make(map[string]*Record)}
}

func (m *MemoryStore) Get(ctx context.Context, orgID uuid.UUID, key string) (*Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.recs[orgID.String()+":"+key]; ok {
		return r, nil
	}
	return nil, ErrNotFound
}

func (m *MemoryStore) Put(ctx context.Context, rec *Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recs[rec.OrgID.String()+":"+rec.Key] = rec
	return nil
}
