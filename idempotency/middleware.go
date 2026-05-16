package idempotency

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// orgIDKey is a context key the service must set before this middleware runs.
// Services typically populate this in the auth middleware after JWT/API-key verify.
type orgIDKey struct{}

// FromContext extracts the org_id from the request context.
// Returns false when absent — middleware will bypass idempotency in that case.
func FromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(orgIDKey{}).(uuid.UUID)
	return v, ok
}

// WithOrgID returns a context carrying the org_id for idempotency lookups.
// The service's auth middleware should call this after extracting org_id from the JWT/API-key.
func WithOrgID(ctx context.Context, orgID uuid.UUID) context.Context {
	return context.WithValue(ctx, orgIDKey{}, orgID)
}

const ttl = 24 * time.Hour

// Require returns an HTTP middleware that implements Idempotency-Key semantics.
func Require(store Store, opts ...Option) func(http.Handler) http.Handler {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			routeKey := r.Method + " " + r.URL.Path
			if _, ok := cfg.excludes[routeKey]; ok {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			orgID, ok := FromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"idempotency_requires_auth"}`, http.StatusUnauthorized)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, `{"error":"read_body"}`, http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			hash := hashRequest(r, body)

			existing, err := store.Get(r.Context(), orgID, key)
			switch {
			case err == nil:
				if existing.RequestHash != hash {
					writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
						"error": "idempotency_key_conflict",
					})
					return
				}
				replayResponse(w, existing)
				return
			case errors.Is(err, ErrNotFound):
				// fall through to execute
			default:
				http.Error(w, `{"error":"idempotency_store"}`, http.StatusInternalServerError)
				return
			}

			captured := &responseCapture{ResponseWriter: w}
			next.ServeHTTP(captured, r)

			if captured.status >= 200 && captured.status < 300 {
				rec := &Record{
					OrgID:           orgID,
					Key:             key,
					RequestHash:     hash,
					ResponseStatus:  captured.status,
					ResponseBody:    captured.body,
					ResponseHeaders: captureHeaders(captured.Header()),
					CreatedAt:       time.Now(),
					ExpiresAt:       time.Now().Add(ttl),
				}
				if err := store.Put(r.Context(), rec); err != nil {
					slog.ErrorContext(r.Context(), "idempotency: store put failed; replay broken for this key",
						"err", err, "org_id", rec.OrgID, "key", rec.Key)
				}
			}
		})
	}
}

func captureHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
