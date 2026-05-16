package healthcheck

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const checkTimeout = 500 * time.Millisecond

type response struct {
	Status string            `json:"status"`
	Checks map[string]Result `json:"checks"`
}

// Handler returns an http.HandlerFunc that runs all registered Checkers
// in parallel with a 500ms per-check budget. Returns 200 when every check
// passes, 503 when any check fails.
func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), checkTimeout+250*time.Millisecond)
		defer cancel()

		results := make(map[string]Result, len(r.checks))
		var mu sync.Mutex
		var wg sync.WaitGroup

		for _, c := range r.checks {
			wg.Add(1)
			go func(c Checker) {
				defer wg.Done()
				cctx, ccancel := context.WithTimeout(ctx, checkTimeout)
				defer ccancel()

				start := time.Now()
				err := c.Check(cctx)
				took := time.Since(start)

				res := Result{
					Name:      c.Name(),
					LatencyMs: took.Milliseconds(),
				}
				if err != nil {
					res.Status = "fail"
					res.Err = err.Error()
				} else {
					res.Status = "ok"
				}
				mu.Lock()
				results[c.Name()] = res
				mu.Unlock()
			}(c)
		}
		wg.Wait()

		status := "ready"
		code := http.StatusOK
		for _, res := range results {
			if res.Status != "ok" {
				status = "not_ready"
				code = http.StatusServiceUnavailable
				break
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(response{Status: status, Checks: results})
	}
}
