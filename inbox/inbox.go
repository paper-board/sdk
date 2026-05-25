package inbox

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Inbox tracks processed event IDs to prevent duplicate processing.
type Inbox interface {
	// Exists returns true if eventID has already been processed.
	Exists(ctx context.Context, eventID uuid.UUID) (bool, error)

	// Mark records that eventID was processed.
	// Idempotent: re-marking the same id is a no-op (uses ON CONFLICT DO NOTHING).
	// Caller passes their own tx so the mark + business state mutation are atomic.
	Mark(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, eventType, subject string) error
}

// New returns an Inbox backed by the given pool for the named schema.
func New(pool *pgxpool.Pool, schema string) Inbox {
	return &pgInbox{store: &pgInboxStore{pool: pool, schema: schema}}
}

// NewMigrationHelper returns an idempotent function that creates the
// processed_events table in the given schema.
func NewMigrationHelper(schema string) func(ctx context.Context, pool *pgxpool.Pool) error {
	return newMigrationHelper(schema)
}

type pgInbox struct {
	store *pgInboxStore
}

func (i *pgInbox) Exists(ctx context.Context, eventID uuid.UUID) (bool, error) {
	return i.store.exists(ctx, eventID)
}

func (i *pgInbox) Mark(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, eventType, subject string) error {
	return i.store.mark(ctx, tx, eventID, eventType, subject)
}
