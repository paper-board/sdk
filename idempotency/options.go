package idempotency

type config struct {
	excludes map[string]struct{}
}

type Option func(*config)

// WithExclude bypasses the middleware for the given routes.
// Format: "METHOD /path" (e.g., "POST /v1/auth/login").
func WithExclude(routes ...string) Option {
	return func(c *config) {
		if c.excludes == nil {
			c.excludes = make(map[string]struct{})
		}
		for _, r := range routes {
			c.excludes[r] = struct{}{}
		}
	}
}
