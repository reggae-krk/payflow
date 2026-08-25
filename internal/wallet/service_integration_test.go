package wallet

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceTransferIntegration(t *testing.T) {
	t.Run("moves funds between two accounts atomically", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTransferEntries(t, testPool)
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userSourceId := insertTestUser(t, testPool)
		userDestinationId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(NewAccountRepository(testPool), testPool)

		sourceAccount, err := accountRepo.Create(t.Context(), userSourceId, PLN)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		destinationAccount, err := accountRepo.Create(t.Context(), userDestinationId, PLN)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = accountRepo.AdjustBalance(t.Context(), sourceAccount.Id, int64(1500))

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		transferReq := TransferRequest{
			SourceAccountID:      sourceAccount.Id,
			DestinationAccountID: destinationAccount.Id,
			RequestingUserID:     sourceAccount.Id,
			AmountMinor:          500,
		}

		transfer, err := service.Transfer(t.Context(), transferReq, "some-key")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assert.Equal(t, sourceAccount.Id, transfer.SourceAccountID)
		assert.Equal(t, destinationAccount.Id, transfer.DestinationAccountID)
		assert.Equal(t, int64(500), transfer.AmountMinor)
		assert.Equal(t, "some-key", *transfer.IdempotencyKey)
		assert.Equal(t, "completed", transfer.Status)

		updatedSource, err := accountRepo.GetByID(t.Context(), sourceAccount.Id)
		require.NoError(t, err)

		updatedDestination, err := accountRepo.GetByID(t.Context(), destinationAccount.Id)
		require.NoError(t, err)

		assert.Equal(t, int64(1000), updatedSource.BalanceMinor)
		assert.Equal(t, int64(500), updatedDestination.BalanceMinor)
	})

	t.Run("fails with ErrForbidden when requesting user does not own source account", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTransferEntries(t, testPool)
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userSourceId := insertTestUser(t, testPool)
		userDestinationId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(NewAccountRepository(testPool), testPool)

		sourceAccount, err := accountRepo.Create(t.Context(), userSourceId, PLN)
		require.NoError(t, err)
		destinationAccount, err := accountRepo.Create(t.Context(), userDestinationId, PLN)
		require.NoError(t, err)

		err = accountRepo.AdjustBalance(t.Context(), sourceAccount.Id, int64(1500))
		require.NoError(t, err)

		transferReq := TransferRequest{
			SourceAccountID:      sourceAccount.Id,
			DestinationAccountID: destinationAccount.Id,
			RequestingUserID:     999999,
			AmountMinor:          500,
		}

		transfer, err := service.Transfer(t.Context(), transferReq, "forbidden-key")

		assert.ErrorIs(t, err, ErrForbidden)
		assert.Nil(t, transfer)

		unchangedSource, err := accountRepo.GetByID(t.Context(), sourceAccount.Id)
		require.NoError(t, err)
		assert.Equal(t, int64(1500), unchangedSource.BalanceMinor)
	})

	t.Run("fails with ErrInvalidTransfer for same source and destination account", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTransferEntries(t, testPool)
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userSourceId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(NewAccountRepository(testPool), testPool)

		sourceAccount, err := accountRepo.Create(t.Context(), userSourceId, PLN)
		require.NoError(t, err)

		err = accountRepo.AdjustBalance(t.Context(), sourceAccount.Id, int64(1500))
		require.NoError(t, err)

		transferReq := TransferRequest{
			SourceAccountID:      sourceAccount.Id,
			DestinationAccountID: sourceAccount.Id,
			RequestingUserID:     userSourceId,
			AmountMinor:          500,
		}

		transfer, err := service.Transfer(t.Context(), transferReq, "same-account-key")

		assert.ErrorIs(t, err, ErrInvalidTransfer)
		assert.Nil(t, transfer)

		unchangedSource, err := accountRepo.GetByID(t.Context(), sourceAccount.Id)
		require.NoError(t, err)
		assert.Equal(t, int64(1500), unchangedSource.BalanceMinor)
	})

	t.Run("fails with ErrInsufficientFunds and rolls back when balance too low", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTransferEntries(t, testPool)
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userSourceId := insertTestUser(t, testPool)
		userDestinationId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(NewAccountRepository(testPool), testPool)

		sourceAccount, err := accountRepo.Create(t.Context(), userSourceId, PLN)
		require.NoError(t, err)
		destinationAccount, err := accountRepo.Create(t.Context(), userDestinationId, PLN)
		require.NoError(t, err)

		err = accountRepo.AdjustBalance(t.Context(), sourceAccount.Id, int64(100))

		transferReq := TransferRequest{
			SourceAccountID:      sourceAccount.Id,
			DestinationAccountID: destinationAccount.Id,
			RequestingUserID:     userSourceId,
			AmountMinor:          500,
		}

		transfer, err := service.Transfer(t.Context(), transferReq, "insufficient-key")

		assert.ErrorIs(t, err, ErrInsufficientFunds)
		assert.Nil(t, transfer)

		unchangedSource, err := accountRepo.GetByID(t.Context(), sourceAccount.Id)
		require.NoError(t, err)
		assert.Equal(t, int64(100), unchangedSource.BalanceMinor)

		unchangedDestination, err := accountRepo.GetByID(t.Context(), destinationAccount.Id)
		require.NoError(t, err)
		assert.Equal(t, int64(0), unchangedDestination.BalanceMinor)
	})

	t.Run("fails with ErrAccountNotFound for nonexistent destination account", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTransferEntries(t, testPool)
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userSourceId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(NewAccountRepository(testPool), testPool)

		sourceAccount, err := accountRepo.Create(t.Context(), userSourceId, PLN)
		require.NoError(t, err)

		err = accountRepo.AdjustBalance(t.Context(), sourceAccount.Id, int64(1500))
		require.NoError(t, err)

		transferReq := TransferRequest{
			SourceAccountID:      sourceAccount.Id,
			DestinationAccountID: 9999999,
			RequestingUserID:     userSourceId,
			AmountMinor:          500,
		}

		transfer, err := service.Transfer(t.Context(), transferReq, "no-destination-key")

		assert.ErrorIs(t, err, ErrAccountNotFound)
		assert.Nil(t, transfer)

		unchangedSource, err := accountRepo.GetByID(t.Context(), sourceAccount.Id)
		require.NoError(t, err)
		assert.Equal(t, int64(1500), unchangedSource.BalanceMinor)
	})

	t.Run("fails with ErrDuplicateTransfer for reused idempotency key and does not move funds twice", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTransferEntries(t, testPool)
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userSourceId := insertTestUser(t, testPool)
		userDestinationId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(NewAccountRepository(testPool), testPool)

		sourceAccount, err := accountRepo.Create(t.Context(), userSourceId, PLN)
		require.NoError(t, err)
		destinationAccount, err := accountRepo.Create(t.Context(), userDestinationId, PLN)
		require.NoError(t, err)

		err = accountRepo.AdjustBalance(t.Context(), sourceAccount.Id, int64(1500))
		require.NoError(t, err)

		transferReq := TransferRequest{
			SourceAccountID:      sourceAccount.Id,
			DestinationAccountID: destinationAccount.Id,
			RequestingUserID:     userSourceId,
			AmountMinor:          500,
		}

		_, err = service.Transfer(t.Context(), transferReq, "reused-key")
		require.NoError(t, err)

		transfer, err := service.Transfer(t.Context(), transferReq, "reused-key")

		assert.ErrorIs(t, err, ErrDuplicateTransfer)
		assert.Nil(t, transfer)

		sourceAfterSecondAttempt, err := accountRepo.GetByID(t.Context(), sourceAccount.Id)
		require.NoError(t, err)
		assert.Equal(t, int64(1000), sourceAfterSecondAttempt.BalanceMinor)

		destinationAfterSecondAttempt, err := accountRepo.GetByID(t.Context(), destinationAccount.Id)
		require.NoError(t, err)
		assert.Equal(t, int64(500), destinationAfterSecondAttempt.BalanceMinor)
	})

	t.Run("fails with ErrInvalidAmount for zero or negative amount", func(t *testing.T) {
		t.Cleanup(func() {
			truncateTransferEntries(t, testPool)
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userSourceId := insertTestUser(t, testPool)
		userDestinationId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(NewAccountRepository(testPool), testPool)

		sourceAccount, err := accountRepo.Create(t.Context(), userSourceId, PLN)
		require.NoError(t, err)
		destinationAccount, err := accountRepo.Create(t.Context(), userDestinationId, PLN)
		require.NoError(t, err)

		transferReq := TransferRequest{
			SourceAccountID:      sourceAccount.Id,
			DestinationAccountID: destinationAccount.Id,
			RequestingUserID:     userSourceId,
			AmountMinor:          0,
		}

		transfer, err := service.Transfer(t.Context(), transferReq, "zero-amount-key")

		assert.ErrorIs(t, err, ErrInvalidAmount)
		assert.Nil(t, transfer)
	})
}

func TestServiceDepositIntegration(t *testing.T) {
	t.Run("increases balance and creates a credit ledger entry", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(accountRepo, testPool)

		account, err := accountRepo.Create(t.Context(), userId, PLN)
		require.NoError(t, err)

		depositReq := DepositRequest{
			AccountID:        account.Id,
			RequestingUserID: userId,
			AmountMinor:      700,
		}

		err = service.Deposit(t.Context(), depositReq)
		require.NoError(t, err)

		updatedAccount, err := accountRepo.GetByID(t.Context(), account.Id)
		require.NoError(t, err)
		assert.Equal(t, int64(700), updatedAccount.BalanceMinor)

		var operationType, entryType string
		var amountMinor int64
		var transferID *int64
		err = testPool.QueryRow(t.Context(),
			"SELECT operation_type, entry_type, amount_minor, transfer_id FROM ledger_entries WHERE account_id = $1",
			account.Id,
		).Scan(&operationType, &entryType, &amountMinor, &transferID)
		require.NoError(t, err)

		assert.Equal(t, string(OperationDeposit), operationType)
		assert.Equal(t, string(EntryCredit), entryType)
		assert.Equal(t, int64(700), amountMinor)
		assert.Nil(t, transferID)
	})

	t.Run("fails with ErrForbidden when requesting user does not own account", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(accountRepo, testPool)

		account, err := accountRepo.Create(t.Context(), userId, PLN)
		require.NoError(t, err)

		depositReq := DepositRequest{
			AccountID:        account.Id,
			RequestingUserID: 999999,
			AmountMinor:      700,
		}

		err = service.Deposit(t.Context(), depositReq)

		assert.ErrorIs(t, err, ErrForbidden)

		unchangedAccount, err := accountRepo.GetByID(t.Context(), account.Id)
		require.NoError(t, err)
		assert.Equal(t, int64(0), unchangedAccount.BalanceMinor)
	})

	t.Run("fails with ErrInvalidAmount for zero amount", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(accountRepo, testPool)

		account, err := accountRepo.Create(t.Context(), userId, PLN)
		require.NoError(t, err)

		depositReq := DepositRequest{
			AccountID:        account.Id,
			RequestingUserID: userId,
			AmountMinor:      0,
		}

		err = service.Deposit(t.Context(), depositReq)

		assert.ErrorIs(t, err, ErrInvalidAmount)
	})

	t.Run("fails with ErrNoRows for nonexistent account", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(accountRepo, testPool)

		depositReq := DepositRequest{
			AccountID:        9999999,
			RequestingUserID: userId,
			AmountMinor:      700,
		}

		err := service.Deposit(t.Context(), depositReq)

		assert.ErrorIs(t, err, ErrNoRows)
	})
}

