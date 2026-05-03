// Package store provides shared Postgres helpers used across paper-board
// services. The primary entry is WithTx, a generic transaction wrapper that
// composes pgx.Tx semantics with strongly-typed return values.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WithTx runs fn inside a database transaction. It begins a tx before fn is
// called; commits on nil error; rolls back on any non-nil error or panic.
// Context cancellation forces rollback. The result of fn is returned on success.
//
// Usage:
//
//	res, err := store.WithTx(ctx, pool, func(tx pgx.Tx) (Result, error) {
//	    if _, err := tx.Exec(ctx, "..."); err != nil {
//	        return Result{}, err
//	    }
//	    return Result{...}, nil
//	})
func WithTx[T any](ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) (T, error)) (T, error) {
	var zero T
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return zero, fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(context.Background())
			panic(p)
		}
	}()

	out, fnErr := fn(tx)
	if fnErr != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return zero, fmt.Errorf("rollback after %w: %v", fnErr, rbErr)
		}
		return zero, fnErr
	}

	if err := tx.Commit(ctx); err != nil {
		return zero, fmt.Errorf("commit: %w", err)
	}
	return out, nil
}
