package errors_test

import (
	stderrors "errors"
	"net/http"
	"testing"

	"github.com/paper-board/sdk/errors"
)

func TestToHTTPStatus(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want int
	}{
		{"nil", nil, http.StatusInternalServerError},
		{"not_found", errors.Wrap(errors.ErrNotFound, "x"), http.StatusNotFound},
		{"conflict", errors.Wrap(errors.ErrConflict, "x"), http.StatusConflict},
		{"unauthorized", errors.Wrap(errors.ErrUnauthorized, "x"), http.StatusUnauthorized},
		{"permission_denied", errors.Wrap(errors.ErrPermissionDenied, "x"), http.StatusForbidden},
		{"invalid_input", errors.Wrap(errors.ErrInvalidInput, "x"), http.StatusBadRequest},
		{"rate_limited", errors.Wrap(errors.ErrRateLimited, "x"), http.StatusTooManyRequests},
		{"unavailable", errors.Wrap(errors.ErrUnavailable, "x"), http.StatusServiceUnavailable},
		{"internal", errors.Wrap(errors.ErrInternal, "x"), http.StatusInternalServerError},
		{"unknown", stderrors.New("rando"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := errors.ToHTTPStatus(c.in); got != c.want {
				t.Fatalf("ToHTTPStatus(%v) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestToHTTPStatusNestedWrap(t *testing.T) {
	leaf := errors.Wrap(errors.ErrNotFound, "user", "id", "abc")
	mid := errors.Wrap(leaf, "load profile")
	if got := errors.ToHTTPStatus(mid); got != http.StatusNotFound {
		t.Fatalf("nested wrap lost sentinel: got %d, want 404", got)
	}
}
