package log_test

import (
	"testing"

	pberrs "github.com/paper-board/sdk/errors"
	pblog "github.com/paper-board/sdk/log"
)

func TestErrorAttrsNil(t *testing.T) {
	if got := pblog.ErrorAttrs(nil); got != nil {
		t.Fatalf("nil err: %v", got)
	}
	if got := pblog.ErrorAttrsAttr(nil); got != nil {
		t.Fatalf("nil err Attr: %v", got)
	}
}

func TestErrorAttrsClassify(t *testing.T) {
	cases := []struct {
		err  error
		kind string
	}{
		{pberrs.Wrap(pberrs.ErrNotFound, "x"), "not_found"},
		{pberrs.Wrap(pberrs.ErrConflict, "x"), "conflict"},
		{pberrs.Wrap(pberrs.ErrUnauthorized, "x"), "unauthorized"},
		{pberrs.Wrap(pberrs.ErrPermissionDenied, "x"), "permission_denied"},
		{pberrs.Wrap(pberrs.ErrInvalidInput, "x"), "invalid_input"},
		{pberrs.Wrap(pberrs.ErrRateLimited, "x"), "rate_limited"},
		{pberrs.Wrap(pberrs.ErrUnavailable, "x"), "unavailable"},
		{pberrs.Wrap(pberrs.ErrInternal, "x"), "internal"},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			attrs := pblog.ErrorAttrsAttr(c.err)
			if len(attrs) != 2 {
				t.Fatalf("len: %d", len(attrs))
			}
			if attrs[0].Key != "error.kind" || attrs[0].Value.String() != c.kind {
				t.Fatalf("kind: %v", attrs[0])
			}
			if attrs[1].Key != "error" {
				t.Fatalf("error: %v", attrs[1])
			}
		})
	}
}
