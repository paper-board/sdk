package httpmw

import (
	"net/http"
	"runtime/debug"

	pberrs "github.com/paper-board/sdk/errors"
	pblog "github.com/paper-board/sdk/log"
)

// Recover catches panics from downstream handlers, logs structured (panic
// value + stack + error.kind=panic), and writes the canonical 500 envelope
// via HandleErr. Service name names the error code suffix.
func Recover(svc string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if p := recover(); p != nil {
					pblog.Pkg(r.Context()).Error("panic recovered",
						"error.kind", "panic",
						"panic", p,
						"stack", string(debug.Stack()),
					)
					HandleErr(w, r, svc, pberrs.Wrap(pberrs.ErrInternal, "panic"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
