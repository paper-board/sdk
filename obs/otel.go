// Package obs provides OpenTelemetry SDK setup, propagation, and middleware
// shells for paper-board services.
//
// Phase 1.0: no-op SetupOTel placeholder.
// Phase 1.5: real exporter (OTLP), W3C TraceContext propagation, gRPC + HTTP middleware.
package obs

import "context"

// Shutdown is returned by SetupOTel; callers must defer it on shutdown.
type Shutdown func(context.Context) error

// SetupOTel initializes the OTel SDK for a service. Returns a shutdown closer.
//
// Phase 1.0 stub: no-op (exports nothing). Replace in Phase 1.5 with OTLP
// exporter, batch span processor, W3C TraceContext propagator.
func SetupOTel(serviceName, version string) (Shutdown, error) {
	return func(context.Context) error { return nil }, nil
}
