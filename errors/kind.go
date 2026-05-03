package errors

// Kind returns a snake_case slug identifying the sentinel an error wraps.
// Used by sdk/httpmw to derive HTTP error codes ("<svc>.<kind>") and by
// sdk/log.ErrorAttrs to populate the "error.kind" attribute.
//
// Returns "internal" for nil-wrapped or unknown errors so callers do not need
// to pre-classify.
func Kind(err error) string {
	if err == nil {
		return "internal"
	}
	switch {
	case Is(err, ErrNotFound):
		return "not_found"
	case Is(err, ErrConflict):
		return "conflict"
	case Is(err, ErrUnauthorized):
		return "unauthorized"
	case Is(err, ErrPermissionDenied):
		return "permission_denied"
	case Is(err, ErrInvalidInput):
		return "invalid_input"
	case Is(err, ErrRateLimited):
		return "rate_limited"
	case Is(err, ErrUnavailable):
		return "unavailable"
	case Is(err, ErrInternal):
		return "internal"
	default:
		return "internal"
	}
}

// Message returns the canonical user-facing message for the sentinel an error
// wraps. Mirrors Kind. Use for HTTP error envelopes; the wrapped chain detail
// goes only to logs (avoid leaking internal context to clients).
func Message(err error) string {
	switch Kind(err) {
	case "not_found":
		return "not found"
	case "conflict":
		return "conflict"
	case "unauthorized":
		return "unauthorized"
	case "permission_denied":
		return "permission denied"
	case "invalid_input":
		return "invalid input"
	case "rate_limited":
		return "rate limited"
	case "unavailable":
		return "service unavailable"
	default:
		return "internal error"
	}
}
