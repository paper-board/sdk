package httpmw

import (
	"net/http"

	"github.com/google/uuid"

	pblog "github.com/paper-board/sdk/log"
)

// HeaderRequestID is the canonical header name for inbound + outbound request IDs.
const HeaderRequestID = "X-Request-Id"

// RequestID reads HeaderRequestID from the inbound request; mints a UUIDv7 if
// absent. The value is stored in ctx via log.WithRequestID and echoed back on
// the response header so clients can correlate.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if id == "" {
			id = newID()
		}
		w.Header().Set(HeaderRequestID, id)
		ctx := pblog.WithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newID() string {
	if v7, err := uuid.NewV7(); err == nil {
		return v7.String()
	}
	return uuid.NewString()
}
