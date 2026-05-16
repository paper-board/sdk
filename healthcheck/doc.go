// Package healthcheck provides composable readiness probes for paper-board
// services. Each service registers a set of Checker implementations and
// exposes them via a single /readyz HTTP handler.
//
// The package distinguishes:
//   - /livez: process-alive only (use plain w.WriteHeader(200) — not this package)
//   - /readyz: dependency-aware; this package's Handler() returns 200 when
//     all registered Checkers pass, 503 otherwise.
//
// Built-in Checkers: DB(*pgxpool.Pool) and GRPC(name, *grpc.ClientConn).
// Services compose them via New(c1, c2, ...).
package healthcheck
