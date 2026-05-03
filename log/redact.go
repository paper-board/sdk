package log

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
)

// REDACT is the placeholder string substituted for redacted values.
const REDACT = "[REDACTED]"

// RedactingHandler wraps an inner slog.Handler and redacts sensitive values
// from log records using two strategies in series:
//
//  1. Key denylist — any attribute whose Key matches DenyKeys
//     (case-insensitive) has its value replaced with REDACT.
//  2. Value regex — any string-typed attribute (and the message) has
//     matching substrings replaced with REDACT.
//
// Defaults cover bearer tokens, JWT-shaped strings, sk-prefixed API keys, and
// email addresses. Callers may extend via RedactOpts.
type RedactingHandler struct {
	inner   slog.Handler
	denyMap map[string]struct{}
	regexes []*regexp.Regexp
}

// RedactOpts extends the default deny/regex sets.
type RedactOpts struct {
	DenyKeys []string
	Patterns []*regexp.Regexp
}

// DefaultDenyKeys returns the canonical denylist (lower-case match).
func DefaultDenyKeys() []string {
	return []string{
		"password", "passwd", "secret", "api_key", "apikey",
		"authorization", "token", "cookie", "set-cookie",
	}
}

// DefaultPatterns returns the canonical regex set applied to string values.
func DefaultPatterns() []*regexp.Regexp {
	return []*regexp.Regexp{
		regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-]+`),
		regexp.MustCompile(`eyJ[A-Za-z0-9._\-]{10,}`),
		regexp.MustCompile(`sk-[A-Za-z0-9_\-]{20,}`),
		regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
	}
}

// NewRedactingHandler wraps inner with default redaction merged with opts.
func NewRedactingHandler(inner slog.Handler, opts RedactOpts) *RedactingHandler {
	deny := append(DefaultDenyKeys(), opts.DenyKeys...)
	denyMap := make(map[string]struct{}, len(deny))
	for _, k := range deny {
		denyMap[strings.ToLower(k)] = struct{}{}
	}
	return &RedactingHandler{
		inner:   inner,
		denyMap: denyMap,
		regexes: append(DefaultPatterns(), opts.Patterns...),
	}
}

// Enabled forwards to inner.
func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle redacts the record's message + attrs, then forwards to inner.
func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	out := slog.NewRecord(r.Time, r.Level, h.redactString(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(h.redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, out)
}

// WithAttrs returns a new handler with attrs (pre-redacted) prepended on inner.
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	red := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		red[i] = h.redactAttr(a)
	}
	return &RedactingHandler{
		inner:   h.inner.WithAttrs(red),
		denyMap: h.denyMap,
		regexes: h.regexes,
	}
}

// WithGroup forwards.
func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{
		inner:   h.inner.WithGroup(name),
		denyMap: h.denyMap,
		regexes: h.regexes,
	}
}

func (h *RedactingHandler) redactAttr(a slog.Attr) slog.Attr {
	if _, deny := h.denyMap[strings.ToLower(a.Key)]; deny {
		return slog.String(a.Key, REDACT)
	}
	v := a.Value
	switch v.Kind() {
	case slog.KindString:
		return slog.String(a.Key, h.redactString(v.String()))
	case slog.KindGroup:
		grp := v.Group()
		out := make([]slog.Attr, len(grp))
		for i, sub := range grp {
			out[i] = h.redactAttr(sub)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(out...)}
	default:
		return a
	}
}

func (h *RedactingHandler) redactString(s string) string {
	for _, re := range h.regexes {
		s = re.ReplaceAllString(s, REDACT)
	}
	return s
}

var _ slog.Handler = (*RedactingHandler)(nil)
