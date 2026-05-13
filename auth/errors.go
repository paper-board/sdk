package auth

import "errors"

// Sentinel errors for callers to use with errors.Is.
// All map to HTTP 401 except ErrInsufficientRole (403).
var (
	// ErrMissingHeader is returned when the Authorization header is absent.
	ErrMissingHeader = errors.New("auth: missing Authorization header")
	// ErrInvalidScheme is returned when the Authorization header does not use Bearer scheme.
	ErrInvalidScheme = errors.New("auth: invalid scheme")
	// ErrUnauthenticated is returned when the credential is invalid or expired.
	ErrUnauthenticated = errors.New("auth: unauthenticated")
	// ErrKeyRetired is returned when the JWT signing key has been retired.
	ErrKeyRetired = errors.New("auth: signing key retired")
	// ErrWrongEnvironment is returned when credential env does not match route expectation.
	ErrWrongEnvironment = errors.New("auth: wrong environment")
	// ErrInsufficientRole is returned when the caller's role does not meet the route requirement (HTTP 403).
	ErrInsufficientRole = errors.New("auth: insufficient role")
)
