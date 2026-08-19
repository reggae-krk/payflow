package wallet

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServiceCreateAccountValid(t *testing.T) {
	repo := NewFakeAccountRepository()
	service := NewService(repo)

	before := time.Now()
	account, err := service.CreateAccount(context.Background(), 1, "PLN")
	after := time.Now()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if account == nil {
		t.Fatal("expected account, got nil")
	}

	assert.Equal(t, int64(0), account.BalanceMinor)
	assert.Equal(t, PLN, account.Currency)
	assert.Greater(t, account.Id, int64(0))

	margin := 5 * time.Second
	assert.False(t, account.CreatedAt.Before(before.Add(-margin)),
		"CreatedAt too far in the past: %v", account.CreatedAt)
	assert.False(t, account.CreatedAt.After(after.Add(margin)),
		"CreatedAt too far in the future: %v", account.CreatedAt)
}

func TestServiceCreateMultipleAccounts(t *testing.T) {
	repo := NewFakeAccountRepository()
	service := NewService(repo)

	account, err := service.CreateAccount(context.Background(), 1, "PLN")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if account == nil {
		t.Fatal("expected account, got nil")
	}

	assert.Equal(t, int64(0), account.BalanceMinor)
	assert.Equal(t, PLN, account.Currency)
	assert.Greater(t, account.Id, int64(0))

	service.CreateAccount(context.Background(), 1, "PLN")

	accounts, err := repo.GetByUserID(context.Background(), account.UserId)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assert.Equal(t, 2, len(accounts))
}

func TestServiceCreateAccountUnacceptableCurrency(t *testing.T) {
	repo := NewFakeAccountRepository()
	service := NewService(repo)

	_, err := service.CreateAccount(context.Background(), 1, "USD")

	assert.EqualError(t, err, "unsupported currency: USD")
}

func TestServiceGetBalanceValidOwner(t *testing.T) {
	repo := NewFakeAccountRepository()
	service := NewService(repo)

	account, err := service.CreateAccount(context.Background(), 1, "PLN")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if account == nil {
		t.Fatal("expected account, got nil")
	}

	err = repo.AdjustBalance(t.Context(), account.Id, 200)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	balance, err := service.GetBalance(context.Background(), account.Id, account.UserId)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assert.Equal(t, int64(200), balance)

	err = repo.AdjustBalance(t.Context(), account.Id, -300)

	assert.ErrorIs(t, err, ErrInsufficientFunds)
	assert.EqualError(t, err, "insufficient funds")

	balance, _ = service.GetBalance(context.Background(), account.UserId, account.UserId)

	assert.Equal(t, int64(200), balance)

	repo.AdjustBalance(t.Context(), account.Id, -200)
	balance, _ = service.GetBalance(context.Background(), account.UserId, account.UserId)

	assert.Equal(t, int64(0), balance)
}

func TestServiceGetBalanceInvalidOwner(t *testing.T) {
	repo := NewFakeAccountRepository()
	service := NewService(repo)

	account, err := service.CreateAccount(context.Background(), 1, "PLN")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if account == nil {
		t.Fatal("expected account, got nil")
	}

	_, err = service.GetBalance(context.Background(), account.UserId, 154)

	assert.ErrorIs(t, err, ErrForbidden)
	assert.EqualError(t, err, "account does not belong to requesting user")
}

func TestServiceGetBalanceAccountNotFound(t *testing.T) {
	repo := NewFakeAccountRepository()
	service := NewService(repo)

	_, err := service.GetBalance(context.Background(), 1, 1)

	assert.ErrorIs(t, err, ErrNoRows)
	assert.EqualError(t, err, "no rows in result set")
}
