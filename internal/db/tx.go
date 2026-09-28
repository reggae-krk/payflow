package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const rollbackTimeout = 5 * time.Second

// FinishTx is a deferred cleanup helper for transactions.
// It rolls back the transaction using a detached context (preserving original context values
// while resetting cancellation) with a 5-second timeout, ensuring the rollback reaches PostgreSQL
// even if the caller's context was cancelled or timed out.
//
// After a successful Commit, Rollback returns pgx.ErrTxClosed — this is expected and silently ignored.
// Any unexpected rollback error is joined with the original operation error (*opErr) so callers see both.
// It is nil-safe against nil tx, nil ctx, and nil opErr.
func FinishTx(ctx context.Context, tx pgx.Tx, opErr *error) {
	if tx == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	rbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
	defer cancel()

	rbErr := tx.Rollback(rbCtx)
	switch {
	case rbErr == nil:
		// Rollback succeeded — transaction was not committed.
	case errors.Is(rbErr, pgx.ErrTxClosed):
		// Expected after a successful Commit; nothing to do.
	default:
		if opErr != nil {
			*opErr = errors.Join(*opErr, fmt.Errorf("rollback tx: %w", rbErr))
		}
	}
}