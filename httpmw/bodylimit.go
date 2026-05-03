package httpmw

import "net/http"

// BodyLimit caps the inbound request body at n bytes via http.MaxBytesReader.
// Use 1<<20 (1 MiB) for JSON; raise for endpoints accepting file uploads.
// Streaming routes (SSE, chunked uploads) should opt out via chi.Router.With.
func BodyLimit(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}
