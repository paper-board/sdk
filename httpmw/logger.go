package httpmw

import (
	"net/http"
	"time"

	pblog "github.com/paper-board/sdk/log"
)

// Logger emits one INFO record per request with the canonical observability
// keys: method, route, status, duration_ms, bytes_in, bytes_out. Place after
// RequestID + Recover so request_id is in ctx and panics are caught first.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := wrapResponseWriter(w)
		next.ServeHTTP(ww, r)
		pblog.Pkg(r.Context()).Info("http_request",
			"method", r.Method,
			"route", r.URL.Path,
			"status", ww.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes_in", r.ContentLength,
			"bytes_out", ww.bytes,
		)
	})
}

// respWriter intercepts WriteHeader + Write to record status + bytes_out, and
// forwards Flush so SSE handlers keep working.
type respWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func wrapResponseWriter(w http.ResponseWriter) *respWriter {
	return &respWriter{ResponseWriter: w, status: http.StatusOK}
}

func (r *respWriter) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *respWriter) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}

func (r *respWriter) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
