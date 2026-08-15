package config

import (
	"fmt"
	"os"
)

type Config struct {
	DBHost string
	DBPort string
	DBUser string
	DBPassword string
	DBName string
	JWTSecret string
}

func Load() Config {
	jwtSecret := getEnv("JWT_SECRET", "")
    if jwtSecret == "" {
        panic("JWT_SECRET is required")
    }

	return Config{
		DBHost: getEnv("DB_HOST", "localhost"),
		DBPort: getEnv("DB_PORT", "5432"),
		DBUser: getEnv("DB_USER", "payflow"),
		DBPassword: getEnv("DB_PASSWORD", "payflow_dev_password"),
		DBName: getEnv("DB_NAME", "payflow"),
		JWTSecret: getEnv("JWT_SECRET", jwtSecret),
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func (c Config) ConnString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}