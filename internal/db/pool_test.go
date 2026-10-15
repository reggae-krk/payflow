package db

import (
	"testing"

	"github.com/reggae-krk/payflow/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPoolConfigUsesPoolSize(t *testing.T) {
	cfg := config.DBConfig{
		DBHost: "localhost", DBPort: "5432", DBUser: "u", DBPassword: "p", DBName: "d",
		MaxConns: 7, MinConns: 2,
	}

	got, err := poolConfig(cfg)

	require.NoError(t, err)
	assert.Equal(t, int32(7), got.MaxConns)
	assert.Equal(t, int32(2), got.MinConns)
	assert.Equal(t, "localhost", got.ConnConfig.Host)
}

func TestPoolConfigRejectsBadConnString(t *testing.T) {
	_, err := poolConfig(config.DBConfig{DBHost: "localhost", DBPort: "not-a-port", MaxConns: 1})

	assert.ErrorContains(t, err, "parse db config")
}
