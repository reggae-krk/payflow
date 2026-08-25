package wallet

import (
	"context"
	"errors"

	"github.com/reggae-krk/payflow/internal/db"
)

var ErrInvalidOperationType = errors.New("invalid operation type")
var ErrAccountNotFound = errors.New("account not found")
var ErrInvalidLedgerEntry = errors.New("invalid ledger entry (check constraint failed)")
var ErrTransferNotFound = errors.New("transfer not found")

type LedgerRepository interface {
	Insert(ctx context.Context, entry LedgerEntry) error
	GetByAccountID(ctx context.Context, accountID int64, limit, offset int) ([]*LedgerEntry, error)
}

type ledgerRepository struct {
	db db.Querier
}

func NewLedgerRepository(database db.Querier) *ledgerRepository {
	return &ledgerRepository{db: database}
}

func (repo *ledgerRepository) Insert(ctx context.Context, entry LedgerEntry) error {
	if !entry.OperationType.IsValid() {
		return ErrInvalidOperationType
	}

	query := `
        INSERT INTO ledger_entries (transfer_id, account_id, operation_type, entry_type, amount_minor)
        VALUES ($1, $2, $3, $4, $5)
    `

	_, err := repo.db.Exec(ctx, query, entry.TransferID, entry.AccountID, entry.OperationType, entry.EntryType, entry.AmountMinor)
	if err != nil {
		if db.IsForeignKeyViolation(err) {
			constraint, _ := db.ConstraintName(err)
			switch constraint {
			case "ledger_entries_transfer_id_fkey":
				return ErrTransferNotFound
			case "ledger_entries_account_id_fkey":
				return ErrAccountNotFound
			default:
				return err
			}
		}
		if db.IsCheckViolation(err) {
			return ErrInvalidLedgerEntry
		}
		return err
	}

	return nil
}

func (repo *ledgerRepository) GetByAccountID(ctx context.Context, accountID int64, limit, offset int) ([]*LedgerEntry, error) {
	query := `
		SELECT transfer_id, account_id, operation_type, entry_type, amount_minor
		FROM ledger_entries
		WHERE account_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := repo.db.Query(ctx, query, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*LedgerEntry
	for rows.Next() {
		var entry LedgerEntry
		if err := rows.Scan(&entry.TransferID, &entry.AccountID, &entry.OperationType, &entry.EntryType, &entry.AmountMinor); err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}
