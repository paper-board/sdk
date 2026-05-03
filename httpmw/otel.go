package httpmw

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// OtelHTTP returns a middleware wrapping next with otelhttp.NewHandler. The
// service name names the operation; per-route span name refinement happens
// downstream via chi route patterns.
func OtelHTTP(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, serviceName)
	}
}
