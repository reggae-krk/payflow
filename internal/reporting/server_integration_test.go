package reporting

import (
	"context"
	"log"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reggae-krk/payflow/internal/testhelpers"
	"github.com/reggae-krk/payflow/proto/reporting/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := testhelpers.CreatePostgresContainer(ctx)
	if err != nil {
		log.Fatalf("failed to start postgres container: %v", err)
	}
	defer container.Terminate(ctx)

	pool, err := pgxpool.New(ctx, container.ConnectionString)
	if err != nil {
		log.Fatalf("failed to connect to test database: %v", err)
	}
	defer pool.Close()

	testPool = pool
	m.Run()
}

func startBufconnServer(t *testing.T) pb.ReportingServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)

	grpcServer := grpc.NewServer()
	pb.RegisterReportingServiceServer(grpcServer, NewServer(testPool))
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("bufconn server exited: %v", err)
		}
	}()
	t.Cleanup(grpcServer.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial bufconn: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return pb.NewReportingServiceClient(conn)
}

func TestGetAccountSummary_Integration_ReturnsAggregatedData(t *testing.T) {
	ctx := context.Background()

	var userID int64
	err := testPool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		"reporting-test@example.com", "irrelevant-hash",
	).Scan(&userID)

	if err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	var accountID int64
	err = testPool.QueryRow(ctx,
		`INSERT INTO accounts (user_id, currency, balance_minor) VALUES ($1, 'PLN', 5000) RETURNING id`,
		userID,
	).Scan(&accountID)

	if err != nil {
		t.Fatalf("failed to seed account: %v", err)
	}

	_, err = testPool.Exec(ctx,
		`INSERT INTO ledger_entries (account_id, amount_minor, operation_type, entry_type) VALUES
		($1, 3000, 'deposit', 'credit'),
		($1, 1000, 'withdrawal', 'debit')`,
		accountID,
	)

	if err != nil {
		t.Fatalf("failed to seed ledger entries: %v", err)
	}

	client := startBufconnServer(t)
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := client.GetAccountSummary(ctx, &pb.GetAccountSummaryRequest{AccountId: accountID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.CurrentBalanceMinor != 5000 {
		t.Errorf("expected balance 5000, got %d", resp.CurrentBalanceMinor)
	}
	if resp.TotalDepositsMinor != 3000 {
		t.Errorf("expected deposits 3000, got %d", resp.TotalDepositsMinor)
	}
	if resp.TotalWithdrawalsMinor != 1000 {
		t.Errorf("expected withdrawals 1000, got %d", resp.TotalWithdrawalsMinor)
	}
}

func TestGetAccountSummary_Integration_ReturnsNotFound(t *testing.T) {
	client := startBufconnServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := client.GetAccountSummary(ctx, &pb.GetAccountSummaryRequest{AccountId: 999999})

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.NotFound {
		t.Errorf("expected codes.NotFound, got: %v", err)
	}
}
