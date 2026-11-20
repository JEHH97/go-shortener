package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jesush1236/go-shortener/internal/domain"
	"github.com/redis/go-redis/v9"
)

type Store struct{ client *redis.Client }

func Open(ctx context.Context, address, password string) (*Store, error) {
	client := redis.NewClient(&redis.Options{Addr: address, Password: password})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Store{client: client}, nil
}
func (s *Store) Close() error { return s.client.Close() }

func (s *Store) GetLink(ctx context.Context, code string) (domain.Link, bool, error) {
	value, err := s.client.Get(ctx, "shortener:link:"+code).Bytes()
	if errors.Is(err, redis.Nil) {
		return domain.Link{}, false, nil
	}
	if err != nil {
		return domain.Link{}, false, err
	}
	var link domain.Link
	if err := json.Unmarshal(value, &link); err != nil {
		return domain.Link{}, false, err
	}
	return link, true, nil
}

func (s *Store) SetLink(ctx context.Context, link domain.Link, ttl time.Duration) error {
	value, err := json.Marshal(link)
	if err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	return s.client.Set(ctx, "shortener:link:"+link.Code, value, ttl).Err()
}

func (s *Store) DeleteLink(ctx context.Context, code string) error {
	return s.client.Del(ctx, "shortener:link:"+code).Err()
}

func (s *Store) Client() *redis.Client { return s.client }
