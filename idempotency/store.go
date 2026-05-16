package idempotency

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Record captures a server response keyed by (org_id, key).
type Record struct {
	OrgID           uuid.UUID
	Key             string
	RequestHash     string
	ResponseStatus  int
	ResponseBody    []byte
	ResponseHeaders map[string]string
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

// Store persists idempotency records per-tenant.
type Store interface {
	Get(ctx context.Context, orgID uuid.UUID, key string) (*Record, error)
	Put(ctx context.Context, rec *Record) error
}

// ErrNotFound is returned by Store.Get when no record exists for the key.
var ErrNotFound = errors.New("idempotency: record not found")
