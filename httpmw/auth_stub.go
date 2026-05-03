package httpmw

import (
	"net/http"

	"github.com/google/uuid"

	pblog "github.com/paper-board/sdk/log"
)

// AuthStub stamps a hardcoded org_id into the request ctx. Used in Phase 1.x
// before identity Phase 2 ships. Phase 2 swaps this for real Auth that
// validates JWT/API keys and stamps real org_id/user_id/roles.
//
// SECURITY: AuthStub bypasses authentication entirely. Use only behind a
// trusted boundary (Phase 1.x dev cluster + private network). Phase 2 onward
// MUST replace it with Auth.
func AuthStub(orgID uuid.UUID) func(http.Handler) http.Handler {
	org := orgID.String()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := pblog.WithOrg(r.Context(), org)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
