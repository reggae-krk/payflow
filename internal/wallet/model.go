package wallet

import "time"

type Account struct {
	Id           int64
	UserId       int64
	Currency     Currency
	BalanceMinor int64
	CreatedAt    time.Time
}