func TestServiceWithdrawIntegration(t *testing.T) {
	t.Run("decreases balance and creates a debit ledger entry", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(accountRepo, testPool)

		account, err := accountRepo.Create(t.Context(), userId, PLN)
		require.NoError(t, err)

		err = accountRepo.AdjustBalance(t.Context(), account.Id, int64(1000))
		require.NoError(t, err)

		withdrawReq := WithdrawRequest{
			AccountID:        account.Id,
			RequestingUserID: userId,
			AmountMinor:      300,
		}

		err = service.Withdraw(t.Context(), withdrawReq)
		require.NoError(t, err)

		updatedAccount, err := accountRepo.GetByID(t.Context(), account.Id)
		require.NoError(t, err)
		assert.Equal(t, int64(700), updatedAccount.BalanceMinor)

		var operationType, entryType string
		var amountMinor int64
		err = testPool.QueryRow(t.Context(),
			"SELECT operation_type, entry_type, amount_minor FROM ledger_entries WHERE account_id = $1",
			account.Id,
		).Scan(&operationType, &entryType, &amountMinor)
		require.NoError(t, err)

		assert.Equal(t, string(OperationWithdrawal), operationType)
		assert.Equal(t, string(EntryDebit), entryType)
		assert.Equal(t, int64(300), amountMinor)
	})

	t.Run("fails with ErrInsufficientFunds and does not change balance", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(accountRepo, testPool)

		account, err := accountRepo.Create(t.Context(), userId, PLN)
		require.NoError(t, err)

		err = accountRepo.AdjustBalance(t.Context(), account.Id, int64(100))
		require.NoError(t, err)

		withdrawReq := WithdrawRequest{
			AccountID:        account.Id,
			RequestingUserID: userId,
			AmountMinor:      500,
		}

		err = service.Withdraw(t.Context(), withdrawReq)

		assert.ErrorIs(t, err, ErrInsufficientFunds)

		unchangedAccount, err := accountRepo.GetByID(t.Context(), account.Id)
		require.NoError(t, err)
		assert.Equal(t, int64(100), unchangedAccount.BalanceMinor)

		var count int
		err = testPool.QueryRow(t.Context(),
			"SELECT COUNT(*) FROM ledger_entries WHERE account_id = $1",
			account.Id,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count) // rollback
	})

	t.Run("fails with ErrForbidden when requesting user does not own account", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(accountRepo, testPool)

		account, err := accountRepo.Create(t.Context(), userId, PLN)
		require.NoError(t, err)

		err = accountRepo.AdjustBalance(t.Context(), account.Id, int64(1000))
		require.NoError(t, err)

		withdrawReq := WithdrawRequest{
			AccountID:        account.Id,
			RequestingUserID: 999999,
			AmountMinor:      300,
		}

		err = service.Withdraw(t.Context(), withdrawReq)

		assert.ErrorIs(t, err, ErrForbidden)

		unchangedAccount, err := accountRepo.GetByID(t.Context(), account.Id)
		require.NoError(t, err)
		assert.Equal(t, int64(1000), unchangedAccount.BalanceMinor)
	})

	t.Run("fails with ErrInvalidAmount for negative amount", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(accountRepo, testPool)

		account, err := accountRepo.Create(t.Context(), userId, PLN)
		require.NoError(t, err)

		withdrawReq := WithdrawRequest{
			AccountID:        account.Id,
			RequestingUserID: userId,
			AmountMinor:      -50,
		}

		err = service.Withdraw(t.Context(), withdrawReq)

		assert.ErrorIs(t, err, ErrInvalidAmount)
	})

	t.Run("fails with ErrNoRows for nonexistent account", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(accountRepo, testPool)

		withdrawReq := WithdrawRequest{
			AccountID:        9999999,
			RequestingUserID: userId,
			AmountMinor:      300,
		}

		err := service.Withdraw(t.Context(), withdrawReq)

		assert.ErrorIs(t, err, ErrNoRows)
	})
}

