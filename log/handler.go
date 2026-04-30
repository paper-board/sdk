// Package log provides slog-based structured logging for paper-board services.
//
// Phase 1.0: minimal JSONHandler. Redaction rules + OTel correlation Phase 1.5.
package log

import (
	"io"
	"log/slog"
	"os"
)

// New returns a slog.Logger writing structured JSON to w. If w is nil, os.Stderr is used.
// Pass service name for the "service" attribute.
func New(w io.Writer, service string, level slog.Level) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
	})
	return slog.New(h).With("service", service)
}

// SetDefault installs `l` as the package default. Once called, slog.Default()
// returns this logger across the process.
func SetDefault(l *slog.Logger) {
	slog.SetDefault(l)
}
