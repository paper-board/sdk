package healthcheck

import "net/http"

// IsProbePath returns true for k8s probe endpoints. Use this in
// service middleware to skip logger + otel for probe traffic
// (probes hit every 10s; logging them is spam).
func IsProbePath(r *http.Request) bool {
	switch r.URL.Path {
	case "/livez", "/readyz", "/healthz":
		return true
	}
	return false
}
