package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
)

func hashRequest(r *http.Request, body []byte) string {
	h := sha256.New()
	h.Write([]byte(r.Method))
	h.Write([]byte(" "))
	h.Write([]byte(r.URL.Path))
	h.Write([]byte("\n"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}
