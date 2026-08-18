package wallet

import (
	"context"
)

type AccountService interface {
	CreateAccount(ctx context.Context, userId int64, currency string) (*Account, error)
	GetBalance(ctx context.Context, accountId int64) (int64, error)
}

type service struct {
	repo AccountRepository
}

func NewService(repo AccountRepository) *service {
	return &service{repo: repo}
}

func (s *service) CreateAccount(ctx context.Context, userId int64, curr string) (*Account, error) {
	currency, err := ParseCurrency(curr)

	if err != nil {
		return nil, err
	}

	account, err := s.repo.Create(ctx, userId, currency)

	if err != nil {
		return nil, err
	}
	return account, nil
}

func (s *service) GetBalance(ctx context.Context, accountId int64) (int64, error) {
	account, err := s.repo.GetByID(ctx, accountId)

	if err != nil {
		return 0, err
	}

	return account.BalanceMinor, nil
}
