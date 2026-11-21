package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Address, BaseURL, DatabaseURL, RedisURL, RedisAddr, RedisPassword, APIKey string
	RateLimit, ClickWorkers, ClickBuffer, ClickBatch                          int
	RateWindow                                                                time.Duration
	LogLevel                                                                  slog.Level
}

func Load() Config {
	return Config{
		Address: env("APP_ADDR", ":8080"), BaseURL: env("BASE_URL", "http://localhost:8080"),
		DatabaseURL: env("DATABASE_URL", "postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable"),
		RedisURL:    os.Getenv("REDIS_URL"), RedisAddr: env("REDIS_ADDR", "localhost:6379"), RedisPassword: env("REDIS_PASSWORD", ""),
		APIKey: env("API_KEY", "replace-this-development-key"), RateLimit: envInt("RATE_LIMIT", 10),
		RateWindow: envDuration("RATE_WINDOW", time.Minute), ClickWorkers: envInt("CLICK_WORKERS", 2),
		ClickBuffer: envInt("CLICK_BUFFER", 2048), ClickBatch: envInt("CLICK_BATCH", 100), LogLevel: parseLevel(env("LOG_LEVEL", "info")),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
func envDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
func parseLevel(value string) slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(value)); err != nil {
		return slog.LevelInfo
	}
	return level
}
