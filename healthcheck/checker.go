package healthcheck

import (
	"context"
	"time"
)

// Checker probes a single dependency for readiness.
type Checker interface {
	// Name uniquely identifies this checker in handler responses.
	Name() string
	// Check returns nil when the dependency is reachable.
	// Implementations should respect ctx deadline (~500ms recommended).
	Check(ctx context.Context) error
}

// Result is the per-checker outcome included in handler responses.
type Result struct {
	Name      string        `json:"name"`
	Status    string        `json:"status"` // "ok" or "fail"
	LatencyMs int64         `json:"latency_ms"`
	Err       string        `json:"err,omitempty"`
	took      time.Duration `json:"-"`
}
