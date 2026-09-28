package wallet

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTx is a minimal pgx.Tx stub used only for testing finishTx.
// It does not talk to PostgreSQL; it only returns pre-configured errors
// from Rollback. All other methods panic if called.
type fakeTx struct {
	rollbackErr error
}

func (f *fakeTx) Rollback(context.Context) error { return f.rollbackErr }

// The remaining methods satisfy the pgx.Tx interface but are not exercised by
// finishTx. They panic so any accidental call is caught immediately.
func (f *fakeTx) Begin(context.Context) (pgx.Tx, error) { panic("fakeTx: Begin not expected") }
func (f *fakeTx) Commit(context.Context) error          { panic("fakeTx: Commit not expected") }
func (f *fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("fakeTx: CopyFrom not expected")
}
func (f *fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	panic("fakeTx: SendBatch not expected")
}
func (f *fakeTx) LargeObjects() pgx.LargeObjects         { panic("fakeTx: LargeObjects not expected") }
func (f *fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("fakeTx: Prepare not expected")
}
func (f *fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	panic("fakeTx: Exec not expected")
}
func (f *fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("fakeTx: Query not expected")
}
func (f *fakeTx) QueryRow(context.Context, string, ...any) pgx.Row {
	panic("fakeTx: QueryRow not expected")
}
func (f *fakeTx) Conn() *pgx.Conn { panic("fakeTx: Conn not expected") }

// These tests verify the finishTx helper in isolation. They exercise only
// the rollback path — the fake never touches a real database connection.

func TestFinishTx_RollbackAfterSuccessfulCommit(t *testing.T) {
	// After a successful Commit, pgx returns ErrTxClosed on Rollback.
	// finishTx must treat this as a no-op and leave opErr unchanged.
	tx := &fakeTx{rollbackErr: pgx.ErrTxClosed}
	var opErr error

	finishTx(tx, &opErr)

	assert.NoError(t, opErr)
}

func TestFinishTx_RollbackAfterOperationError(t *testing.T) {
	// When the operation failed before Commit, Rollback succeeds (nil error).
	// finishTx must preserve the original operation error.
	tx := &fakeTx{rollbackErr: nil}
	opErr := errors.New("some business error")

	finishTx(tx, &opErr)

	require.Error(t, opErr)
	assert.Equal(t, "some business error", opErr.Error())
}

func TestFinishTx_RollbackError_JoinedWithOperationError(t *testing.T) {
	// When both the operation and the rollback fail, finishTx must join
	// both errors so the original cause is not lost.
	rollbackErr := errors.New("connection reset")
	tx := &fakeTx{rollbackErr: rollbackErr}
	originalErr := errors.New("forbidden")
	opErr := originalErr

	finishTx(tx, &opErr)

	require.Error(t, opErr)
	assert.ErrorIs(t, opErr, originalErr)                      // it's a joined error
	assert.ErrorIs(t, opErr, rollbackErr)                      // underlying rollback cause
	assert.Contains(t, opErr.Error(), "forbidden")             // original preserved
	assert.Contains(t, opErr.Error(), "rollback tx")           // rollback context added
	assert.Contains(t, opErr.Error(), "connection reset")      // underlying rollback cause
}

func TestFinishTx_RollbackError_NoOriginalError(t *testing.T) {
	// When the operation succeeded (opErr == nil) but rollback fails with
	// an unexpected error, finishTx must surface it.
	tx := &fakeTx{rollbackErr: errors.New("broken pipe")}
	var opErr error

	finishTx(tx, &opErr)

	require.Error(t, opErr)
	assert.Contains(t, opErr.Error(), "rollback tx")
	assert.Contains(t, opErr.Error(), "broken pipe")
}

