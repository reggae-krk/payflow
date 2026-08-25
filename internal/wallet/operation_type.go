package wallet

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
