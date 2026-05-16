package healthcheck

// Registry holds the set of Checkers a service registers.
type Registry struct {
	checks []Checker
}

// New constructs a Registry with the given checks.
func New(checks ...Checker) *Registry {
	return &Registry{checks: checks}
}

// Checks returns the registered checks (for test introspection).
func (r *Registry) Checks() []Checker {
	return r.checks
}
