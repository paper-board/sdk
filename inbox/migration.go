package inbox

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func newMigrationHelper(schema string) func(ctx context.Context, pool *pgxpool.Pool) error {
	return func(ctx context.Context, pool *pgxpool.Pool) error {
		ddl := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.processed_events (
    event_id      UUID         PRIMARY KEY,
    event_type    TEXT         NOT NULL,
    subject       TEXT         NOT NULL,
    processed_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS processed_events_cleanup
    ON %s.processed_events (processed_at);
`, schema, schema)

		_, err := pool.Exec(ctx, ddl)
		if err != nil {
			return fmt.Errorf("inbox migration (%s): %w", schema, err)
		}
		return nil
	}
}
