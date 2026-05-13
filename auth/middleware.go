// Package auth provides per-route HTTP middleware for verifying caller identity
// against the paper-board identity gRPC AuthService.
//
// Usage (Option C — separate config wiring from per-route factories):
//
//	cfg := auth.New(auth.WithClient(grpcConn))
//	router.Use(cfg.Require(auth.JWT, auth.APIKey))
//	adminRoutes.Use(cfg.RequireRole(auth.Owner))
package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	identityv1 "github.com/paper-board/proto/gen/go/identity/v1"
)

// Config holds the shared dependencies for the auth middleware factories.
// Create with New; then call Require / RequireRole on routes.
type Config struct {
	client      identityv1.AuthServiceClient
	store       *Keystore
	keystoreTTL time.Duration
}

// Option mutates Config during construction.
type Option func(*Config)

// WithClient injects the identity gRPC client. Required for non-mock usage.
func WithClient(c identityv1.AuthServiceClient) Option {
	return func(cfg *Config) { cfg.client = c }
}

// WithKeystore injects a custom Keystore (defaults to a fresh in-memory one built from WithClient).
func WithKeystore(s *Keystore) Option {
	return func(cfg *Config) { cfg.store = s }
}

// WithKeystoreTTL overrides the default 5min Keystore TTL.
// Ignored if WithKeystore is also supplied.
func WithKeystoreTTL(d time.Duration) Option {
	return func(cfg *Config) { cfg.keystoreTTL = d }
}

// New constructs a Config from the supplied options.
// WithClient is required unless WithKeystore is supplied (e.g. in tests).
func New(opts ...Option) *Config {
	cfg := &Config{}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.store == nil && cfg.client != nil {
		if cfg.keystoreTTL > 0 {
			cfg.store = NewKeystoreWithTTL(cfg.client, cfg.keystoreTTL)
		} else {
			cfg.store = NewKeystore(cfg.client)
		}
	}
	return cfg
}

// Require returns middleware that accepts only requests carrying one of the
// listed AuthModes. Modes are an OR set — order is irrelevant.
// The verified AuthCtx is injected into the request context for downstream handlers.
//
// Respond policy:
//
//	missing header     → 401
//	non-Bearer scheme  → 401
//	mode not accepted  → 401
//	verify failure     → 401 (or 403 for ErrInsufficientRole)
func (cfg *Config) Require(modes ...AuthMode) func(http.Handler) http.Handler {
	allowed := make(map[AuthMode]bool, len(modes))
	for _, m := range modes {
		allowed[m] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hdr := r.Header.Get("Authorization")
			if hdr == "" {
				writeAuthErr(w, http.StatusUnauthorized, ErrMissingHeader.Error())
				return
			}
			if !strings.HasPrefix(hdr, "Bearer ") {
				writeAuthErr(w, http.StatusUnauthorized, ErrInvalidScheme.Error())
				return
			}
			tok := strings.TrimPrefix(hdr, "Bearer ")

			mode := detect(tok)
			if !allowed[mode] {
				writeAuthErr(w, http.StatusUnauthorized, ErrUnauthenticated.Error())
				return
			}

			var (
				ac  AuthCtx
				err error
			)
			switch mode {
			case APIKey:
				ac, err = verifyAPIKey(r.Context(), cfg.client, tok)
			case JWT:
				ac, err = verifyJWT(r.Context(), cfg.store, tok)
			}
			if err != nil {
				if errors.Is(err, ErrInsufficientRole) {
					writeAuthErr(w, http.StatusForbidden, err.Error())
					return
				}
				writeAuthErr(w, http.StatusUnauthorized, err.Error())
				return
			}

			next.ServeHTTP(w, r.WithContext(WithContext(r.Context(), ac)))
		})
	}
}

// RequireRole returns middleware that gates routes by role.
// Must run AFTER Require in the chain (depends on AuthCtx already in context).
// Roles are an OR set — any one of the listed roles passes.
func (cfg *Config) RequireRole(roles ...Role) func(http.Handler) http.Handler {
	allowed := make(map[Role]bool, len(roles))
	for _, ro := range roles {
		allowed[ro] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := FromContext(r.Context())
			if !ok {
				writeAuthErr(w, http.StatusUnauthorized, ErrUnauthenticated.Error())
				return
			}
			if !allowed[ac.Role] {
				writeAuthErr(w, http.StatusForbidden, ErrInsufficientRole.Error())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeAuthErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":{"message":"` + msg + `"}}`))
}
