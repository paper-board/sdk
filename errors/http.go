package errors

import "net/http"

// ToHTTPStatus maps a wrapped sentinel error to an HTTP status code.
// Used by sdk/httpmw.HandleErr as the central HTTP error boundary.
//
// Mapping:
//
//	ErrNotFound          → 404
//	ErrConflict          → 409
//	ErrUnauthorized      → 401
//	ErrPermissionDenied  → 403
//	ErrInvalidInput      → 400
//	ErrRateLimited       → 429
//	ErrUnavailable       → 503
//	ErrInternal          → 500
//	nil or unknown       → 500
//
// Example:
//
//	if err := repo.Get(ctx, id); err != nil {
//	    http.Error(w, err.Error(), errors.ToHTTPStatus(err))
//	    return
//	}
func ToHTTPStatus(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}
	switch {
	case Is(err, ErrNotFound):
		return http.StatusNotFound
	case Is(err, ErrConflict):
		return http.StatusConflict
	case Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case Is(err, ErrPermissionDenied):
		return http.StatusForbidden
	case Is(err, ErrInvalidInput):
		return http.StatusBadRequest
	case Is(err, ErrRateLimited):
		return http.StatusTooManyRequests
	case Is(err, ErrUnavailable):
		return http.StatusServiceUnavailable
	case Is(err, ErrInternal):
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}
