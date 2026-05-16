// Package idempotency provides HTTP middleware that implements the
// Idempotency-Key header semantics (Stripe-style). When a client retries
// a request with the same key + same body, the server replays the original
// response byte-for-byte instead of executing again.
//
// Storage is provided by the service (per-schema table) via the Store
// interface. The middleware is opt-in per service via Require(store, ...).
//
// Default behavior: applies to POST/PUT/PATCH/DELETE; bypasses GET/HEAD/OPTIONS.
// Use WithExclude to bypass specific routes (e.g., auth flows).
package idempotency
