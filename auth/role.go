package auth

// AuthMode identifies the credential format expected by a route.
type AuthMode int

const (
	// JWT indicates Bearer token authentication.
	JWT AuthMode = 1
	// APIKey indicates API key authentication (pbk_(live|test)_* format).
	APIKey AuthMode = 2
)

// Role is the org-scoped membership level returned by identity.
type Role int

const (
	// Owner has full administrative access within an org.
	Owner Role = 1
	// Member has standard access within an org.
	Member Role = 2
)

// Env identifies the credential environment.
type Env string

const (
	// EnvLive is the production credential environment.
	EnvLive Env = "live"
	// EnvTest is the sandbox credential environment.
	EnvTest Env = "test"
)
