package config

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"
)

type DBConfig struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	MaxConns   int32
	MinConns   int32
}

type TimeoutConfig struct {
	Health  time.Duration
	LogReg  time.Duration
	Request time.Duration
}

type Config struct {
	DBConfig
	TimeoutConfig
	JWTSecret string
}

// LoadDBConfig reads the connection settings and the pool size.
// DB_MAX_CONNS and DB_MIN_CONNS are optional; the fallbacks match the pgxpool defaults.
func LoadDBConfig() (DBConfig, error) {
	maxConns, errMax := getEnvInt32("DB_MAX_CONNS", int32(max(4, runtime.NumCPU())))
	minConns, errMin := getEnvInt32("DB_MIN_CONNS", 0)

	if errMax != nil || errMin != nil {
		return DBConfig{}, errors.Join(errMax, errMin)
	}
	if maxConns < 1 {
		return DBConfig{}, fmt.Errorf("DB_MAX_CONNS=%d must be greater than 0", maxConns)
	}
	if minConns > maxConns {
		return DBConfig{}, fmt.Errorf("DB_MIN_CONNS=%d must be <= DB_MAX_CONNS=%d", minConns, maxConns)
	}

	return DBConfig{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "payflow"),
		DBPassword: getEnv("DB_PASSWORD", "payflow_dev_password"),
		DBName:     getEnv("DB_NAME", "payflow"),
		MaxConns:   maxConns,
		MinConns:   minConns,
	}, nil
}

func LoadTimeoutConfig() (TimeoutConfig, error) {
	health, errHealth := getEnvTime("HEALTH_TIMEOUT")
	logreg, errLogReg := getEnvTime("LOGREG_TIMEOUT")
	request, errRequest := getEnvTime("REQUEST_TIMEOUT")

	if errHealth != nil || errLogReg != nil || errRequest != nil {
		return TimeoutConfig{}, errors.Join(errHealth, errLogReg, errRequest)
	}

	return TimeoutConfig{
		Health:  health,
		LogReg:  logreg,
		Request: request,
	}, nil
}

func LoadJWTSecret() (string, error) {
	jwtSecret := getEnv("JWT_SECRET", "")
	if jwtSecret == "" {
		return "", fmt.Errorf("JWT_SECRET is required")
	}
	return jwtSecret, nil
}

func Load() (Config, error) {
	dbCfg, errDB := LoadDBConfig()
	jwtSecret, errJWT := LoadJWTSecret()
	timeoutCfg, errTimeout := LoadTimeoutConfig()

	// Report every config problem at once, not one per restart.
	if err := errors.Join(errDB, errJWT, errTimeout); err != nil {
		return Config{}, err
	}

	return Config{
		DBConfig:      dbCfg,
		TimeoutConfig: timeoutCfg,
		JWTSecret:     jwtSecret,
	}, nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getEnvTime(key string) (time.Duration, error) {
	if val, ok := os.LookupEnv(key); ok {
		duration, err := time.ParseDuration(val)

		if err != nil {
			return 0, fmt.Errorf("%s has wrong format in env vars: %w", key, err)
		}
		if duration <= 0 {
			return 0, fmt.Errorf("%s must be greater than 0", key)
		}
		return duration, nil
	}
	return 0, fmt.Errorf("%s not found in env vars", key)
}

func getEnvInt32(key string, fallback int32) (int32, error) {
	if val, ok := os.LookupEnv(key); ok {
		i, err := strconv.ParseInt(val, 10, 32)

		if err != nil {
			return 0, fmt.Errorf("%s has wrong format in env vars: %w", key, err)
		}
		if i < 0 {
			return 0, fmt.Errorf("%s must not be negative", key)
		}
		return int32(i), nil
	}
	return fallback, nil
}

func (c DBConfig) ConnString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}
