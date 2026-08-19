package app

import (
	"context"

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

func (r *registrationService) RegisterWithDefaultAccount(ctx context.Context, email string, password string) (*users.User, *wallet.Account, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	txUserRepo := users.NewUserRepository(tx)
	txAccountRepo := wallet.NewAccountRepository(tx)

	userService := users.NewService(txUserRepo)
	accountService := wallet.NewService(txAccountRepo)

	user, err := userService.Register(ctx, email, password)
	if err != nil {
		return nil, nil, err
	}

	account, err := accountService.CreateAccount(ctx, user.Id, wallet.PLN.String())
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return user, account, nil
}
