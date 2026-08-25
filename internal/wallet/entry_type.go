package wallet

type EntryType string

const (
	EntryCredit EntryType = "credit"
	EntryDebit  EntryType = "debit"
)

func (e EntryType) IsValid() bool {
	switch e {
	case EntryCredit:
		return true
	case EntryDebit:
		return true
	default:
		return false
	}
}