func TestServiceTransferNoDeadlockOnConcurrentOppositeTransfers(t *testing.T) {
	t.Cleanup(func() {
		truncateTransferEntries(t, testPool)
		truncateLedgerEntries(t, testPool)
		truncateAccounts(t, testPool)
		truncateUsers(t, testPool)
	})

	userA := insertTestUser(t, testPool)
	userB := insertTestUser(t, testPool)
	accountRepo := NewAccountRepository(testPool)
	service := NewTransactionalService(accountRepo, testPool)

	accountA, err := accountRepo.Create(t.Context(), userA, PLN)
	require.NoError(t, err)
	accountB, err := accountRepo.Create(t.Context(), userB, PLN)
	require.NoError(t, err)

	require.NoError(t, accountRepo.AdjustBalance(t.Context(), accountA.Id, 1000))
	require.NoError(t, accountRepo.AdjustBalance(t.Context(), accountB.Id, 1000))

	const iterations = 20
	errCh := make(chan error, iterations*2)
	var wg sync.WaitGroup

	for i := 0; i < iterations; i++ {
		wg.Add(2)

		go func(key string) {
			defer wg.Done()
			_, err := service.Transfer(t.Context(), TransferRequest{
				SourceAccountID: accountA.Id, DestinationAccountID: accountB.Id,
				RequestingUserID: userA, AmountMinor: 10,
			}, key)
			errCh <- err
		}(fmt.Sprintf("a-to-b-%d", i))

		go func(key string) {
			defer wg.Done()
			_, err := service.Transfer(t.Context(), TransferRequest{
				SourceAccountID: accountB.Id, DestinationAccountID: accountA.Id,
				RequestingUserID: userB, AmountMinor: 10,
			}, key)
			errCh <- err
		}(fmt.Sprintf("b-to-a-%d", i))
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			assert.NotContains(t, err.Error(), "deadlock detected")
		}
	}

	finalA, err := accountRepo.GetByID(t.Context(), accountA.Id)
	require.NoError(t, err)
	finalB, err := accountRepo.GetByID(t.Context(), accountB.Id)
	require.NoError(t, err)

	assert.Equal(t, int64(1000), finalA.BalanceMinor)
	assert.Equal(t, int64(1000), finalB.BalanceMinor)
}

