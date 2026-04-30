package errors

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToGRPCStatus maps a wrapped sentinel error to a gRPC status.
// Used by service handlers to convert internal errors into RPC responses.
//
//	if err := repo.Get(ctx, id); err != nil {
//	    return nil, errors.ToGRPCStatus(err)
//	}
func ToGRPCStatus(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, ErrConflict):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, ErrUnauthorized):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, ErrPermissionDenied):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ErrRateLimited):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, ErrUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

// FromGRPCStatus converts a gRPC status error back to a wrapped sentinel.
// Useful in client code to use errors.Is across the wire.
func FromGRPCStatus(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.NotFound:
		return wrap(ErrNotFound, st.Message())
	case codes.AlreadyExists:
		return wrap(ErrConflict, st.Message())
	case codes.Unauthenticated:
		return wrap(ErrUnauthorized, st.Message())
	case codes.PermissionDenied:
		return wrap(ErrPermissionDenied, st.Message())
	case codes.InvalidArgument:
		return wrap(ErrInvalidInput, st.Message())
	case codes.ResourceExhausted:
		return wrap(ErrRateLimited, st.Message())
	case codes.Unavailable:
		return wrap(ErrUnavailable, st.Message())
	default:
		return wrap(ErrInternal, st.Message())
	}
}
