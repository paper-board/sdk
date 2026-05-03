// Package httpmw provides shared HTTP middleware for paper-board services.
//
// Canonical chain order (top-of-stack first), per
// docs/standards/http-api-conventions.md §2:
//
//  1. RequestID         — read X-Request-Id or mint UUIDv7
//  2. TrustHeaders      — Phase 7+ from gateway; pre-Phase-7 acts as a no-op
//  3. OtelHTTP          — otelhttp.NewHandler wrapper
//  4. Recover           — panic-to-500 with structured log
//  5. Logger            — start/end log with route/status/duration
//  6. BodyLimit         — http.MaxBytesReader
//  7. AuthStub | Auth   — Phase 1: AuthStub stamps DefaultOrgID; Phase 2: real Auth
//  8. (chi.Compress)    — gzip (provided by chi)
//  9. Timeout           — request-scoped deadline (chi.Middleware.Timeout)
//
// HandleErr is the central HTTP error boundary; every handler converts a
// service-level error into the canonical envelope via this function.
package httpmw

// ErrorEnvelope is the canonical JSON wire shape for HTTP error responses.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody is the payload inside ErrorEnvelope.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// ValidationDetail describes a single validator/v10 field failure surfaced in
// ErrorBody.Details when an HTTP boundary is handed a validator.ValidationErrors.
type ValidationDetail struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Message string `json:"message,omitempty"`
}
