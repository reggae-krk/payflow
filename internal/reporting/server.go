package reporting

import (
	"context"

	"github.com/reggae-krk/payflow/proto/reporting/pb"
)

type server struct {
	pb.UnimplementedReportingServiceServer
}

func NewServer() *server {
	return &server{}
}

func (s *server) GetAccountSummary(ctx context.Context, req *pb.GetAccountSummaryRequest) (*pb.GetAccountSummaryResponse, error) {
	return &pb.GetAccountSummaryResponse{
		AccountId:             req.AccountId,
		CurrentBalanceMinor:   0,
		TotalDepositsMinor:    0,
		TotalWithdrawalsMinor: 0,
	}, nil
}
