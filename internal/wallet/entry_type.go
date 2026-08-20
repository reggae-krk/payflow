package wallet

import "fmt"

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

func (e EntryType) String() string {
	return string(e)
}

func ParseEntryType(s string) (EntryType, error) {
	e := EntryType(s)
	if !e.IsValid() {
		return "", fmt.Errorf("unsupported currency: %s", s)
	}
	return e, nil
}
