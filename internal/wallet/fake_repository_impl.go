package wallet

import (
	"context"
	"errors"
	"time"
)

type fakeAccountRepository struct {
	accounts map[int64]*Account
	byUser   map[int64][]*Account
	nextID   int64
}

func NewFakeAccountRepository() *fakeAccountRepository {
	return &fakeAccountRepository{accounts: make(map[int64]*Account),
		byUser: map[int64][]*Account{},
		nextID: 1}
}

func (r *fakeAccountRepository) Create(ctx context.Context, userId int64, currency Currency) (*Account, error) {
	account := &Account{
		Id:        r.nextID,
		UserId:    userId,
		Currency:  currency,
		CreatedAt: time.Now(),
	}
	if !currency.IsValid() {
		return nil, errors.New("")
	}
	r.nextID++
	r.accounts[account.Id] = account
	r.byUser[userId] = append(r.byUser[userId], account)

	return account, nil
}

func (r *fakeAccountRepository) GetByID(ctx context.Context, id int64) (*Account, error) {
	account, ok := r.accounts[id]
	if !ok {
		return nil, ErrNoRows
	}
	return account, nil
}

func (r *fakeAccountRepository) GetByUserID(ctx context.Context, userId int64) ([]*Account, error) {
	accounts, exist := r.byUser[userId]
	if !exist {
		return nil, ErrNoRows
	}
	return accounts, nil
}

func (r *fakeAccountRepository) AdjustBalance(ctx context.Context, id int64, deltaMinor int64) error {
	account, ok := r.accounts[id]
	if !ok {
		return ErrNoRows
	}

	balance := account.BalanceMinor

	if balance+deltaMinor < 0 {
		return ErrInsufficientFunds
	}
	account.BalanceMinor += deltaMinor
	return nil
}

func (r *fakeAccountRepository) GetByIDForUpdate(ctx context.Context, id int64) (*Account, error) {
	return r.GetByID(ctx, id)
}
