package wallet

import (
	"context"
	"errors"

	"github.com/reggae-krk/payflow/internal/db"
)

var ErrDuplicateTransfer = errors.New("transfer is duplicated")

type TransferRepository interface {
	Create(ctx context.Context, sourceAccountID, destAccountID int64, amountMinor int64, idempotencyKey string) (*Transfer, error)
	MarkCompleted(ctx context.Context, transferID int64) error
}

type transferRepository struct {
	db db.Querier
}

func NewTransferRepository(database db.Querier) *transferRepository {
	return &transferRepository{db: database}
}

func (r *transferRepository) Create(ctx context.Context, sourceAccountID, destAccountID int64, amountMinor int64, idempotencyKey string) (*Transfer, error) {
	var transfer Transfer
	query := `
		INSERT INTO transfers (source_account_id, destination_account_id, amount_minor, idempotency_key, status)
		VALUES ($1, $2, $3, $4, 'pending')
		RETURNING id, source_account_id, destination_account_id, amount_minor, idempotency_key, status, created_at
	`

	err := r.db.QueryRow(ctx, query, sourceAccountID, destAccountID, amountMinor, idempotencyKey).Scan(
		&transfer.Id, &transfer.SourceAccountID, &transfer.DestinationAccountID,
		&transfer.AmountMinor, &transfer.IdempotencyKey, &transfer.Status, &transfer.CreatedAt,
	)

	if err != nil {
		if db.IsUniqueViolation(err) {
			return nil, ErrDuplicateTransfer
		}
		return nil, err
	}
	return &transfer, nil
}

func (r *transferRepository) MarkCompleted(ctx context.Context, transferID int64) error {
	query := `
		UPDATE transfers SET status = 'completed' WHERE id = $1
	`
	cmdTag, err := r.db.Exec(ctx, query, transferID)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrNoRowToUpdate
	}

	return nil
}
