package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type DBConfig struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
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

func LoadDBConfig() DBConfig {
	return DBConfig{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "payflow"),
		DBPassword: getEnv("DB_PASSWORD", "payflow_dev_password"),
		DBName:     getEnv("DB_NAME", "payflow"),
	}
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
	jwtSecret, err := LoadJWTSecret()

	if err != nil {
		return Config{}, err
	}

	timeoutCfg, err := LoadTimeoutConfig()

	if err != nil {
		return Config{}, err
	}

	return Config{
		DBConfig:      LoadDBConfig(),
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

func (c DBConfig) ConnString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}
