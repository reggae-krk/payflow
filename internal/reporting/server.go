package reporting

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reggae-krk/payflow/proto/reporting/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedReportingServiceServer
	pool *pgxpool.Pool
}

func NewServer(pool *pgxpool.Pool) *server {
	return &server{pool: pool}
}

func (s *server) GetAccountSummary(ctx context.Context, req *pb.GetAccountSummaryRequest) (*pb.GetAccountSummaryResponse, error) {
	query := `
		SELECT
		COALESCE(SUM(amount_minor) FILTER (WHERE operation_type = 'deposit' AND entry_type = 'credit'), 0),
		COALESCE(SUM(amount_minor) FILTER (WHERE operation_type = 'withdrawal' AND entry_type ='debit'), 0),
		COALESCE(SUM(amount_minor) FILTER (WHERE operation_type = 'transfer' AND entry_type = 'credit'), 0),
		COALESCE(SUM(amount_minor) FILTER (WHERE operation_type = 'transfer' AND entry_type = 'debit'), 0)
		FROM ledger_entries WHERE account_id = $1
	`

	var deposits, withdrawals, transfersIn, transfersOut int64
	err := s.pool.QueryRow(ctx, query, req.AccountId).Scan(&deposits, &withdrawals, &transfersIn, &transfersOut)

	if err != nil {
		return nil, err
	}

	var balance int64
	err = s.pool.QueryRow(ctx, "SELECT balance_minor FROM accounts WHERE id = $1", req.AccountId).Scan(&balance)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "account %d not found", req.AccountId)
		}
		return nil, err
	}

	return &pb.GetAccountSummaryResponse{
		AccountId: req.AccountId, CurrentBalanceMinor: balance,
		TotalDepositsMinor: deposits, TotalWithdrawalsMinor: withdrawals,
		TotalTransfersInMinor: transfersIn, TotalTransfersOutMinor: transfersOut,
	}, nil
}
