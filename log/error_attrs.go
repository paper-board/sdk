package log

import (
	"log/slog"

	pberrs "github.com/paper-board/sdk/errors"
)

// ErrorAttrs returns slog attrs describing err — its sentinel kind and message.
// Designed to be spread into logger calls:
//
//	log.Pkg(ctx).Error("load profile failed", log.ErrorAttrs(err)...)
//	slog.LogAttrs(ctx, slog.LevelError, "x", log.ErrorAttrsAttr(err)...)
//
// Returns nil for nil err so it composes harmlessly.
func ErrorAttrs(err error) []any {
	if err == nil {
		return nil
	}
	return []any{
		slog.String("error.kind", pberrs.Kind(err)),
		slog.String("error", err.Error()),
	}
}

// ErrorAttrsAttr is the []slog.Attr variant for use with slog.LogAttrs.
func ErrorAttrsAttr(err error) []slog.Attr {
	if err == nil {
		return nil
	}
	return []slog.Attr{
		slog.String("error.kind", pberrs.Kind(err)),
		slog.String("error", err.Error()),
	}
}
