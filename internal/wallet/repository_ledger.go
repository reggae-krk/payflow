package wallet

import (
	"context"
	"errors"

	"github.com/reggae-krk/payflow/internal/db"
)

var ErrInvalidOperationType = errors.New("invalid operation type")
var ErrAccountNotFound = errors.New("account not found")
var ErrInvalidLedgerEntry = errors.New("invalid ledger entry (check constraint failed)")

type LedgerRepository interface {
	Insert(ctx context.Context, accountID int64, amountMinor int64, entryType string) error
}

type ledgerRepository struct {
	db db.Querier
}

func NewLedgerRepository(database db.Querier) *ledgerRepository {
	return &ledgerRepository{db: database}
}

func (repo *ledgerRepository) Insert(ctx context.Context, entry LedgerEntry) error {
	if entry.OperationType != OperationDeposit &&
		entry.OperationType != OperationWithdrawal &&
		entry.OperationType != OperationTransfer {
		return ErrInvalidOperationType
	}

	query := `
        INSERT INTO ledger_entries (transfer_id, account_id, operation_type, entry_type, amount_minor)
        VALUES ($1, $2, $3, $4, $5)
    `

	_, err := repo.db.Exec(ctx, query, entry.TransferID, entry.AccountID, entry.OperationType, entry.EntryType, entry.AmountMinor)
	if err != nil {
		if db.IsForeignKeyViolation(err) {
			return ErrAccountNotFound
		}
		if db.IsCheckViolation(err) {
			return ErrInvalidLedgerEntry
		}
		return err
	}

	return nil
}
