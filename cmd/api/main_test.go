package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/reggae-krk/payflow/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startServe runs serve in the background on a random local port.
func startServe(t *testing.T, ctx context.Context, h http.Handler, timeout time.Duration) (string, <-chan error) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, &http.Server{Handler: h, ReadHeaderTimeout: time.Second}, lis, timeout)
	}()
	return "http://" + lis.Addr().String(), done
}

func TestServeFinishesInFlightRequestOnShutdown(t *testing.T) {
	started := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond) // still working when the signal comes
		_, _ = io.WriteString(w, "finished")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	url, done := startServe(t, ctx, h, 5*time.Second)

	type result struct {
		status int
		body   string
		err    error
	}
	resCh := make(chan result, 1)
	go func() {
		resp, err := http.Get(url)
		if err != nil {
			resCh <- result{err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		b, err := io.ReadAll(resp.Body)
		resCh <- result{status: resp.StatusCode, body: string(b), err: err}
	}()

	<-started
	cancel() // simulates SIGTERM while the request is in flight

	res := <-resCh
	require.NoError(t, res.err)
	assert.Equal(t, http.StatusOK, res.status)
	assert.Equal(t, "finished", res.body)

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after shutdown")
	}
}

func TestServeRefusesNewConnectionsAfterShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	url, done := startServe(t, ctx, http.NotFoundHandler(), time.Second)

	cancel()
	require.NoError(t, <-done)

	_, err := http.Get(url)
	assert.Error(t, err)
}

func TestServeReturnsErrorWhenShutdownTimesOut(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release // a request that outlives the shutdown timeout
	})

	ctx, cancel := context.WithCancel(context.Background())
	url, done := startServe(t, ctx, h, 100*time.Millisecond)

	go func() {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started
	cancel()

	select {
	case err := <-done:
		assert.True(t, errors.Is(err, context.DeadlineExceeded), "got %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not give up after the shutdown timeout")
	}
}

func TestServeReturnsServerErrorWithoutSignal(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, lis.Close()) // Serve fails at once, like a listener that died

	err = serve(context.Background(), &http.Server{ReadHeaderTimeout: time.Second}, lis, time.Second)
	assert.Error(t, err)
}

func TestTimeoutsCheck(t *testing.T) {
	tests := []struct {
		name       string
		cfgTimeout config.TimeoutConfig
		want       string
		timeout    time.Duration
	}{
		{"happy path",
			config.TimeoutConfig{
				Health:  2 * time.Second,
				LogReg:  3 * time.Second,
				Request: 5 * time.Second},
			"",
			shutdownTimeout,
		},
		{"time too long",
			config.TimeoutConfig{
				Health:  2 * time.Second,
				LogReg:  15 * time.Second,
				Request: 5 * time.Second},
			"LOGREG_TIMEOUT",
			shutdownTimeout,
		},
		{"time on edge",
			config.TimeoutConfig{
				Health:  2 * time.Second,
				LogReg:  3 * time.Second,
				Request: 10 * time.Second},
			"REQUEST_TIMEOUT",
			shutdownTimeout,
		},
		{"shutdownTimeout too short",
			config.TimeoutConfig{
				Health:  2 * time.Second,
				LogReg:  4 * time.Second,
				Request: 5 * time.Second},
			"< shutdown timeout",
			8 * time.Second,
		},
		{"shutdownTimeout on edge",
			config.TimeoutConfig{
				Health:  2 * time.Second,
				LogReg:  3 * time.Second,
				Request: 3 * time.Second},
			"< shutdown timeout",
			8 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkTimeouts(tt.cfgTimeout, writeTimeout, tt.timeout)
			if tt.want == "" {
				assert.NoError(t, got)
			} else {
				assert.ErrorContains(t, got, tt.want)
			}
		})
	}
}
