package errors_test

import (
	"fmt"
	"testing"

	pberrs "github.com/paper-board/sdk/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func grpcErr(code codes.Code) error {
	return status.Error(code, code.String())
}

func TestClassifyGRPC_nil(t *testing.T) {
	if got := pberrs.ClassifyGRPC(nil); got != pberrs.ClassificationUnknown {
		t.Fatalf("nil: want Unknown, got %v", got)
	}
}

func TestClassifyGRPC_nonGRPC(t *testing.T) {
	err := fmt.Errorf("plain error")
	if got := pberrs.ClassifyGRPC(err); got != pberrs.ClassificationUnknown {
		t.Fatalf("non-grpc: want Unknown, got %v", got)
	}
}

func TestClassifyGRPC_retryable(t *testing.T) {
	cases := []codes.Code{
		codes.Unavailable,
		codes.DeadlineExceeded,
		codes.Aborted,
		codes.ResourceExhausted,
		codes.Internal,
		codes.Unknown,
	}
	for _, code := range cases {
		t.Run(code.String(), func(t *testing.T) {
			if got := pberrs.ClassifyGRPC(grpcErr(code)); got != pberrs.ClassificationRetryable {
				t.Fatalf("code %v: want Retryable, got %v", code, got)
			}
		})
	}
}

func TestClassifyGRPC_permanent(t *testing.T) {
	cases := []codes.Code{
		codes.InvalidArgument,
		codes.NotFound,
		codes.FailedPrecondition,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.Canceled,
		codes.Unimplemented,
		codes.OutOfRange,
		codes.DataLoss,
	}
	for _, code := range cases {
		t.Run(code.String(), func(t *testing.T) {
			if got := pberrs.ClassifyGRPC(grpcErr(code)); got != pberrs.ClassificationPermanent {
				t.Fatalf("code %v: want Permanent, got %v", code, got)
			}
		})
	}
}

func TestClassifyGRPC_wrapped(t *testing.T) {
	inner := grpcErr(codes.Unavailable)
	wrapped1 := fmt.Errorf("layer1: %w", inner)
	wrapped2 := fmt.Errorf("layer2: %w", wrapped1)
	wrapped3 := fmt.Errorf("layer3: %w", wrapped2)

	if got := pberrs.ClassifyGRPC(wrapped3); got != pberrs.ClassificationRetryable {
		t.Fatalf("3-level wrap: want Retryable, got %v", got)
	}
}

func TestIsRetryable(t *testing.T) {
	if !pberrs.IsRetryable(grpcErr(codes.Unavailable)) {
		t.Fatal("Unavailable should be retryable")
	}
	if pberrs.IsRetryable(grpcErr(codes.NotFound)) {
		t.Fatal("NotFound should not be retryable")
	}
}

func TestIsPermanent(t *testing.T) {
	if !pberrs.IsPermanent(grpcErr(codes.NotFound)) {
		t.Fatal("NotFound should be permanent")
	}
	if pberrs.IsPermanent(grpcErr(codes.Unavailable)) {
		t.Fatal("Unavailable should not be permanent")
	}
}

func TestCodeString(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{fmt.Errorf("plain"), ""},
		{grpcErr(codes.Unavailable), "UNAVAILABLE"},
		{grpcErr(codes.NotFound), "NOT_FOUND"},
		{grpcErr(codes.InvalidArgument), "INVALID_ARGUMENT"},
		{grpcErr(codes.DeadlineExceeded), "DEADLINE_EXCEEDED"},
		{grpcErr(codes.PermissionDenied), "PERMISSION_DENIED"},
	}
	for _, tc := range tests {
		got := pberrs.CodeString(tc.err)
		if got != tc.want {
			t.Errorf("CodeString(%v): want %q, got %q", tc.err, tc.want, got)
		}
	}
}

func TestToGRPCStatus(t *testing.T) {
	if pberrs.ToGRPCStatus(nil) != nil {
		t.Fatal("nil should return nil")
	}
	err := pberrs.ToGRPCStatus(pberrs.Wrap(pberrs.ErrNotFound, "user"))
	if err == nil {
		t.Fatal("want non-nil gRPC error")
	}
}

func TestFromGRPCStatus(t *testing.T) {
	if pberrs.FromGRPCStatus(nil) != nil {
		t.Fatal("nil should return nil")
	}
	grpcNotFound := grpcErr(codes.NotFound)
	converted := pberrs.FromGRPCStatus(grpcNotFound)
	if !pberrs.Is(converted, pberrs.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", converted)
	}
}

func TestWrap(t *testing.T) {
	err := pberrs.Wrap(pberrs.ErrNotFound, "user", "id", "123")
	if err == nil {
		t.Fatal("want non-nil")
	}
	if !pberrs.Is(err, pberrs.ErrNotFound) {
		t.Fatal("want ErrNotFound in chain")
	}
}

func TestKindAndMessage(t *testing.T) {
	sentinels := []struct {
		err  error
		kind string
		msg  string
	}{
		{pberrs.ErrNotFound, "not_found", "not found"},
		{pberrs.ErrConflict, "conflict", "conflict"},
		{pberrs.ErrUnauthorized, "unauthorized", "unauthorized"},
		{pberrs.ErrPermissionDenied, "permission_denied", "permission denied"},
		{pberrs.ErrInvalidInput, "invalid_input", "invalid input"},
		{pberrs.ErrRateLimited, "rate_limited", "rate limited"},
		{pberrs.ErrUnavailable, "unavailable", "service unavailable"},
		{pberrs.ErrInternal, "internal", "internal error"},
		{nil, "internal", "internal error"},
	}
	for _, tc := range sentinels {
		if got := pberrs.Kind(tc.err); got != tc.kind {
			t.Errorf("Kind(%v): want %s, got %s", tc.err, tc.kind, got)
		}
		if got := pberrs.Message(tc.err); got != tc.msg {
			t.Errorf("Message(%v): want %s, got %s", tc.err, tc.msg, got)
		}
	}
}

func TestToGRPCStatusAllSentinels(t *testing.T) {
	sentinels := []error{
		pberrs.ErrNotFound,
		pberrs.ErrConflict,
		pberrs.ErrUnauthorized,
		pberrs.ErrPermissionDenied,
		pberrs.ErrInvalidInput,
		pberrs.ErrRateLimited,
		pberrs.ErrUnavailable,
		pberrs.ErrInternal,
		fmt.Errorf("unknown"),
	}
	for _, s := range sentinels {
		got := pberrs.ToGRPCStatus(s)
		if got == nil {
			t.Errorf("ToGRPCStatus(%v): want non-nil", s)
		}
	}
}

func TestFromGRPCStatusAllCodes(t *testing.T) {
	allCodes := []codes.Code{
		codes.NotFound,
		codes.AlreadyExists,
		codes.Unauthenticated,
		codes.PermissionDenied,
		codes.InvalidArgument,
		codes.ResourceExhausted,
		codes.Unavailable,
		codes.Internal,
	}
	for _, c := range allCodes {
		got := pberrs.FromGRPCStatus(grpcErr(c))
		if got == nil {
			t.Errorf("FromGRPCStatus(%v): want non-nil", c)
		}
	}

	// Non-gRPC error passes through.
	plain := fmt.Errorf("plain")
	if pberrs.FromGRPCStatus(plain) != plain {
		t.Fatal("non-grpc error should pass through unchanged")
	}
}
