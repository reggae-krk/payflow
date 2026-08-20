package wallet

import "time"

type Account struct {
	Id           int64
	UserId       int64
	Currency     Currency
	BalanceMinor int64
	CreatedAt    time.Time
}

type LedgerEntry struct {
	TransferID    *int64 // nil for deposit/withdrawal
	AccountID     int64
	OperationType OperationType
	EntryType     EntryType
	AmountMinor   int64
}
