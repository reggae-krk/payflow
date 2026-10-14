package main

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// startServe runs serve in the background with the standard health service.
// Every RPC first calls block with its own ctx, so a test can keep an RPC in flight.
func startServe(t *testing.T, ctx context.Context, block func(rpcCtx context.Context), timeout time.Duration) (healthpb.HealthClient, <-chan error) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer(grpc.UnaryInterceptor(
		func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			block(ctx)
			return handler(ctx, req)
		}))
	healthpb.RegisterHealthServer(srv, health.NewServer())

	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, srv, lis, timeout)
	}()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return healthpb.NewHealthClient(conn), done
}

func TestServeFinishesInFlightRPCOnShutdown(t *testing.T) {
	started := make(chan struct{})
	block := func(context.Context) {
		close(started)
		time.Sleep(300 * time.Millisecond) // still working when the signal comes
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, done := startServe(t, ctx, block, 5*time.Second)

	rpcErr := make(chan error, 1)
	go func() {
		resp, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{})
		if err == nil && resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			err = errors.New("unexpected status " + resp.GetStatus().String())
		}
		rpcErr <- err
	}()

	<-started
	cancel() // simulates SIGTERM while the RPC is in flight

	assert.NoError(t, <-rpcErr)
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after graceful stop")
	}
}

func TestServeForcesStopWhenGracefulStopTimesOut(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	block := func(rpcCtx context.Context) {
		close(started)
		select { // an RPC that outlives the shutdown timeout, but respects its ctx like a pgx query
		case <-release:
		case <-rpcCtx.Done():
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	client, done := startServe(t, ctx, block, 100*time.Millisecond)

	rpcErr := make(chan error, 1)
	go func() {
		_, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{})
		rpcErr <- err
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, errGracefulStopTimeout)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not force a stop after the timeout")
	}

	// The cut RPC fails on the client side instead of hanging forever.
	select {
	case err := <-rpcErr:
		assert.Equal(t, codes.Unavailable, status.Code(err), "got %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("client RPC still hanging after forced stop")
	}
}

func TestServeReturnsServerErrorWithoutSignal(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, lis.Close()) // Serve fails at once, like a listener that died

	err = serve(context.Background(), grpc.NewServer(), lis, time.Second)
	assert.Error(t, err)
}
