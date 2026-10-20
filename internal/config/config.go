package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServerPort            string
	DatabaseURL           string
	OrderRateLimitMinutes time.Duration
}

func LoadConfig() *Config {
	return &Config{
		ServerPort:            getEnv("SERVER_PORT", "8080"),
		DatabaseURL:           getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/marketplace_db?sslmode=disable"),
		OrderRateLimitMinutes: getEnvAsDuration("ORDER_RATE_LIMIT_MINUTES", 1) * time.Minute,
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsDuration(key string, defaultVal int) time.Duration {
	if valStr, ok := os.LookupEnv(key); ok {
		if val, err := strconv.Atoi(valStr); err == nil {
			return time.Duration(val)
		}
	}
	return time.Duration(defaultVal)
}
