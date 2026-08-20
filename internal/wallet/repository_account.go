package wallet

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/reggae-krk/payflow/internal/db"
)

var ErrNoRows = errors.New("no rows in result set")
var ErrNoRowToUpdate = errors.New("no row found to update")
var ErrInsufficientFunds = errors.New("insufficient funds")
var ErrUserNotFound = errors.New("user not found")

type AccountRepository interface {
	Create(ctx context.Context, userId int64, currency Currency) (*Account, error)
	GetByID(ctx context.Context, id int64) (*Account, error)
	GetByUserID(ctx context.Context, userId int64) ([]*Account, error)
	AdjustBalance(ctx context.Context, id int64, deltaMinor int64) error
}

type accountRepository struct {
	db db.Querier
}

func NewAccountRepository(database db.Querier) *accountRepository {
	return &accountRepository{db: database}
}

func (accountRepo *accountRepository) Create(ctx context.Context, userId int64, currency Currency) (*Account, error) {
	var account Account

	query := `
		INSERT INTO accounts (user_id, currency, balance_minor)
		VALUES ($1, $2, 0)
		RETURNING id, user_id, currency, balance_minor, created_at
	`

	err := accountRepo.db.QueryRow(ctx, query, userId, currency).Scan(
		&account.Id, &account.UserId, &account.Currency, &account.BalanceMinor, &account.CreatedAt,
	)

	if err != nil {
		if db.IsForeignKeyViolation(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &account, nil
}

func (accountRepo *accountRepository) GetByID(ctx context.Context, id int64) (*Account, error) {
	var account Account

	query := `
		SELECT id, user_id, currency, balance_minor, created_at
		FROM accounts
		WHERE id = $1
	`

	err := accountRepo.db.QueryRow(ctx, query, id).Scan(
		&account.Id, &account.UserId, &account.Currency, &account.BalanceMinor, &account.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoRows
		}
		return nil, err
	}
	return &account, nil
}

func (accountRepo *accountRepository) GetByUserID(ctx context.Context, userId int64) ([]*Account, error) {
	query := `
		SELECT id, user_id, currency, balance_minor, created_at
		FROM accounts
		WHERE user_id = $1
	`

	rows, err := accountRepo.db.Query(ctx, query, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*Account

	for rows.Next() {
		var account Account
		if err := rows.Scan(
			&account.Id, &account.UserId, &account.Currency, &account.BalanceMinor, &account.CreatedAt,
		); err != nil {
			return nil, err
		}
		accounts = append(accounts, &account)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return accounts, nil
}

func (accountRepo *accountRepository) AdjustBalance(ctx context.Context, id int64, deltaMinor int64) error {
	query := `
		UPDATE accounts SET balance_minor = balance_minor + $1
		WHERE id = $2 AND balance_minor + $1 >= 0
	`

	cmdTag, err := accountRepo.db.Exec(ctx, query, deltaMinor, id)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		exists, err := accountRepo.exists(ctx, id)
		if err != nil {
			return err
		}
		if !exists {
			return ErrNoRowToUpdate
		}
		return ErrInsufficientFunds
	}

	return nil
}

func (accountRepo *accountRepository) exists(ctx context.Context, id int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1)`

	var exists bool
	err := accountRepo.db.QueryRow(ctx, query, id).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
