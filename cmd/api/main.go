package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reggae-krk/payflow/internal/app"
	"github.com/reggae-krk/payflow/internal/config"
	"github.com/reggae-krk/payflow/internal/db"
	"github.com/reggae-krk/payflow/internal/users"
	"github.com/reggae-krk/payflow/internal/wallet"
)

// shutdownTimeout must cover the slowest request (request deadline + FinishTx rollback)
// and stay below the orchestrator's grace period (docker-compose stop_grace_period, k8s 30 s).
const shutdownTimeout = 25 * time.Second
const writeTimeout = 15 * time.Second

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
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := checkTimeouts(cfg.TimeoutConfig, writeTimeout, shutdownTimeout); err != nil {
		return fmt.Errorf("wrong timeouts: %w", err)
	}

	pool, err := db.NewPool(ctx, cfg.DBConfig)
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

	srv := &http.Server{
		Handler:           router(pool, cfg.TimeoutConfig),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       30 * time.Second,
	}

	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	log.Println("PayFlow API listening on :8080")
	return serve(ctx, srv, lis, shutdownTimeout)
}

// serve runs srv on lis until ctx is cancelled, then shuts it down gracefully:
// no new connections, in-flight requests get up to timeout to finish.
func serve(ctx context.Context, srv *http.Server, lis net.Listener, timeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(lis)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Println("shutting down: waiting for in-flight requests")
	shCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	if err := srv.Shutdown(shCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

func router(pool *pgxpool.Pool, cfg config.TimeoutConfig) *gin.Engine {
	usersRepo := users.NewUserRepository(pool)
	usersService := users.NewService(usersRepo)
	usersHandler := users.NewHandler(usersService)

	accountRepo := wallet.NewAccountRepository(pool)
	accountService := wallet.NewTransactionalService(accountRepo, pool)
	accountHandler := wallet.NewHandler(accountService)

	registrationService := app.NewService(pool)
	registrationHandler := app.NewHandler(registrationService)

	router := SetupRouter(cfg, usersHandler, accountHandler, registrationHandler, func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return router
}

func checkTimeouts(cfg config.TimeoutConfig, writeTimeout, shutdownTimeout time.Duration) error {
	checks := []struct {
		name string
		d    time.Duration
	}{
		{"HEALTH_TIMEOUT", cfg.Health},
		{"LOGREG_TIMEOUT", cfg.LogReg},
		{"REQUEST_TIMEOUT", cfg.Request},
	}

	for _, c := range checks {
		if c.d+db.RollbackTimeout >= writeTimeout {
			return fmt.Errorf("%s=%s + rollback %s must be < write timeout %s", c.name, c.d, db.RollbackTimeout, writeTimeout)
		}
		if c.d+db.RollbackTimeout >= shutdownTimeout {
			return fmt.Errorf("%s=%s + rollback %s must be < shutdown timeout %s", c.name, c.d, db.RollbackTimeout, shutdownTimeout)
		}
	}

	return nil
}
