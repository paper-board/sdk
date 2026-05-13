package auth

import (
	"context"

	"github.com/google/uuid"
)

// AuthCtx is the verified caller identity injected into request context after Require succeeds.
type AuthCtx struct {
	UserID    uuid.UUID
	OrgID     uuid.UUID
	Mode      AuthMode
	AuthKeyID *uuid.UUID // present when Mode=JWT (the signing key kid parsed as UUID)
	APIKeyID  *uuid.UUID // present when Mode=APIKey
	Method    string     // raw method string from identity (e.g. "jwt", "api_key")
	Env       Env
	Role      Role
}

type ctxKey struct{}

// WithContext returns a new context carrying the AuthCtx.
func WithContext(ctx context.Context, a AuthCtx) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

// FromContext extracts the AuthCtx from a request context. The bool reports presence.
func FromContext(ctx context.Context) (AuthCtx, bool) {
	a, ok := ctx.Value(ctxKey{}).(AuthCtx)
	return a, ok
}
