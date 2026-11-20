package limiter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisIntegration(t *testing.T) {
	address := os.Getenv("INTEGRATION_REDIS_ADDR")
	if address == "" {
		t.Skip("set INTEGRATION_REDIS_ADDR to run the compose-backed integration test")
	}
	client := redis.NewClient(&redis.Options{Addr: address, Password: os.Getenv("INTEGRATION_REDIS_PASSWORD")})
	defer func() { _ = client.Close() }()
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	key := "integration-" + time.Now().Format("150405.000000000")
	lim := New(client, 1, time.Minute)
	first, _, err := lim.Allow(ctx, key)
	if err != nil || !first {
		t.Fatalf("first allowed=%v err=%v", first, err)
	}
	second, retry, err := lim.Allow(ctx, key)
	if err != nil || second || retry <= 0 {
		t.Fatalf("second allowed=%v retry=%s err=%v", second, retry, err)
	}
}
