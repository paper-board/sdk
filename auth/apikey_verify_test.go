package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	identityv1 "github.com/paper-board/proto/gen/go/identity/v1"
	"github.com/paper-board/sdk/auth/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	testUserID = uuid.New()
	testOrgID  = uuid.New()
	testKeyID  = uuid.New()
)

func TestVerifyAPIKey_success(t *testing.T) {
	mc := &mock.AuthClient{
		VerifyAPIKeyFn: func(_ context.Context, req *identityv1.VerifyAPIKeyRequest, _ ...grpc.CallOption) (*identityv1.VerifyAPIKeyResponse, error) {
			return &identityv1.VerifyAPIKeyResponse{
				Ctx: &identityv1.AuthContext{
					UserId:   testUserID.String(),
					OrgId:    testOrgID.String(),
					Mode:     identityv1.AuthMode_AUTH_MODE_API_KEY,
					ApiKeyId: testKeyID.String(),
					Method:   "api_key",
					Env:      identityv1.Env_ENV_LIVE,
					Role:     identityv1.Role_ROLE_OWNER,
				},
			}, nil
		},
	}

	ac, err := verifyAPIKey(context.Background(), mc, "pbk_live_TESTKEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ac.UserID != testUserID {
		t.Errorf("UserID = %v; want %v", ac.UserID, testUserID)
	}
	if ac.OrgID != testOrgID {
		t.Errorf("OrgID = %v; want %v", ac.OrgID, testOrgID)
	}
	if ac.Mode != APIKey {
		t.Errorf("Mode = %v; want APIKey", ac.Mode)
	}
	if ac.APIKeyID == nil || *ac.APIKeyID != testKeyID {
		t.Errorf("APIKeyID = %v; want %v", ac.APIKeyID, testKeyID)
	}
	if ac.Env != EnvLive {
		t.Errorf("Env = %v; want EnvLive", ac.Env)
	}
	if ac.Role != Owner {
		t.Errorf("Role = %v; want Owner", ac.Role)
	}
}

func TestVerifyAPIKey_status_to_sentinel(t *testing.T) {
	cases := []struct {
		name       string
		grpcCode   codes.Code
		wantSentinel error
	}{
		{
			name:         "unauthenticated",
			grpcCode:     codes.Unauthenticated,
			wantSentinel: ErrUnauthenticated,
		},
		{
			name:         "permission denied",
			grpcCode:     codes.PermissionDenied,
			wantSentinel: ErrInsufficientRole,
		},
		{
			name:         "internal",
			grpcCode:     codes.Internal,
			wantSentinel: nil, // not a sentinel — just a wrapped error
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mc := &mock.AuthClient{
				VerifyAPIKeyFn: func(_ context.Context, _ *identityv1.VerifyAPIKeyRequest, _ ...grpc.CallOption) (*identityv1.VerifyAPIKeyResponse, error) {
					return nil, status.Error(tc.grpcCode, tc.name)
				},
			}
			_, err := verifyAPIKey(context.Background(), mc, "pbk_live_TESTKEY")
			if err == nil {
				t.Fatal("expected error; got nil")
			}
			if tc.wantSentinel != nil && !errors.Is(err, tc.wantSentinel) {
				t.Errorf("errors.Is(%v) = false; want true for %v", err, tc.wantSentinel)
			}
		})
	}
}

func TestVerifyAPIKey_nil_ctx_returns_unauthenticated(t *testing.T) {
	mc := &mock.AuthClient{
		VerifyAPIKeyFn: func(_ context.Context, _ *identityv1.VerifyAPIKeyRequest, _ ...grpc.CallOption) (*identityv1.VerifyAPIKeyResponse, error) {
			return &identityv1.VerifyAPIKeyResponse{Ctx: nil}, nil
		},
	}
	_, err := verifyAPIKey(context.Background(), mc, "pbk_live_TESTKEY")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("got %v; want ErrUnauthenticated", err)
	}
}

func TestVerifyAPIKey_env_test(t *testing.T) {
	mc := &mock.AuthClient{
		VerifyAPIKeyFn: func(_ context.Context, _ *identityv1.VerifyAPIKeyRequest, _ ...grpc.CallOption) (*identityv1.VerifyAPIKeyResponse, error) {
			return &identityv1.VerifyAPIKeyResponse{
				Ctx: &identityv1.AuthContext{
					UserId: testUserID.String(),
					OrgId:  testOrgID.String(),
					Env:    identityv1.Env_ENV_TEST,
					Role:   identityv1.Role_ROLE_MEMBER,
				},
			}, nil
		},
	}
	ac, err := verifyAPIKey(context.Background(), mc, "pbk_test_TESTKEY")
	if err != nil {
		t.Fatal(err)
	}
	if ac.Env != EnvTest {
		t.Errorf("Env = %v; want EnvTest", ac.Env)
	}
	if ac.Role != Member {
		t.Errorf("Role = %v; want Member", ac.Role)
	}
}
