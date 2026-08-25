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
	"github.com/stretchr/testify/require"
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

func TestLedgerRepositoryInsert(t *testing.T) {
	t.Run("inserts valid deposit entry", func(t *testing.T) {
		t.Cleanup(func() {
			truncateUsers(t, testPool)
			truncateAccounts(t, testPool)
			truncateLedgerEntries(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		account, err := accountRepo.Create(t.Context(), userId, PLN)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		repo := NewLedgerRepository(testPool)
		entry := LedgerEntry{
			TransferID:    nil,
			AccountID:     account.Id,
			OperationType: OperationDeposit,
			EntryType:     EntryCredit,
			AmountMinor:   50,
		}

		err = repo.Insert(t.Context(), entry)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails with ErrAccountNotFound for nonexistent account", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
		})

		repo := NewLedgerRepository(testPool)
		entry := LedgerEntry{
			TransferID:    nil,
			AccountID:     1,
			OperationType: OperationDeposit,
			EntryType:     EntryCredit,
			AmountMinor:   50,
		}

		err := repo.Insert(t.Context(), entry)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAccountNotFound)
	})

	t.Run("fails with ErrTransferNotFound for wrong transferId", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		account, err := accountRepo.Create(t.Context(), userId, PLN)
		require.NoError(t, err)

		repo := NewLedgerRepository(testPool)
		transferId := int64(999) // nie istnieje, ale account_id jest poprawny
		entry := LedgerEntry{
			TransferID:    &transferId,
			AccountID:     account.Id,
			OperationType: OperationDeposit,
			EntryType:     EntryCredit,
			AmountMinor:   50,
		}

		err = repo.Insert(t.Context(), entry)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTransferNotFound)
	})

	t.Run("fails with ErrInvalidLedgerEntry for wrong AmountMinor", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
		})

		repo := NewLedgerRepository(testPool)
		entry := LedgerEntry{
			TransferID:    nil,
			AccountID:     1,
			OperationType: OperationDeposit,
			EntryType:     EntryCredit,
			AmountMinor:   -50,
		}

		err := repo.Insert(t.Context(), entry)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidLedgerEntry)
	})

	t.Run("fails with ErrInvalidLedgerEntry for wrong EntruType", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
		})

		repo := NewLedgerRepository(testPool)
		entry := LedgerEntry{
			TransferID:    nil,
			AccountID:     1,
			OperationType: OperationDeposit,
			EntryType:     EntryType("invalid"),
			AmountMinor:   -50,
		}

		err := repo.Insert(t.Context(), entry)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidLedgerEntry)
	})

	t.Run("fails with ErrInvalidOperationType for wrong OperationType", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
		})

		repo := NewLedgerRepository(testPool)
		entry := LedgerEntry{
			TransferID:    nil,
			AccountID:     1,
			OperationType: OperationType("invalid"),
			EntryType:     EntryCredit,
			AmountMinor:   -50,
		}

		err := repo.Insert(t.Context(), entry)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidOperationType)
	})
}

func TestTransferRepositoryCreate(t *testing.T) {
	t.Run("inserts valid transfer entry", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userSourceId := insertTestUser(t, testPool)
		userDestinationId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		sourceAccount, err := accountRepo.Create(t.Context(), userSourceId, PLN)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		destinationAccount, err := accountRepo.Create(t.Context(), userDestinationId, PLN)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		repo := NewTransferRepository(testPool)

		transfer, err := repo.Create(t.Context(), sourceAccount.Id, destinationAccount.Id, 100, "some-key")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assert.Equal(t, sourceAccount.Id, transfer.SourceAccountID)
		assert.Equal(t, destinationAccount.Id, transfer.DestinationAccountID)
		assert.Equal(t, int64(100), transfer.AmountMinor)
		assert.Equal(t, "some-key", *transfer.IdempotencyKey)
		assert.Equal(t, "pending", transfer.Status)
	})

	t.Run("fails with ErrDuplicateTransfer for duplicate idempotency key", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userSourceId := insertTestUser(t, testPool)
		userDestinationId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		sourceAccount, _ := accountRepo.Create(t.Context(), userSourceId, PLN)
		destinationAccount, _ := accountRepo.Create(t.Context(), userDestinationId, PLN)

		repo := NewTransferRepository(testPool)

		_, err := repo.Create(t.Context(), sourceAccount.Id, destinationAccount.Id, 100, "duplicate-key")
		require.NoError(t, err)

		_, err = repo.Create(t.Context(), sourceAccount.Id, destinationAccount.Id, 200, "duplicate-key")

		assert.ErrorIs(t, err, ErrDuplicateTransfer)
	})

	t.Run("fails for nonexistent source account", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userDestinationId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		destinationAccount, _ := accountRepo.Create(t.Context(), userDestinationId, PLN)

		repo := NewTransferRepository(testPool)

		_, err := repo.Create(t.Context(), 9999, destinationAccount.Id, 100, "some-other-key")

		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrDuplicateTransfer)
	})

	t.Run("fails for non-positive amount", func(t *testing.T) {
		t.Cleanup(func() {
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userSourceId := insertTestUser(t, testPool)
		userDestinationId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		sourceAccount, _ := accountRepo.Create(t.Context(), userSourceId, PLN)
		destinationAccount, _ := accountRepo.Create(t.Context(), userDestinationId, PLN)

		repo := NewTransferRepository(testPool)

		_, err := repo.Create(t.Context(), sourceAccount.Id, destinationAccount.Id, 0, "zero-amount-key")

		require.Error(t, err)
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

func truncateLedgerEntries(t *testing.T, testPool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := testPool.Exec(ctx, "TRUNCATE ledger_entries RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("failed to truncate ledger_entries table: %v", err)
	}
}

func truncateTransferEntries(t *testing.T, testPool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := testPool.Exec(ctx, "TRUNCATE transfers RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("failed to truncate transfers table: %v", err)
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
