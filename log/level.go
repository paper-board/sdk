package log

import (
	"log/slog"
	"os"
	"strings"
)

// LevelFromEnv reads LOG_LEVEL and returns the corresponding slog.Level.
// Accepted values (case-insensitive): debug, info, warn|warning, error|err.
// Defaults to info when LOG_LEVEL is unset, empty, or unrecognised.
func LevelFromEnv() slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "err":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
