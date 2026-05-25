package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Event is the payload appended to the outbox by service code.
type Event struct {
	// EventID is the dedupe key. If zero-valued, the publisher stamps a fresh UUID.
	EventID uuid.UUID
	// EventType uses dotted convention: "identity.user.created".
	EventType string
	// Stream is the Redis stream name, e.g. "paperboard.identity".
	Stream string
	// Payload is protojson-encoded bytes of one of the events/v1 payload messages.
	Payload []byte
	// TraceID is the OpenTelemetry trace id propagated from the caller context.
	TraceID string
	// OrgID is the tenancy scope; empty allowed only for identity.user.created.
	OrgID string
	// SchemaVersion is the payload schema version (1 in Wave 1).
	SchemaVersion int32
	// OccurredAt is the service clock in UTC.
	OccurredAt time.Time
}

// Publisher writes events into the outbox and drains to Redis in the background.
type Publisher interface {
	// Publish writes the event into the outbox table within the caller's tx.
	// Must be called inside the same pgx.Tx that mutated business state.
	// Returns immediately after row insert; actual Redis delivery is async.
	Publish(ctx context.Context, tx pgx.Tx, e Event) error

	// Start launches the drain + cleanup goroutines.
	// Blocks until ctx is cancelled; safe to call only once.
	Start(ctx context.Context) error

	// Stop signals both goroutines to exit cleanly.
	// Blocks until in-flight publish + cleanup pass complete (bounded by FlushTimeout).
	Stop(ctx context.Context) error
}

// Config holds all configuration for the outbox publisher.
type Config struct {
	// Schema is the owning service's Postgres schema, e.g. "identity".
	Schema string
	// SourceService matches Schema in Wave 1; baked into Envelope.source_service.
	SourceService string
	// Stream is the single Redis stream name per emitter, e.g. "paperboard.identity".
	Stream string
	// RedisAddr is the Redis server address, e.g. "redis-master.paper-board:6379".
	RedisAddr string
	// DrainInterval is how often the drain goroutine polls for pending rows.
	// Default: 200ms.
	DrainInterval time.Duration
	// DrainBatchSize is the max rows fetched per drain tick.
	// Default: 100.
	DrainBatchSize int
	// MaxAttempts is the outbox publish retry budget before dead-lettering.
	// Default: 5.
	MaxAttempts int
	// BackoffSchedule is per-attempt wait times. len must equal MaxAttempts.
	// Default: [1s, 2s, 4s, 8s, 16s].
	BackoffSchedule []time.Duration
	// DeliveredTTL is how long to keep delivered rows before cleanup.
	// Default: 7 days.
	DeliveredTTL time.Duration
	// DeadTTL is how long to keep dead rows before cleanup.
	// Default: 30 days.
	DeadTTL time.Duration
	// CleanupInterval is how often the cleanup goroutine runs.
	// Default: 1 minute.
	CleanupInterval time.Duration
	// FlushTimeout is the max wait on Stop before giving up.
	// Default: 5s.
	FlushTimeout time.Duration
	// Logger is injected; uses slog.Default() if nil.
	Logger *slog.Logger
	// now is injectable for testing clock-dependent cleanup.
	now func() time.Time
}

func (c *Config) applyDefaults() {
	if c.DrainInterval == 0 {
		c.DrainInterval = 200 * time.Millisecond
	}
	if c.DrainBatchSize == 0 {
		c.DrainBatchSize = 100
	}
	if c.MaxAttempts == 0 {
		c.MaxAttempts = 5
	}
	if len(c.BackoffSchedule) == 0 {
		c.BackoffSchedule = []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}
	}
	if c.DeliveredTTL == 0 {
		c.DeliveredTTL = 7 * 24 * time.Hour
	}
	if c.DeadTTL == 0 {
		c.DeadTTL = 30 * 24 * time.Hour
	}
	if c.CleanupInterval == 0 {
		c.CleanupInterval = 1 * time.Minute
	}
	if c.FlushTimeout == 0 {
		c.FlushTimeout = 5 * time.Second
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.now == nil {
		c.now = time.Now
	}
}

// NewPublisher creates a Publisher backed by the given pool and config.
func NewPublisher(pool *pgxpool.Pool, cfg Config) (Publisher, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	return newPublisherImpl(pool, cfg)
}

func validateConfig(cfg Config) error {
	switch {
	case cfg.Schema == "":
		return errorf("outbox: Schema is required")
	case cfg.SourceService == "":
		return errorf("outbox: SourceService is required")
	case cfg.Stream == "":
		return errorf("outbox: Stream is required")
	case cfg.RedisAddr == "":
		return errorf("outbox: RedisAddr is required")
	}
	if len(cfg.BackoffSchedule) > 0 && cfg.MaxAttempts > 0 && len(cfg.BackoffSchedule) != cfg.MaxAttempts {
		return errorf("outbox: BackoffSchedule length (%d) must equal MaxAttempts (%d)", len(cfg.BackoffSchedule), cfg.MaxAttempts)
	}
	return nil
}
