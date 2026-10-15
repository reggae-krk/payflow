package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/reggae-krk/payflow/internal/config"
	"github.com/reggae-krk/payflow/internal/db"
	"github.com/reggae-krk/payflow/internal/reporting"
	"github.com/reggae-krk/payflow/proto/reporting/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// shutdownTimeout must stay below the orchestrator's grace period (docker-compose stop_grace_period, k8s 30 s).
const shutdownTimeout = 25 * time.Second

var errGracefulStopTimeout = errors.New("graceful stop timed out, forced stop")

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// After the first signal, restore the default behaviour: a second Ctrl+C kills the process at once.
	context.AfterFunc(ctx, stop)

	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.LoadDBConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create db pool: %w", err)
	}
	defer pool.Close()

	// The pool doesn't connect yet; fail fast if the database is unreachable.
	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	defer cancelPing()
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("unable to connect to database: %w", err)
	}

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterReportingServiceServer(grpcServer, reporting.NewServer(pool))
	reflection.Register(grpcServer)

	log.Println("reporting grpc server listening on :50051")
	return serve(ctx, grpcServer, lis, shutdownTimeout)
}

// serve runs srv on lis until ctx is cancelled, then stops it gracefully:
// no new RPCs, in-flight RPCs get up to timeout to finish, then the rest are cut with Stop.
func serve(ctx context.Context, srv *grpc.Server, lis net.Listener, timeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(lis)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Println("shutting down: waiting for in-flight RPCs")
	stopped := make(chan struct{})
	go func() {
		srv.GracefulStop() // has no timeout of its own: it waits for every RPC
		close(stopped)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-stopped:
		return nil
	case <-timer.C:
		// Stop closes every connection and cancels the RPC contexts, so handlers that
		// respect ctx (pgx queries do) return at once. A handler that ignores its ctx
		// would still block here: the running GracefulStop holds the server lock while
		// it waits for handlers. Then the orchestrator's SIGKILL is the last resort.
		srv.Stop()
		return errGracefulStopTimeout
	}
}
