package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	identityv1 "github.com/paper-board/proto/gen/go/identity/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// verifyAPIKey calls identity to verify the raw API key, returning a populated AuthCtx.
// gRPC status codes are mapped to package sentinels:
//
//	Unauthenticated  → ErrUnauthenticated
//	PermissionDenied → ErrInsufficientRole
//	other            → wrapped through
func verifyAPIKey(ctx context.Context, client identityv1.AuthServiceClient, raw string) (AuthCtx, error) {
	resp, err := client.VerifyAPIKey(ctx, &identityv1.VerifyAPIKeyRequest{ApiKey: raw})
	if err != nil {
		st, ok := status.FromError(err)
		if !ok {
			return AuthCtx{}, fmt.Errorf("auth: verify api key: %w", err)
		}
		switch st.Code() {
		case codes.Unauthenticated:
			return AuthCtx{}, ErrUnauthenticated
		case codes.PermissionDenied:
			return AuthCtx{}, ErrInsufficientRole
		default:
			return AuthCtx{}, fmt.Errorf("auth: verify api key: %w", err)
		}
	}

	ac := resp.GetCtx()
	if ac == nil {
		return AuthCtx{}, ErrUnauthenticated
	}

	userID, err := uuid.Parse(ac.GetUserId())
	if err != nil {
		return AuthCtx{}, fmt.Errorf("%w: invalid user_id", ErrUnauthenticated)
	}
	orgID, err := uuid.Parse(ac.GetOrgId())
	if err != nil {
		return AuthCtx{}, fmt.Errorf("%w: invalid org_id", ErrUnauthenticated)
	}

	var apiKeyID *uuid.UUID
	if raw := ac.GetApiKeyId(); raw != "" {
		id, err := uuid.Parse(raw)
		if err == nil {
			apiKeyID = &id
		}
	}

	env, ok := protoEnvToEnv(ac.GetEnv())
	if !ok {
		return AuthCtx{}, fmt.Errorf("%w: invalid env in identity response", ErrUnauthenticated)
	}
	role, ok := protoRoleToRole(ac.GetRole())
	if !ok {
		return AuthCtx{}, fmt.Errorf("%w: invalid role in identity response", ErrUnauthenticated)
	}

	return AuthCtx{
		UserID:   userID,
		OrgID:    orgID,
		Mode:     APIKey,
		APIKeyID: apiKeyID,
		Method:   "api_key",
		Env:      env,
		Role:     role,
	}, nil
}

func protoEnvToEnv(e identityv1.Env) (Env, bool) {
	switch e {
	case identityv1.Env_ENV_LIVE:
		return EnvLive, true
	case identityv1.Env_ENV_TEST:
		return EnvTest, true
	default:
		return "", false
	}
}

func protoRoleToRole(r identityv1.Role) (Role, bool) {
	switch r {
	case identityv1.Role_ROLE_OWNER:
		return Owner, true
	case identityv1.Role_ROLE_MEMBER:
		return Member, true
	default:
		return 0, false
	}
}
