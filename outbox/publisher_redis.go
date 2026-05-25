package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type publisherImpl struct {
	pool    *pgxpool.Pool
	cfg     Config
	store   *pgStore
	redis   *redis.Client
	stopCh  chan struct{}
	mu      sync.Mutex
	started bool
}

func newPublisherImpl(pool *pgxpool.Pool, cfg Config) (*publisherImpl, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})
	return &publisherImpl{
		pool:   pool,
		cfg:    cfg,
		store:  &pgStore{pool: pool, schema: cfg.Schema},
		redis:  rdb,
		stopCh: make(chan struct{}),
	}, nil
}

func (p *publisherImpl) Publish(ctx context.Context, tx pgx.Tx, e Event) error {
	return p.store.insertEvent(ctx, tx, e, p.cfg.SourceService)
}

func (p *publisherImpl) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return fmt.Errorf("outbox: already started")
	}
	p.started = true
	p.mu.Unlock()

	drainTicker := time.NewTicker(p.cfg.DrainInterval)
	cleanupTicker := time.NewTicker(p.cfg.CleanupInterval)
	defer drainTicker.Stop()
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-p.stopCh:
			return nil
		case <-drainTicker.C:
			p.drain(ctx)
		case <-cleanupTicker.C:
			p.runCleanup(ctx)
		}
	}
}

func (p *publisherImpl) Stop(_ context.Context) error {
	select {
	case p.stopCh <- struct{}{}:
	default:
	}
	return nil
}

func (p *publisherImpl) drain(ctx context.Context) {
	delivered := 0
	deadPromoted := 0

	err := p.store.fetchAndProcess(ctx, p.cfg.DrainBatchSize, func(tx pgx.Tx, rows []pendingRow) error {
		for _, row := range rows {
			env := buildEnvelope(row, p.cfg.SourceService)
			data, marshalErr := json.Marshal(env)
			if marshalErr != nil {
				p.cfg.Logger.Error("outbox marshal envelope failed",
					"schema", p.cfg.Schema,
					"event_id", row.eventID,
					"event_type", row.eventType,
					"error", marshalErr)
				continue
			}

			xaddErr := p.redis.XAdd(ctx, &redis.XAddArgs{
				Stream: row.stream,
				Values: map[string]any{"data": string(data)},
			}).Err()

			if xaddErr != nil {
				p.cfg.Logger.Warn("outbox publish failed",
					"schema", p.cfg.Schema,
					"event_id", row.eventID,
					"event_type", row.eventType,
					"attempts", row.attempts+1,
					"error", xaddErr)

				if markErr := p.store.markFailedTx(ctx, tx, row.eventID, xaddErr.Error()); markErr != nil {
					p.cfg.Logger.Error("outbox mark-failed error", "event_id", row.eventID, "error", markErr)
				}

				promoted, promErr := p.store.promoteDeadLetterTx(ctx, tx, row.eventID, p.cfg.MaxAttempts)
				if promErr != nil {
					p.cfg.Logger.Error("outbox dead-letter promotion error", "event_id", row.eventID, "error", promErr)
				}
				if promoted {
					deadPromoted++
					p.cfg.Logger.Warn("outbox event dead-lettered",
						"schema", p.cfg.Schema,
						"event_id", row.eventID,
						"event_type", row.eventType,
						"attempts", row.attempts+1,
						"last_error", xaddErr.Error())
				}
				continue
			}

			if markErr := p.store.markDeliveredTx(ctx, tx, row.eventID); markErr != nil {
				p.cfg.Logger.Error("outbox mark-delivered error", "event_id", row.eventID, "error", markErr)
			}
			delivered++
		}
		return nil
	})
	if err != nil {
		p.cfg.Logger.Error("outbox drain error", "schema", p.cfg.Schema, "error", err)
		return
	}

	if delivered > 0 || deadPromoted > 0 {
		pending, _ := p.store.countPending(ctx)
		p.cfg.Logger.Info("outbox drain tick",
			"schema", p.cfg.Schema,
			"pending_count", pending,
			"delivered_this_tick", delivered,
			"dead_promoted_this_tick", deadPromoted)
	}
}

func (p *publisherImpl) runCleanup(ctx context.Context) {
	deliveredPurged, deadPurged, err := p.store.cleanup(ctx, p.cfg.DeliveredTTL, p.cfg.DeadTTL)
	if err != nil {
		p.cfg.Logger.Error("outbox cleanup failed", "schema", p.cfg.Schema, "error", err)
		return
	}
	p.cfg.Logger.Info("outbox cleanup pass",
		"schema", p.cfg.Schema,
		"delivered_purged", deliveredPurged,
		"dead_purged", deadPurged)
}

// envelope is the JSON structure written to Redis Streams.
type envelope struct {
	EventID       string `json:"event_id"`
	EventType     string `json:"event_type"`
	SourceService string `json:"source_service"`
	SchemaVersion int32  `json:"schema_version"`
	Stream        string `json:"stream"`
	Payload       []byte `json:"payload"`
	TraceID       string `json:"trace_id,omitempty"`
	OrgID         string `json:"org_id,omitempty"`
	OccurredAt    string `json:"occurred_at"`
}

func buildEnvelope(row pendingRow, sourceService string) envelope {
	return envelope{
		EventID:       row.eventID.String(),
		EventType:     row.eventType,
		SourceService: sourceService,
		SchemaVersion: row.schemaVersion,
		Stream:        row.stream,
		Payload:       row.payload,
		TraceID:       row.traceID,
		OrgID:         row.orgID,
		OccurredAt:    row.occurredAt.UTC().Format(time.RFC3339Nano),
	}
}
