package httpmw

import (
	"net/http"
	"strings"

	pblog "github.com/paper-board/sdk/log"
)

// Gateway-issued auth header names (Phase 7+).
const (
	HeaderOrgID  = "X-Org-Id"
	HeaderUserID = "X-User-Id"
	HeaderRoles  = "X-Roles"
)

// TrustHeaders reads gateway-issued auth headers and writes them into the
// request ctx. Used Phase 7+ when the gateway terminates auth and downstream
// services trust headers. Pre-Phase-7, headers are usually empty so this is a
// silent no-op.
//
// SECURITY: enable only when fronted by an authenticated gateway. Direct
// internet exposure with TrustHeaders enabled is a confused-deputy risk —
// any caller can claim any org/user.
func TrustHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if v := r.Header.Get(HeaderOrgID); v != "" {
			ctx = pblog.WithOrg(ctx, v)
		}
		if v := r.Header.Get(HeaderUserID); v != "" {
			ctx = pblog.WithUser(ctx, v)
		}
		if v := r.Header.Get(HeaderRoles); v != "" {
			if roles := splitTrim(v); len(roles) > 0 {
				ctx = pblog.WithRoles(ctx, roles)
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
