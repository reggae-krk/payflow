package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reggae-krk/payflow/internal/config"
)

// NewPool creates a connection pool sized by cfg. Like pgxpool.New, it doesn't connect yet.
func NewPool(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	poolCfg, err := poolConfig(cfg)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, poolCfg)
}

// poolConfig is split out so the pool settings can be tested without a database.
func poolConfig(cfg config.DBConfig) (*pgxpool.Config, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.ConnString())
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	return poolCfg, nil
}
