package inbox

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgInboxStore struct {
	pool   *pgxpool.Pool
	schema string
}

func (s *pgInboxStore) exists(ctx context.Context, eventID uuid.UUID) (bool, error) {
	q := fmt.Sprintf(
		`SELECT 1 FROM %s.processed_events WHERE event_id = $1 LIMIT 1`,
		s.schema,
	)
	var dummy int
	err := s.pool.QueryRow(ctx, q, eventID).Scan(&dummy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("inbox exists: %w", err)
	}
	return true, nil
}

func (s *pgInboxStore) mark(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, eventType, subject string) error {
	q := fmt.Sprintf(`
		INSERT INTO %s.processed_events (event_id, event_type, subject)
		VALUES ($1, $2, $3)
		ON CONFLICT (event_id) DO NOTHING
	`, s.schema)
	_, err := tx.Exec(ctx, q, eventID, eventType, subject)
	if err != nil {
		return fmt.Errorf("inbox mark: %w", err)
	}
	return nil
}
