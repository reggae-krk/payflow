package reporting

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/reggae-krk/payflow/proto/reporting/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeRow struct {
	scan func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error {
	return r.scan(dest...)
}

type fakeQuerier struct {
	ledgerRow   fakeRow
	accountsRow fakeRow
}

func (f *fakeQuerier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *fakeQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (f *fakeQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "ledger_entries") {
		return f.ledgerRow
	}
	return f.accountsRow
}

func TestGetAccountSummary_ReturnsAggregatedData(t *testing.T) {
	fq := &fakeQuerier{
		ledgerRow: fakeRow{scan: func(dest ...any) error {
			*dest[0].(*int64) = 500  // deposits
			*dest[1].(*int64) = 100  // withdrawals
			*dest[2].(*int64) = 200  // transfersIn
			*dest[3].(*int64) = 50   // transfersOut
			return nil
		}},
		accountsRow: fakeRow{scan: func(dest ...any) error {
			*dest[0].(*int64) = 550 // balance
			return nil
		}},
	}
	s := NewServer(fq)

	resp, err := s.GetAccountSummary(context.Background(), &pb.GetAccountSummaryRequest{AccountId: 1})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CurrentBalanceMinor != 550 {
		t.Errorf("expected balance 550, got %d", resp.CurrentBalanceMinor)
	}
	if resp.TotalDepositsMinor != 500 {
		t.Errorf("expected deposits 500, got %d", resp.TotalDepositsMinor)
	}
}

func TestGetAccountSummary_ReturnsNotFoundForMissingAccount(t *testing.T) {
	fq := &fakeQuerier{
		ledgerRow: fakeRow{scan: func(dest ...any) error {
			*dest[0].(*int64) = 0
			*dest[1].(*int64) = 0
			*dest[2].(*int64) = 0
			*dest[3].(*int64) = 0
			return nil
		}},
		accountsRow: fakeRow{scan: func(dest ...any) error {
			return pgx.ErrNoRows
		}},
	}
	s := NewServer(fq)

	_, err := s.GetAccountSummary(context.Background(), &pb.GetAccountSummaryRequest{AccountId: 999})

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.NotFound {
		t.Errorf("expected codes.NotFound, got: %v", err)
	}
}