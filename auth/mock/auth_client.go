// Package mock provides test doubles for the identity gRPC AuthService.
package mock

import (
	"context"
	"fmt"

	identityv1 "github.com/paper-board/proto/gen/go/identity/v1"
	"google.golang.org/grpc"
)

// AuthClient is a stub implementation of identityv1.AuthServiceClient for tests.
// Each RPC dispatches to the corresponding Fn field. Unset Fn returns a generic error.
type AuthClient struct {
	VerifyAPIKeyFn func(context.Context, *identityv1.VerifyAPIKeyRequest, ...grpc.CallOption) (*identityv1.VerifyAPIKeyResponse, error)
	VerifyJWTFn    func(context.Context, *identityv1.VerifyJWTRequest, ...grpc.CallOption) (*identityv1.VerifyJWTResponse, error)
	IssueJWTFn     func(context.Context, *identityv1.IssueJWTRequest, ...grpc.CallOption) (*identityv1.IssueJWTResponse, error)
	GetPublicKeyFn func(context.Context, *identityv1.GetPublicKeyRequest, ...grpc.CallOption) (*identityv1.GetPublicKeyResponse, error)
}

// VerifyAPIKey dispatches to VerifyAPIKeyFn if set.
func (m *AuthClient) VerifyAPIKey(ctx context.Context, in *identityv1.VerifyAPIKeyRequest, opts ...grpc.CallOption) (*identityv1.VerifyAPIKeyResponse, error) {
	if m.VerifyAPIKeyFn != nil {
		return m.VerifyAPIKeyFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("mock.AuthClient.VerifyAPIKey: no Fn set")
}

// VerifyJWT dispatches to VerifyJWTFn if set.
func (m *AuthClient) VerifyJWT(ctx context.Context, in *identityv1.VerifyJWTRequest, opts ...grpc.CallOption) (*identityv1.VerifyJWTResponse, error) {
	if m.VerifyJWTFn != nil {
		return m.VerifyJWTFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("mock.AuthClient.VerifyJWT: no Fn set")
}

// IssueJWT dispatches to IssueJWTFn if set.
func (m *AuthClient) IssueJWT(ctx context.Context, in *identityv1.IssueJWTRequest, opts ...grpc.CallOption) (*identityv1.IssueJWTResponse, error) {
	if m.IssueJWTFn != nil {
		return m.IssueJWTFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("mock.AuthClient.IssueJWT: no Fn set")
}

// GetPublicKey dispatches to GetPublicKeyFn if set.
func (m *AuthClient) GetPublicKey(ctx context.Context, in *identityv1.GetPublicKeyRequest, opts ...grpc.CallOption) (*identityv1.GetPublicKeyResponse, error) {
	if m.GetPublicKeyFn != nil {
		return m.GetPublicKeyFn(ctx, in, opts...)
	}
	return nil, fmt.Errorf("mock.AuthClient.GetPublicKey: no Fn set")
}
