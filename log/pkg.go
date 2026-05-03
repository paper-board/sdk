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
	ctxKeyRoles     ctxKey = "roles"
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

// WithRoles stores the caller's role list (RBAC; populated by Auth middleware).
// Roles intentionally do not auto-attach to Pkg() loggers — they vary in size
// and most log lines do not need them.
func WithRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, ctxKeyRoles, roles)
}

// RequestIDFromContext returns the stored request_id, or "" if absent.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

// TraceIDFromContext returns the stored W3C trace ID, or "" if absent.
func TraceIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyTraceID).(string)
	return v
}

// OrgFromContext returns the stored org_id, or "" if absent.
func OrgFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyOrgID).(string)
	return v
}

// UserFromContext returns the stored user_id, or "" if absent.
func UserFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyUserID).(string)
	return v
}

// RolesFromContext returns the stored roles slice, or nil if absent.
func RolesFromContext(ctx context.Context) []string {
	v, _ := ctx.Value(ctxKeyRoles).([]string)
	return v
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
