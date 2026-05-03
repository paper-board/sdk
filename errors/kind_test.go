package errors_test

import (
	stderrors "errors"
	"testing"

	pberrs "github.com/paper-board/sdk/errors"
)

func TestKind(t *testing.T) {
	cases := []struct {
		name string
		in   error
		kind string
	}{
		{"nil", nil, "internal"},
		{"not_found", pberrs.Wrap(pberrs.ErrNotFound, "x"), "not_found"},
		{"conflict", pberrs.Wrap(pberrs.ErrConflict, "x"), "conflict"},
		{"unauthorized", pberrs.Wrap(pberrs.ErrUnauthorized, "x"), "unauthorized"},
		{"permission_denied", pberrs.Wrap(pberrs.ErrPermissionDenied, "x"), "permission_denied"},
		{"invalid_input", pberrs.Wrap(pberrs.ErrInvalidInput, "x"), "invalid_input"},
		{"rate_limited", pberrs.Wrap(pberrs.ErrRateLimited, "x"), "rate_limited"},
		{"unavailable", pberrs.Wrap(pberrs.ErrUnavailable, "x"), "unavailable"},
		{"internal", pberrs.Wrap(pberrs.ErrInternal, "x"), "internal"},
		{"unknown", stderrors.New("rando"), "internal"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pberrs.Kind(c.in); got != c.kind {
				t.Fatalf("Kind = %q, want %q", got, c.kind)
			}
		})
	}
}

func TestMessage(t *testing.T) {
	if got := pberrs.Message(pberrs.Wrap(pberrs.ErrNotFound, "user", "id", "abc")); got != "not found" {
		t.Fatalf("Message = %q", got)
	}
	if got := pberrs.Message(stderrors.New("rando")); got != "internal error" {
		t.Fatalf("Message unknown = %q", got)
	}
}
