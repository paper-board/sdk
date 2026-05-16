package healthcheck

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type grpcChecker struct {
	name string
	conn *grpc.ClientConn
}

// GRPC returns a Checker that probes a gRPC ClientConn's connectivity state.
// Returns nil when the conn is Ready or Idle; otherwise returns an error.
// Use this to verify downstream services (e.g., identity-AuthService from agents).
func GRPC(name string, conn *grpc.ClientConn) Checker {
	return &grpcChecker{name: name, conn: conn}
}

func (g *grpcChecker) Name() string { return g.name }

func (g *grpcChecker) Check(ctx context.Context) error {
	state := g.conn.GetState()
	switch state {
	case connectivity.Ready, connectivity.Idle:
		return nil
	default:
		g.conn.Connect()
		if !g.conn.WaitForStateChange(ctx, state) {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("grpc %s: %w", g.name, err)
			}
			return fmt.Errorf("grpc %s: connection state unchanged from %s (current: %s)", g.name, state, g.conn.GetState())
		}
		newState := g.conn.GetState()
		if newState == connectivity.Ready || newState == connectivity.Idle {
			return nil
		}
		return fmt.Errorf("grpc %s: state %s", g.name, newState)
	}
}
