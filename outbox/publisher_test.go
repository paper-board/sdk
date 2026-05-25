package outbox

import (
	"testing"
	"time"
)

func TestValidateConfig_missingSchema(t *testing.T) {
	err := validateConfig(Config{SourceService: "svc", Stream: "s", RedisAddr: "addr"})
	if err == nil {
		t.Fatal("want error for missing Schema")
	}
}

func TestValidateConfig_missingSourceService(t *testing.T) {
	err := validateConfig(Config{Schema: "svc", Stream: "s", RedisAddr: "addr"})
	if err == nil {
		t.Fatal("want error for missing SourceService")
	}
}

func TestValidateConfig_missingStream(t *testing.T) {
	err := validateConfig(Config{Schema: "svc", SourceService: "svc", RedisAddr: "addr"})
	if err == nil {
		t.Fatal("want error for missing Stream")
	}
}

func TestValidateConfig_missingRedisAddr(t *testing.T) {
	err := validateConfig(Config{Schema: "svc", SourceService: "svc", Stream: "s"})
	if err == nil {
		t.Fatal("want error for missing RedisAddr")
	}
}

func TestValidateConfig_backoffScheduleLengthMismatch(t *testing.T) {
	err := validateConfig(Config{
		Schema:          "svc",
		SourceService:   "svc",
		Stream:          "s",
		RedisAddr:       "addr",
		MaxAttempts:     5,
		BackoffSchedule: []time.Duration{1 * time.Second, 2 * time.Second},
	})
	if err == nil {
		t.Fatal("want error for BackoffSchedule length != MaxAttempts")
	}
}

func TestValidateConfig_valid(t *testing.T) {
	err := validateConfig(Config{
		Schema:        "svc",
		SourceService: "svc",
		Stream:        "s",
		RedisAddr:     "addr",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigApplyDefaults(t *testing.T) {
	cfg := Config{
		Schema:        "svc",
		SourceService: "svc",
		Stream:        "s",
		RedisAddr:     "addr",
	}
	cfg.applyDefaults()

	if cfg.DrainInterval != 200*time.Millisecond {
		t.Errorf("DrainInterval: want 200ms, got %v", cfg.DrainInterval)
	}
	if cfg.DrainBatchSize != 100 {
		t.Errorf("DrainBatchSize: want 100, got %d", cfg.DrainBatchSize)
	}
	if cfg.MaxAttempts != 5 {
		t.Errorf("MaxAttempts: want 5, got %d", cfg.MaxAttempts)
	}
	if len(cfg.BackoffSchedule) != 5 {
		t.Errorf("BackoffSchedule: want len 5, got %d", len(cfg.BackoffSchedule))
	}
	if cfg.DeliveredTTL != 7*24*time.Hour {
		t.Errorf("DeliveredTTL: want 7d, got %v", cfg.DeliveredTTL)
	}
	if cfg.DeadTTL != 30*24*time.Hour {
		t.Errorf("DeadTTL: want 30d, got %v", cfg.DeadTTL)
	}
	if cfg.CleanupInterval != 1*time.Minute {
		t.Errorf("CleanupInterval: want 1m, got %v", cfg.CleanupInterval)
	}
	if cfg.FlushTimeout != 5*time.Second {
		t.Errorf("FlushTimeout: want 5s, got %v", cfg.FlushTimeout)
	}
	if cfg.Logger == nil {
		t.Error("Logger: want non-nil default")
	}
}

func TestBuildEnvelope(t *testing.T) {
	eventID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	row := pendingRow{
		eventID:       eventID,
		eventType:     "identity.user.created",
		sourceService: "identity",
		schemaVersion: 1,
		stream:        "paperboard.identity",
		payload:       []byte(`{"user_id":"abc"}`),
		traceID:       "trace-123",
		orgID:         "org-456",
		occurredAt:    time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
	}
	env := buildEnvelope(row, "identity")
	if env.EventType != "identity.user.created" {
		t.Errorf("EventType: want identity.user.created, got %s", env.EventType)
	}
	if env.SourceService != "identity" {
		t.Errorf("SourceService: want identity, got %s", env.SourceService)
	}
	if env.TraceID != "trace-123" {
		t.Errorf("TraceID: want trace-123, got %s", env.TraceID)
	}
	if env.OrgID != "org-456" {
		t.Errorf("OrgID: want org-456, got %s", env.OrgID)
	}
}
