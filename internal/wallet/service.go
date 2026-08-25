package wallet

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reggae-krk/payflow/internal/db"
)

var ErrForbidden = errors.New("account does not belong to requesting user")
var ErrInvalidTransfer = errors.New("cannot transfer to the same account")
var ErrInvalidAmount = errors.New("amount must be positive")

type AccountService interface {
	CreateAccount(ctx context.Context, userId int64, currency string) (*Account, error)
	GetBalance(ctx context.Context, accountId, requestingUserId int64) (int64, error)
	Deposit(ctx context.Context, req DepositRequest) error
	Withdraw(ctx context.Context, req WithdrawRequest) error
	Transfer(ctx context.Context, req TransferRequest, idempotencyKey string) (*Transfer, error)
	GetHistory(ctx context.Context, accountId, requestingUserId int64, limit, offset int) ([]*LedgerEntry, error)
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
	if req.AmountMinor <= 0 {
		return ErrInvalidAmount
	}
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
	if req.AmountMinor <= 0 {
		return ErrInvalidAmount
	}
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

func (s *service) Transfer(ctx context.Context, req TransferRequest, idempotencyKey string) (*Transfer, error) {
	if req.AmountMinor <= 0 {
		return nil, ErrInvalidAmount
	}
	if req.SourceAccountID == req.DestinationAccountID {
		return nil, ErrInvalidTransfer
	}
	if s.pool == nil {
		return nil, errors.New("wallet: transfer requires service built with NewTransactionalService")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	txAccountRepo := NewAccountRepository(tx)
	txLedgerRepo := NewLedgerRepository(tx)
	txTransferRepo := NewTransferRepository(tx)

	firstID, secondID := req.SourceAccountID, req.DestinationAccountID
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}

	firstAccount, err := txAccountRepo.GetByIDForUpdate(ctx, firstID)
	if err != nil {
		if errors.Is(err, ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}

	secondAccount, err := txAccountRepo.GetByIDForUpdate(ctx, secondID)
	if err != nil {
		if errors.Is(err, ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}

	var sourceAccount *Account
	if firstAccount.Id == req.SourceAccountID {
		sourceAccount = firstAccount
	} else {
		sourceAccount = secondAccount
	}

	if req.RequestingUserID != sourceAccount.UserId {
		return nil, ErrForbidden
	}

	if err := txAccountRepo.AdjustBalance(ctx, req.SourceAccountID, -req.AmountMinor); err != nil {
		return nil, err
	}

	if err := txAccountRepo.AdjustBalance(ctx, req.DestinationAccountID, req.AmountMinor); err != nil {
		return nil, err
	}

	//create transfer
	transfer, err := txTransferRepo.Create(ctx, req.SourceAccountID, req.DestinationAccountID, req.AmountMinor, idempotencyKey)

	if err != nil {
		if db.IsUniqueViolation(err) {
			// TODO(PF-14.1): this is not true idempotency yet. Instead of returning
			// ErrDuplicateTransfer (409), a real idempotency implementation should look up
			// the existing transfer by idempotency_key and return it as if this were the
			// first successful call — same approach Stripe/Adyen use. Right now the client
			// has no way to distinguish "my transfer succeeded" from "something went wrong
			// on retry".
			return nil, ErrDuplicateTransfer
		}
		return nil, err
	}

	//create source ledger entry
	ledgerEntrySource := LedgerEntry{
		TransferID:    &transfer.Id,
		AccountID:     req.SourceAccountID,
		OperationType: OperationTransfer,
		EntryType:     EntryDebit,
		AmountMinor:   req.AmountMinor,
	}

	err = txLedgerRepo.Insert(ctx, ledgerEntrySource)

	if err != nil {
		return nil, err
	}

	//create destination ledger entry
	ledgerEntryDestination := LedgerEntry{
		TransferID:    &transfer.Id,
		AccountID:     req.DestinationAccountID,
		OperationType: OperationTransfer,
		EntryType:     EntryCredit,
		AmountMinor:   req.AmountMinor,
	}

	err = txLedgerRepo.Insert(ctx, ledgerEntryDestination)

	if err != nil {
		return nil, err
	}

	//TODO: track failed transactions
	if err := txTransferRepo.MarkCompleted(ctx, transfer.Id); err != nil {
		return nil, err
	}

	transfer.Status = "completed"

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return transfer, nil
}

func (s *service) GetHistory(ctx context.Context, accountId, requestingUserId int64, limit, offset int) ([]*LedgerEntry, error) {
	account, err := s.repo.GetByID(ctx, accountId)
	if err != nil {
		return nil, err
	}
	if requestingUserId != account.UserId {
		return nil, ErrForbidden
	}

	ledgerRepo := NewLedgerRepository(s.pool)
	return ledgerRepo.GetByAccountID(ctx, accountId, limit, offset)
}
