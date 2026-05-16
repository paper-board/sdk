package idempotency

import "net/http"

// responseCapture wraps http.ResponseWriter to record status, headers, and body
// for storage when an idempotent request completes.
type responseCapture struct {
	http.ResponseWriter
	status int
	body   []byte
}

func (c *responseCapture) WriteHeader(code int) {
	c.status = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *responseCapture) Write(b []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	c.body = append(c.body, b...)
	return c.ResponseWriter.Write(b)
}

func replayResponse(w http.ResponseWriter, rec *Record) {
	for k, v := range rec.ResponseHeaders {
		w.Header().Set(k, v)
	}
	w.Header().Set("Idempotent-Replay", "true")
	w.WriteHeader(rec.ResponseStatus)
	_, _ = w.Write(rec.ResponseBody)
}
