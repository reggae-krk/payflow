package wallet

import "fmt"

type OperationType string

const (
	OperationDeposit    OperationType = "deposit"
	OperationWithdrawal OperationType = "withdrawal"
	OperationTransfer   OperationType = "transfer"
)

func (o OperationType) IsValid() bool {
	switch o {
	case OperationDeposit:
		return true
	case OperationWithdrawal:
		return true
	case OperationTransfer:
		return true
	default:
		return false
	}
}

func (o OperationType) String() string {
	return string(o)
}

func ParseOperationType(s string) (EntryType, error) {
	o := EntryType(s)
	if !o.IsValid() {
		return "", fmt.Errorf("unsupported currency: %s", s)
	}
	return o, nil
}
