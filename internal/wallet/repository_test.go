package wallet

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reggae-krk/payflow/internal/testhelpers"
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

func TestAccountRepositoryCreate(t *testing.T) {
	t.Run("creates account with valid currency", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		repo := NewAccountRepository(testPool)

		account, err := repo.Create(t.Context(), userId, PLN)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if account.BalanceMinor != 0 {
			t.Errorf("expected balance 0, got %d", account.BalanceMinor)
		}
	})

	t.Run("fails for nonexistent user due to FK constraint", func(t *testing.T) {
		t.Cleanup(func() { truncateAccounts(t, testPool) })
		repo := NewAccountRepository(testPool)

		_, err := repo.Create(t.Context(), 999999, PLN)

		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("fails for invalid currency", func(t *testing.T) {
		t.Cleanup(func() { truncateAccounts(t, testPool) })
		repo := NewAccountRepository(testPool)

		_, err := repo.Create(t.Context(), 1, "USD")

		assert.EqualError(t, err, "ERROR: new row for relation \"accounts\" violates check constraint \"accounts_currency_supported\" (SQLSTATE 23514)")
	})
}

func TestAccountRepositoryGetById(t *testing.T) {
	t.Run("get account with valid accountId", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		repo := NewAccountRepository(testPool)

		account, err := repo.Create(t.Context(), userId, PLN)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		obtainedAccount, err := repo.GetByID(t.Context(), account.Id)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assert.Equal(t, account.Id, obtainedAccount.Id)
		assert.Equal(t, account.UserId, obtainedAccount.UserId)
	})

	t.Run("get account with invalid accountId", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		repo := NewAccountRepository(testPool)

		_, err := repo.GetByID(t.Context(), 9999)

		if !errors.Is(err, ErrNoRows) {
			t.Errorf("expected ErrNoRows, got %v", err)
		}
	})
}

func TestAccountRepositoryGetByUserId(t *testing.T) {
	t.Run("get account with valid userId", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		repo := NewAccountRepository(testPool)

		account, err := repo.Create(t.Context(), userId, PLN)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		obtainedAccount, err := repo.GetByUserID(t.Context(), account.UserId)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assert.Equal(t, account.Id, obtainedAccount[0].Id)
		assert.Equal(t, account.UserId, obtainedAccount[0].UserId)
	})

	t.Run("get account with invalid userId", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		repo := NewAccountRepository(testPool)

		accounts, err := repo.GetByUserID(t.Context(), 9999)

		assert.Nil(t, err)
		assert.Empty(t, accounts)
	})

	t.Run("get get err for crating multiple accounts with valid userId (1 user, 1 account per currency)", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		repo := NewAccountRepository(testPool)

		_, err := repo.Create(t.Context(), userId, PLN)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = repo.Create(t.Context(), userId, PLN)

		assert.EqualError(t, err, "ERROR: duplicate key value violates unique constraint \"accounts_user_currency_unique\" (SQLSTATE 23505)")
	})
}

func TestAccuntRepositoryAdjustBalance(t *testing.T) {
	t.Run("adjust balance with valid userId", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		repo := NewAccountRepository(testPool)

		account, err := repo.Create(t.Context(), userId, PLN)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assert.Equal(t, int64(0), account.BalanceMinor)

		err = repo.AdjustBalance(t.Context(), account.Id, int64(200))

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		obtainedAccount, err := repo.GetByID(t.Context(), account.Id)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assert.Equal(t, int64(200), obtainedAccount.BalanceMinor)
	})

	t.Run("adjust balance with valid userId - insufficient funds ", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		repo := NewAccountRepository(testPool)

		account, err := repo.Create(t.Context(), userId, PLN)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assert.Equal(t, int64(0), account.BalanceMinor)

		err = repo.AdjustBalance(t.Context(), account.Id, int64(-200))

		if !errors.Is(err, ErrInsufficientFunds) {
			t.Errorf("expected ErrInsufficientFunds, got %v", err)
		}

		obtainedAccount, err := repo.GetByID(t.Context(), account.Id)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assert.Equal(t, int64(0), obtainedAccount.BalanceMinor)
	})
}

func truncateUsers(t *testing.T, testPool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := testPool.Exec(ctx, "TRUNCATE users RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("failed to truncate users table: %v", err)
	}
}

func truncateAccounts(t *testing.T, testPool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := testPool.Exec(ctx, "TRUNCATE accounts RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("failed to truncate accounts table: %v", err)
	}
}

func insertTestUser(t *testing.T, pool *pgxpool.Pool) int64 {
	var userId int64

	query := `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id
	`

	email := fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())

	err := pool.QueryRow(context.Background(), query, email, "fake-hash").Scan(&userId)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	return userId
}
