package config

import "testing"

func TestLoadRedisEnvironment(t *testing.T) {
	t.Setenv("REDIS_URL", "rediss://:url-secret@cache.example:6380/2")
	t.Setenv("REDIS_ADDR", "legacy.example:6379")
	t.Setenv("REDIS_PASSWORD", "legacy-secret")

	cfg := Load()
	if cfg.RedisURL != "rediss://:url-secret@cache.example:6380/2" {
		t.Fatalf("RedisURL = %q", cfg.RedisURL)
	}
	if cfg.RedisAddr != "legacy.example:6379" || cfg.RedisPassword != "legacy-secret" {
		t.Fatalf("legacy Redis settings were not retained: addr=%q password=%q", cfg.RedisAddr, cfg.RedisPassword)
	}
}
