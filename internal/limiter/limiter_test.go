package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSlidingWindowLimit(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	lim := New(client, 2, time.Minute)
	for i := 0; i < 2; i++ {
		ok, _, err := lim.Allow(context.Background(), "client")
		if err != nil || !ok {
			t.Fatalf("request %d: allowed=%v err=%v", i, ok, err)
		}
	}
	ok, retry, err := lim.Allow(context.Background(), "client")
	if err != nil || ok || retry <= 0 {
		t.Fatalf("limit response: allowed=%v retry=%v err=%v", ok, retry, err)
	}
	server.FastForward(time.Minute)
	ok, _, err = lim.Allow(context.Background(), "client")
	if err != nil || !ok {
		t.Fatalf("expired window: allowed=%v err=%v", ok, err)
	}
}
