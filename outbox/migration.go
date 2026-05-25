package outbox

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewMigrationHelper returns an idempotent function that creates the
// outbox_events table in the given schema. Safe to call multiple times
// (all DDL uses IF NOT EXISTS guards).
func NewMigrationHelper(schema string) func(ctx context.Context, pool *pgxpool.Pool) error {
	return func(ctx context.Context, pool *pgxpool.Pool) error {
		ddl := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.outbox_events (
    event_id        UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type      TEXT         NOT NULL,
    source_service  TEXT         NOT NULL,
    schema_version  INT          NOT NULL DEFAULT 1,
    stream          TEXT         NOT NULL,
    payload         JSONB        NOT NULL,
    trace_id        TEXT,
    org_id          UUID,
    occurred_at     TIMESTAMPTZ  NOT NULL,
    status          TEXT         NOT NULL DEFAULT 'pending',
    attempts        INT          NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ,
    last_error      TEXT,
    delivered_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS outbox_events_pending
    ON %s.outbox_events (status, occurred_at, event_id)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS outbox_events_delivered
    ON %s.outbox_events (delivered_at)
    WHERE status = 'delivered';

CREATE INDEX IF NOT EXISTS outbox_events_dead
    ON %s.outbox_events (last_attempt_at)
    WHERE status = 'dead';
`, schema, schema, schema, schema)

		_, err := pool.Exec(ctx, ddl)
		if err != nil {
			return fmt.Errorf("outbox migration (%s): %w", schema, err)
		}
		return nil
	}
}
