package wallet

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrForbidden = errors.New("account does not belong to requesting user")

type AccountService interface {
	CreateAccount(ctx context.Context, userId int64, currency string) (*Account, error)
	GetBalance(ctx context.Context, accountId, requestingUserId int64) (int64, error)
	Deposit(ctx context.Context, req DepositRequest) error
	Withdraw(ctx context.Context, req WithdrawRequest) error
}

type service struct {
	repo AccountRepository
	pool *pgxpool.Pool
}

func NewService(repo AccountRepository) *service {
	return &service{repo: repo}
}

func NewTransactionalService(repo AccountRepository, pool *pgxpool.Pool) *service {
	return &service{repo: repo,
		pool: pool,
	}
}

func (s *service) CreateAccount(ctx context.Context, userId int64, curr string) (*Account, error) {
	currency, err := ParseCurrency(curr)

	if err != nil {
		return nil, err
	}

	account, err := s.repo.Create(ctx, userId, currency)

	if err != nil {
		return nil, err
	}
	return account, nil
}

func (s *service) GetBalance(ctx context.Context, accountId, requestingUserId int64) (int64, error) {
	account, err := s.repo.GetByID(ctx, accountId)

	if err != nil {
		return 0, err
	}

	if requestingUserId != account.UserId {
		return 0, ErrForbidden
	}

	return account.BalanceMinor, nil
}

func (s *service) Deposit(ctx context.Context, req DepositRequest) error {
	if s.pool == nil {
		return errors.New("wallet: Deposit requires service built with NewTransactionalService")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	txAccountRepo := NewAccountRepository(tx)
	txLedgerRepo := NewLedgerRepository(tx)

	account, err := txAccountRepo.GetByID(ctx, req.AccountID)

	if err != nil {
		return err
	}

	if req.RequestingUserID != account.UserId {
		return ErrForbidden
	}

	ledgerEntry := LedgerEntry{
		TransferID:    nil, // deposit is not a transfer
		AccountID:     req.AccountID,
		OperationType: OperationDeposit,
		EntryType:     EntryCredit,
		AmountMinor:   req.AmountMinor,
	}

	err = txAccountRepo.AdjustBalance(ctx, req.AccountID, req.AmountMinor)

	if err != nil {
		return err
	}

	err = txLedgerRepo.Insert(ctx, ledgerEntry)

	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *service) Withdraw(ctx context.Context, req WithdrawRequest) error {
	if s.pool == nil {
		return errors.New("wallet: withdraw requires service built with NewTransactionalService")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	txAccountRepo := NewAccountRepository(tx)
	txLedgerRepo := NewLedgerRepository(tx)

	account, err := txAccountRepo.GetByID(ctx, req.AccountID)

	if err != nil {
		return err
	}

	if req.RequestingUserID != account.UserId {
		return ErrForbidden
	}

	ledgerEntry := LedgerEntry{
		TransferID:    nil, // withdraw is not a transfer
		AccountID:     req.AccountID,
		OperationType: OperationWithdrawal,
		EntryType:     EntryDebit,
		AmountMinor:   req.AmountMinor,
	}

	err = txAccountRepo.AdjustBalance(ctx, req.AccountID, -req.AmountMinor)

	if err != nil {
		return err
	}

	err = txLedgerRepo.Insert(ctx, ledgerEntry)

	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}