package errors

import (
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Classification categorises a gRPC error for retry decisions.
type Classification int

const (
	// ClassificationUnknown is returned when the error is not a gRPC status
	// or when err is nil. sdk/retry treats Unknown as Permanent (fail-loud).
	ClassificationUnknown Classification = iota
	// ClassificationRetryable indicates the operation may succeed on retry.
	ClassificationRetryable
	// ClassificationPermanent indicates the operation will not succeed on retry.
	ClassificationPermanent
)

// ClassifyGRPC inspects err for a wrapped gRPC status (errors.As walk, up to
// 3 levels). Non-gRPC errors return ClassificationUnknown.
// nil returns ClassificationUnknown (callers should branch on err != nil first).
func ClassifyGRPC(err error) Classification {
	if err == nil {
		return ClassificationUnknown
	}
	st, ok := extractStatus(err)
	if !ok {
		return ClassificationUnknown
	}
	switch st.Code() {
	case codes.Unavailable,
		codes.DeadlineExceeded,
		codes.Aborted,
		codes.ResourceExhausted,
		codes.Internal,
		codes.Unknown:
		return ClassificationRetryable

	case codes.InvalidArgument,
		codes.NotFound,
		codes.FailedPrecondition,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.Canceled,
		codes.Unimplemented,
		codes.OutOfRange,
		codes.DataLoss:
		return ClassificationPermanent

	default:
		return ClassificationUnknown
	}
}

// IsRetryable returns true iff ClassifyGRPC(err) == ClassificationRetryable.
func IsRetryable(err error) bool {
	return ClassifyGRPC(err) == ClassificationRetryable
}

// IsPermanent returns true iff ClassifyGRPC(err) == ClassificationPermanent.
func IsPermanent(err error) bool {
	return ClassifyGRPC(err) == ClassificationPermanent
}

// CodeString returns the canonical UPPERCASE gRPC code string for use in
// event payloads (e.g. "UNAVAILABLE"). Returns "" for non-gRPC errors.
// "OK" is never returned — the success path is err == nil.
func CodeString(err error) string {
	if err == nil {
		return ""
	}
	st, ok := extractStatus(err)
	if !ok {
		return ""
	}
	if st.Code() == codes.OK {
		return ""
	}
	// codes.Code.String() returns mixed-case (e.g. "NotFound"); convert to
	// SCREAMING_SNAKE per spec §2.4.2 CodeString contract.
	return toScreamingSnake(st.Code().String())
}

func toScreamingSnake(s string) string {
	var out []byte
	for i, r := range s {
		if r >= 'A' && r <= 'Z' && i > 0 {
			out = append(out, '_')
		}
		out = append(out, strings.ToUpper(string(r))...)
	}
	return string(out)
}

// extractStatus walks the error chain (errors.As, max 3 levels) looking for
// a gRPC status. Returns the status and true if found.
func extractStatus(err error) (*status.Status, bool) {
	// Attempt direct conversion first (common fast path).
	if st, ok := status.FromError(err); ok && st.Code() != codes.OK {
		return st, true
	}

	// Walk up to 3 levels of wrapping.
	current := err
	for range 3 {
		var grpcErr interface{ GRPCStatus() *status.Status }
		if As(current, &grpcErr) {
			st := grpcErr.GRPCStatus()
			if st != nil {
				return st, true
			}
		}
		unwrapped := Unwrap(current)
		if unwrapped == nil {
			break
		}
		current = unwrapped
		if st, ok := status.FromError(current); ok && st.Code() != codes.OK {
			return st, true
		}
	}
	return nil, false
}
