package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reggae-krk/payflow/internal/users"
	"github.com/reggae-krk/payflow/internal/wallet"
)

type RegistrationService interface {
	RegisterWithDefaultAccount(ctx context.Context, email string, password string) (*users.User, *wallet.Account, error)
}

type registrationService struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *registrationService {
	return &registrationService{pool: pool}
}

const rollbackTimeout = 5 * time.Second

// finishTx is a deferred cleanup helper for transactions.
// It rolls back the transaction using a fresh, short-lived context so the
// rollback can still reach PostgreSQL even if the caller's context was cancelled.
// After a successful Commit, Rollback returns pgx.ErrTxClosed — that is expected
// and silently ignored. Any other rollback error is joined with the original
// operation error so callers see both.
func finishTx(tx pgx.Tx, opErr *error) {
	rbCtx, cancel := context.WithTimeout(context.Background(), rollbackTimeout)
	defer cancel()

	rbErr := tx.Rollback(rbCtx)
	switch {
	case rbErr == nil:
		// Rollback succeeded — transaction was not committed.
	case errors.Is(rbErr, pgx.ErrTxClosed):
		// Expected after a successful Commit; nothing to do.
	default:
		*opErr = errors.Join(*opErr, fmt.Errorf("rollback tx: %w", rbErr))
	}
}

func (r *registrationService) RegisterWithDefaultAccount(ctx context.Context, email string, password string) (user *users.User, account *wallet.Account, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer finishTx(tx, &err)

	txUserRepo := users.NewUserRepository(tx)
	txAccountRepo := wallet.NewAccountRepository(tx)

	userService := users.NewService(txUserRepo)
	accountService := wallet.NewService(txAccountRepo)

	user, err = userService.Register(ctx, email, password)
	if err != nil {
		return nil, nil, err
	}

	account, err = accountService.CreateAccount(ctx, user.Id, wallet.PLN.String())
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return user, account, nil
}
