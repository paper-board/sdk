// Package httpclient provides a default *http.Client for outbound HTTP, wrapped
// with otelhttp.NewTransport so spans propagate W3C TraceContext.
package httpclient

import (
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Default returns a fresh *http.Client with the given total timeout. Connect
// timeout is fixed at 5s; idle connections recycle after 90s.
//
// Each call returns a new client. http.Client is concurrency-safe, so callers
// MAY share the result across goroutines but MUST NOT mutate it after use.
func Default(timeout time.Duration) *http.Client {
	base := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Transport: otelhttp.NewTransport(base),
		Timeout:   timeout,
	}
}