func TestServiceGetHistoryIntegration(t *testing.T) {
	t.Run("returns paginated history for account owner", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(accountRepo, testPool)

		account, err := accountRepo.Create(t.Context(), userId, PLN)
		require.NoError(t, err)

		require.NoError(t, service.Deposit(t.Context(), DepositRequest{
			AccountID: account.Id, RequestingUserID: userId, AmountMinor: 500,
		}))
		require.NoError(t, service.Withdraw(t.Context(), WithdrawRequest{
			AccountID: account.Id, RequestingUserID: userId, AmountMinor: 200,
		}))

		entries, err := service.GetHistory(t.Context(), account.Id, userId, 20, 0)

		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Equal(t, EntryDebit, entries[0].EntryType)
		assert.Equal(t, int64(200), entries[0].AmountMinor)
		assert.Equal(t, EntryCredit, entries[1].EntryType)
		assert.Equal(t, int64(500), entries[1].AmountMinor)
	})

	t.Run("fails with ErrForbidden for non-owner", func(t *testing.T) {
		t.Cleanup(func() {
			truncateLedgerEntries(t, testPool)
			truncateAccounts(t, testPool)
			truncateUsers(t, testPool)
		})

		userId := insertTestUser(t, testPool)
		accountRepo := NewAccountRepository(testPool)
		service := NewTransactionalService(accountRepo, testPool)

		account, err := accountRepo.Create(t.Context(), userId, PLN)
		require.NoError(t, err)

		_, err = service.GetHistory(t.Context(), account.Id, 999999, 20, 0)

		assert.ErrorIs(t, err, ErrForbidden)
	})
}
