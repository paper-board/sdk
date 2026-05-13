package auth

import "regexp"

// apiKeyRE matches the canonical paper-board API key form pbk_(live|test)_<crockford32>.
// Duplicated from paper-board/identity internal/core/apikey.formatRE intentionally:
// sdk cannot import identity internal packages (cyclic dep risk + leaks identity internals).
// If identity's regex changes, update here too.
var apiKeyRE = regexp.MustCompile(`^pbk_(live|test)_[A-HJKMNP-TV-Z2-9]{32}$`)

// detect classifies a bearer token as APIKey if it matches the canonical form, else JWT.
func detect(token string) AuthMode {
	if apiKeyRE.MatchString(token) {
		return APIKey
	}
	return JWT
}
