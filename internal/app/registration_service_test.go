package app

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reggae-krk/payflow/internal/testhelpers"
	"github.com/reggae-krk/payflow/internal/users"
	"github.com/reggae-krk/payflow/internal/wallet"
	"github.com/stretchr/testify/assert"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := testhelpers.CreatePostgresContainer(ctx)
	if err != nil {
		log.Fatalf("failed to start postgres container: %v", err)
	}

	pool, err := pgxpool.New(ctx, container.ConnectionString)
	if err != nil {
		log.Fatalf("failed to connect to test db: %v", err)
	}
	testPool = pool

	code := m.Run()

	pool.Close()
	_ = container.Terminate(ctx)

	os.Exit(code)
}

func TestRegistrationServiceRegisterWithDefaultAccount(t *testing.T) {
	t.Run("creates user and account atomically on success", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccountsTable(t, testPool)
			truncateUsersTable(t, testPool)
		})

		service := NewService(testPool)

		user, account, err := service.RegisterWithDefaultAccount(t.Context(), "valid@example.com", "StrongP@ss1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user == nil {
			t.Fatal("expected user, got nil")
		}
		if account == nil {
			t.Fatal("expected account, got nil")
		}

		assert.Equal(t, user.Id, account.UserId)
		assert.Equal(t, int64(0), account.BalanceMinor)

		assert.Equal(t, 1, countUsersRows(t, testPool))
		assert.Equal(t, 1, countAccountsRows(t, testPool))
	})

	t.Run("rolls back everything for invalid email", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccountsTable(t, testPool)
			truncateUsersTable(t, testPool)
		})

		service := NewService(testPool)

		user, account, err := service.RegisterWithDefaultAccount(t.Context(), "not-an-email", "StrongP@ss1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		assert.Nil(t, user)
		assert.Nil(t, account)

		assert.Equal(t, 0, countUsersRows(t, testPool))
		assert.Equal(t, 0, countAccountsRows(t, testPool))
	})

	t.Run("rolls back everything for weak password", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccountsTable(t, testPool)
			truncateUsersTable(t, testPool)
		})

		service := NewService(testPool)

		user, account, err := service.RegisterWithDefaultAccount(t.Context(), "weakpass@example.com", "weak")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		assert.Nil(t, user)
		assert.Nil(t, account)

		assert.Equal(t, 0, countUsersRows(t, testPool))
		assert.Equal(t, 0, countAccountsRows(t, testPool))
	})

	t.Run("rolls back second registration for duplicate email without touching first user's account", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccountsTable(t, testPool)
			truncateUsersTable(t, testPool)
		})

		service := NewService(testPool)

		firstUser, firstAccount, err := service.RegisterWithDefaultAccount(t.Context(), "duplicate@example.com", "StrongP@ss1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		secondUser, secondAccount, err := service.RegisterWithDefaultAccount(t.Context(), "duplicate@example.com", "AnotherP@ss2")
		if !errors.Is(err, users.ErrEmailTaken) {
			t.Errorf("expected ErrEmailTaken, got %v", err)
		}
		assert.Nil(t, secondUser)
		assert.Nil(t, secondAccount)

		assert.Equal(t, 1, countUsersRows(t, testPool))
		assert.Equal(t, 1, countAccountsRows(t, testPool))

		obtainedAccounts, err := wallet.NewAccountRepository(testPool).GetByUserID(t.Context(), firstUser.Id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assert.Len(t, obtainedAccounts, 1)
		assert.Equal(t, firstAccount.Id, obtainedAccounts[0].Id)
	})
}

func truncateUsersTable(t *testing.T, pool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := pool.Exec(ctx, "TRUNCATE users RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("failed to truncate users table: %v", err)
	}
}

func truncateAccountsTable(t *testing.T, pool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := pool.Exec(ctx, "TRUNCATE accounts RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("failed to truncate accounts table: %v", err)
	}
}

func countUsersRows(t *testing.T, pool *pgxpool.Pool) int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count users rows: %v", err)
	}
	return count
}

func countAccountsRows(t *testing.T, pool *pgxpool.Pool) int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM accounts").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count accounts rows: %v", err)
	}
	return count
}