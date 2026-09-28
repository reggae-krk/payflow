package db

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTx struct {
	rollbackErr    error
	capturedCtx    context.Context
	rollbackCtxErr error
}

func (f *fakeTx) Rollback(ctx context.Context) error {
	f.capturedCtx = ctx
	f.rollbackCtxErr = ctx.Err()
	return f.rollbackErr
}

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

func TestFinishTx_RollbackAfterSuccessfulCommit(t *testing.T) {
	tx := &fakeTx{rollbackErr: pgx.ErrTxClosed}
	var opErr error

	FinishTx(context.Background(), tx, &opErr)

	assert.NoError(t, opErr)
}

func TestFinishTx_RollbackAfterOperationError(t *testing.T) {
	tx := &fakeTx{rollbackErr: nil}
	originalErr := errors.New("business error")
	opErr := originalErr

	FinishTx(context.Background(), tx, &opErr)

	require.Error(t, opErr)
	assert.ErrorIs(t, opErr, originalErr)
	assert.Equal(t, "business error", opErr.Error())
}

func TestFinishTx_RollbackError_JoinedWithOperationError(t *testing.T) {
	rollbackErr := errors.New("connection reset")
	tx := &fakeTx{rollbackErr: rollbackErr}
	originalErr := errors.New("forbidden")
	opErr := originalErr

	FinishTx(context.Background(), tx, &opErr)

	require.Error(t, opErr)
	assert.ErrorIs(t, opErr, originalErr)
	assert.ErrorIs(t, opErr, rollbackErr)
	assert.Contains(t, opErr.Error(), "forbidden")
	assert.Contains(t, opErr.Error(), "rollback tx")
	assert.Contains(t, opErr.Error(), "connection reset")
}

func TestFinishTx_RollbackError_NoOriginalError(t *testing.T) {
	rollbackErr := errors.New("broken pipe")
	tx := &fakeTx{rollbackErr: rollbackErr}
	var opErr error

	FinishTx(context.Background(), tx, &opErr)

	require.Error(t, opErr)
	assert.ErrorIs(t, opErr, rollbackErr)
	assert.Contains(t, opErr.Error(), "rollback tx")
	assert.Contains(t, opErr.Error(), "broken pipe")
}

func TestFinishTx_DefensiveNilChecks(t *testing.T) {
	t.Run("nil tx does not panic", func(t *testing.T) {
		var opErr error
		assert.NotPanics(t, func() {
			FinishTx(context.Background(), nil, &opErr)
		})
		assert.NoError(t, opErr)
	})

	t.Run("nil opErr does not panic on rollback failure", func(t *testing.T) {
		tx := &fakeTx{rollbackErr: errors.New("disk full")}
		assert.NotPanics(t, func() {
			FinishTx(context.Background(), tx, nil)
		})
	})

	t.Run("nil ctx falls back to background and executes rollback", func(t *testing.T) {
		tx := &fakeTx{rollbackErr: nil}
		var opErr error
		assert.NotPanics(t, func() {
			FinishTx(nil, tx, &opErr)
		})
		assert.NoError(t, opErr)
		require.NotNil(t, tx.capturedCtx)
	})
}

func TestFinishTx_PreservesContextValuesAndDetachesCancellation(t *testing.T) {
	type ctxKey string
	const key ctxKey = "trace-id"

	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), key, "trace-12345"))
	cancel() // cancel parent context

	tx := &fakeTx{rollbackErr: nil}
	var opErr error

	FinishTx(ctx, tx, &opErr)

	require.NotNil(t, tx.capturedCtx)
	assert.Nil(t, tx.rollbackCtxErr, "rollback context must not be cancelled during rollback execution")
	assert.Equal(t, "trace-12345", tx.capturedCtx.Value(key), "context values must be preserved")
}
