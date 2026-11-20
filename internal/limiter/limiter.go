package limiter

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const slidingWindowScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)
local count = redis.call('ZCARD', key)
if count >= limit then
  local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
  local wait = window
  if oldest[2] then wait = math.max(1, tonumber(oldest[2]) + window - now) end
  return {0, wait}
end
redis.call('ZADD', key, now, member)
redis.call('PEXPIRE', key, window)
return {1, 0}
`

type Limiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

func New(client *redis.Client, limit int, window time.Duration) *Limiter {
	return &Limiter{client: client, limit: limit, window: window}
}

func (l *Limiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	now := time.Now()
	result, err := l.client.Eval(ctx, slidingWindowScript, []string{"shortener:rate:" + key}, now.UnixMilli(), l.window.Milliseconds(), l.limit, strconv.FormatInt(now.UnixNano(), 10)).Int64Slice()
	if err != nil {
		return false, 0, err
	}
	if result[0] == 1 {
		return true, 0, nil
	}
	return false, time.Duration(result[1]) * time.Millisecond, nil
}
