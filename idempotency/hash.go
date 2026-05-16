package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
)

func hashRequest(r *http.Request, body []byte) string {
	h := sha256.New()
	io.WriteString(h, r.Method)
	io.WriteString(h, " ")
	io.WriteString(h, r.URL.Path)
	io.WriteString(h, "\n")
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}
