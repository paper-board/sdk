package config

// SetExitForTest swaps the package exit hook for testing. Returns the prior
// hook so tests can restore it.
func SetExitForTest(fn func(string)) func(string) {
	prev := exit
	exit = fn
	return prev
}
