package inbox

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Inbox tracks processed event IDs to prevent duplicate processing.
type Inbox interface {
	// TryMark atomically records that eventID was processed inside the caller's
	// transaction. Returns claimed=true when this call inserted the row (i.e. the
	// event had not been processed before). Returns claimed=false when another
	// handler already processed the same event (ON CONFLICT DO NOTHING path).
	//
	// Callers MUST skip business-state mutations when claimed=false to prevent
	// duplicate side effects under concurrent delivery.
	TryMark(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, eventType, subject string) (claimed bool, err error)

	// Exists returns true if eventID has already been processed.
	// Non-transactional read for observability/monitoring; use TryMark for
	// deduplication inside a handler transaction.
	Exists(ctx context.Context, eventID uuid.UUID) (bool, error)
}

// New returns an Inbox backed by the given pool for the named schema.
// Panics on nil pool or invalid schema identifier.
func New(pool *pgxpool.Pool, schema string) Inbox {
	if pool == nil {
		panic("inbox: nil pgx pool")
	}
	if err := validateSchemaIdent(schema); err != nil {
		panic(err)
	}
	return &pgInbox{store: &pgInboxStore{pool: pool, schema: schema}}
}

// NewMigrationHelper returns an idempotent function that creates the
// processed_events table in the given schema.
// Panics on invalid schema identifier.
func NewMigrationHelper(schema string) func(ctx context.Context, pool *pgxpool.Pool) error {
	if err := validateSchemaIdent(schema); err != nil {
		panic(err)
	}
	return newMigrationHelper(schema)
}

type pgInbox struct {
	store *pgInboxStore
}

func (i *pgInbox) TryMark(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, eventType, subject string) (bool, error) {
	return i.store.tryMark(ctx, tx, eventID, eventType, subject)
}

func (i *pgInbox) Exists(ctx context.Context, eventID uuid.UUID) (bool, error) {
	return i.store.exists(ctx, eventID)
}
