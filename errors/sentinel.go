// Package errors provides shared sentinel errors and gRPC mapping for
// paper-board services. All public service errors should wrap one of these
// sentinels so callers can use errors.Is for routing.
package errors

import "errors"

// Sentinel errors.
var (
	ErrNotFound          = errors.New("not found")
	ErrConflict          = errors.New("conflict")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrPermissionDenied  = errors.New("permission denied")
	ErrInvalidInput      = errors.New("invalid input")
	ErrRateLimited       = errors.New("rate limited")
	ErrInternal          = errors.New("internal error")
	ErrUnavailable       = errors.New("service unavailable")
)

// Is is a re-export so callers don't need to import stdlib errors separately.
func Is(err, target error) bool { return errors.Is(err, target) }

// As re-exports errors.As.
func As(err error, target any) bool { return errors.As(err, target) }
