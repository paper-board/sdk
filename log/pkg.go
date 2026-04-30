package log

import (
	"context"
	"log/slog"
)

type ctxKey string

const (
	ctxKeyRequestID ctxKey = "request_id"
	ctxKeyTraceID   ctxKey = "trace_id"
	ctxKeyOrgID     ctxKey = "org_id"
	ctxKeyUserID    ctxKey = "user_id"
)

// WithRequestID stores request_id in ctx for downstream Pkg() calls.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// WithTraceID stores W3C trace ID.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyTraceID, id)
}

// WithOrg stores org_id (tenant context).
func WithOrg(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyOrgID, id)
}

// WithUser stores user_id.
func WithUser(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, id)
}

// Pkg returns a logger pre-loaded with context attributes (request_id, trace_id,
// org_id, user_id). Use this everywhere instead of slog.Default() directly so
// auto-fields are included.
//
//	log.Pkg(ctx).Info("user created", "id", u.ID)
func Pkg(ctx context.Context) *slog.Logger {
	l := slog.Default()
	if ctx == nil {
		return l
	}
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok && v != "" {
		l = l.With("request_id", v)
	}
	if v, ok := ctx.Value(ctxKeyTraceID).(string); ok && v != "" {
		l = l.With("trace_id", v)
	}
	if v, ok := ctx.Value(ctxKeyOrgID).(string); ok && v != "" {
		l = l.With("org_id", v)
	}
	if v, ok := ctx.Value(ctxKeyUserID).(string); ok && v != "" {
		l = l.With("user_id", v)
	}
	return l
}
