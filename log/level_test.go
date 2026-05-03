package log_test

import (
	"log/slog"
	"testing"

	pblog "github.com/paper-board/sdk/log"
)

func TestLevelFromEnv(t *testing.T) {
	cases := map[string]slog.Level{
		"":          slog.LevelInfo,
		"info":      slog.LevelInfo,
		"INFO":      slog.LevelInfo,
		" debug ":   slog.LevelDebug,
		"debug":     slog.LevelDebug,
		"warn":      slog.LevelWarn,
		"warning":   slog.LevelWarn,
		"WARN":      slog.LevelWarn,
		"error":     slog.LevelError,
		"err":       slog.LevelError,
		"weird":     slog.LevelInfo,
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", in)
			if got := pblog.LevelFromEnv(); got != want {
				t.Fatalf("LevelFromEnv(%q) = %v, want %v", in, got, want)
			}
		})
	}
}
