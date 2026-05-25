package outbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgStore struct {
	pool   *pgxpool.Pool
	schema string
}

type pendingRow struct {
	eventID       uuid.UUID
	eventType     string
	sourceService string
	schemaVersion int32
	stream        string
	payload       []byte
	traceID       string
	orgID         string
	occurredAt    time.Time
	attempts      int
}

func (s *pgStore) insertEvent(ctx context.Context, tx pgx.Tx, e Event, sourceService string) error {
	q := fmt.Sprintf(`
		INSERT INTO %s.outbox_events
			(event_id, event_type, source_service, schema_version, stream,
			 payload, trace_id, org_id, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, s.schema)

	eventID := e.EventID
	if eventID == uuid.Nil {
		eventID = uuid.New()
	}

	var orgID *string
	if e.OrgID != "" {
		orgID = &e.OrgID
	}
	var traceID *string
	if e.TraceID != "" {
		traceID = &e.TraceID
	}

	_, err := tx.Exec(ctx, q,
		eventID, e.EventType, sourceService, e.SchemaVersion, e.Stream,
		e.Payload, traceID, orgID, e.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("outbox insert: %w", err)
	}
	return nil
}

// backoffCaseExpr builds the CASE LEAST(attempts, N-1) ... END fragment from
// the configured schedule so the DB uses the same intervals as Config.BackoffSchedule.
func backoffCaseExpr(schedule []time.Duration) string {
	var sb strings.Builder
	sb.WriteString("CASE LEAST(attempts, ")
	fmt.Fprintf(&sb, "%d)", len(schedule)-1)
	for i, d := range schedule {
		fmt.Fprintf(&sb, " WHEN %d THEN INTERVAL '%dms'", i, d.Milliseconds())
	}
	sb.WriteString(" END")
	return sb.String()
}

// fetchAndProcess claims a batch of pending rows in a transaction, calls
// processFn for each, and commits or rolls back. processFn must not hold
// the transaction after returning.
func (s *pgStore) fetchAndProcess(ctx context.Context, batchSize int, schedule []time.Duration, processFn func(tx pgx.Tx, rows []pendingRow) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("outbox begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	q := fmt.Sprintf(`
		SELECT event_id, event_type, source_service, schema_version, stream,
		       payload, trace_id, org_id, occurred_at, attempts
		  FROM %s.outbox_events
		 WHERE status = 'pending'
		   AND (last_attempt_at IS NULL
		        OR last_attempt_at < NOW() - (%s))
		 ORDER BY occurred_at, event_id
		 LIMIT $1
		   FOR UPDATE SKIP LOCKED
	`, s.schema, backoffCaseExpr(schedule))

	pgxRows, err := tx.Query(ctx, q, batchSize)
	if err != nil {
		return fmt.Errorf("outbox fetch pending: %w", err)
	}

	var result []pendingRow
	for pgxRows.Next() {
		var r pendingRow
		var traceID *string
		var orgID *string
		if scanErr := pgxRows.Scan(
			&r.eventID, &r.eventType, &r.sourceService, &r.schemaVersion, &r.stream,
			&r.payload, &traceID, &orgID, &r.occurredAt, &r.attempts,
		); scanErr != nil {
			pgxRows.Close()
			return fmt.Errorf("outbox scan: %w", scanErr)
		}
		if traceID != nil {
			r.traceID = *traceID
		}
		if orgID != nil {
			r.orgID = *orgID
		}
		result = append(result, r)
	}
	pgxRows.Close()
	if err := pgxRows.Err(); err != nil {
		return fmt.Errorf("outbox rows err: %w", err)
	}

	if len(result) == 0 {
		return tx.Commit(ctx)
	}

	if err := processFn(tx, result); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *pgStore) markDeliveredTx(ctx context.Context, tx pgx.Tx, eventID uuid.UUID) error {
	q := fmt.Sprintf(`
		UPDATE %s.outbox_events
		   SET status = 'delivered', delivered_at = NOW()
		 WHERE event_id = $1
	`, s.schema)
	_, err := tx.Exec(ctx, q, eventID)
	if err != nil {
		return fmt.Errorf("outbox mark delivered: %w", err)
	}
	return nil
}

func (s *pgStore) markFailedTx(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, lastErr string) error {
	q := fmt.Sprintf(`
		UPDATE %s.outbox_events
		   SET attempts = attempts + 1,
		       last_error = $2,
		       last_attempt_at = NOW()
		 WHERE event_id = $1
	`, s.schema)
	_, err := tx.Exec(ctx, q, eventID, lastErr)
	if err != nil {
		return fmt.Errorf("outbox mark failed: %w", err)
	}
	return nil
}

func (s *pgStore) promoteDeadLetterTx(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, maxAttempts int) (bool, error) {
	q := fmt.Sprintf(`
		UPDATE %s.outbox_events
		   SET status = 'dead'
		 WHERE event_id = $1 AND attempts >= $2 AND status = 'pending'
	`, s.schema)
	tag, err := tx.Exec(ctx, q, eventID, maxAttempts)
	if err != nil {
		return false, fmt.Errorf("outbox dead-letter: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (s *pgStore) cleanup(ctx context.Context, deliveredTTL, deadTTL time.Duration) (int64, int64, error) {
	qDelivered := fmt.Sprintf(`
		DELETE FROM %s.outbox_events
		 WHERE status = 'delivered' AND delivered_at < NOW() - $1::interval
	`, s.schema)
	tagDel, err := s.pool.Exec(ctx, qDelivered, deliveredTTL)
	if err != nil {
		return 0, 0, fmt.Errorf("outbox cleanup delivered: %w", err)
	}

	qDead := fmt.Sprintf(`
		DELETE FROM %s.outbox_events
		 WHERE status = 'dead' AND last_attempt_at < NOW() - $1::interval
	`, s.schema)
	tagDead, err := s.pool.Exec(ctx, qDead, deadTTL)
	if err != nil {
		return tagDel.RowsAffected(), 0, fmt.Errorf("outbox cleanup dead: %w", err)
	}

	return tagDel.RowsAffected(), tagDead.RowsAffected(), nil
}

func (s *pgStore) countPending(ctx context.Context) (int64, error) {
	q := fmt.Sprintf(`SELECT COUNT(*) FROM %s.outbox_events WHERE status = 'pending'`, s.schema)
	var n int64
	if err := s.pool.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("outbox count pending: %w", err)
	}
	return n, nil
}
