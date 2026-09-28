package app

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
	tx := &fakeTx{rollbackErr: pgx.ErrTxClosed}
	var opErr error

	finishTx(tx, &opErr)

	assert.NoError(t, opErr)
}

func TestFinishTx_RollbackAfterOperationError(t *testing.T) {
	tx := &fakeTx{rollbackErr: nil}
	opErr := errors.New("some business error")

	finishTx(tx, &opErr)

	require.Error(t, opErr)
	assert.Equal(t, "some business error", opErr.Error())
}

func TestFinishTx_RollbackError_JoinedWithOperationError(t *testing.T) {
	tx := &fakeTx{rollbackErr: errors.New("connection reset")}
	originalErr := errors.New("forbidden")

	finishTx(tx, &originalErr)

	require.Error(t, originalErr)
	assert.Contains(t, originalErr.Error(), "forbidden")
	assert.Contains(t, originalErr.Error(), "rollback tx")
	assert.Contains(t, originalErr.Error(), "connection reset")
}

func TestFinishTx_RollbackError_NoOriginalError(t *testing.T) {
	tx := &fakeTx{rollbackErr: errors.New("broken pipe")}
	var opErr error

	finishTx(tx, &opErr)

	require.Error(t, opErr)
	assert.Contains(t, opErr.Error(), "rollback tx")
	assert.Contains(t, opErr.Error(), "broken pipe")
}

