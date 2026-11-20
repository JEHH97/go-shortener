package links

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jesush1236/go-shortener/internal/domain"
	"github.com/jesush1236/go-shortener/internal/store"
)

var ErrNotFound = errors.New("link not found")

type Service struct {
	store        store.LinkStore
	cache        store.LinkCache
	baseURL      string
	resolver     domain.Resolver
	observeCache func(bool)
}

func New(source store.LinkStore, cache store.LinkCache, baseURL string) *Service {
	return &Service{store: source, cache: cache, baseURL: baseURL, resolver: netResolver{}}
}

type netResolver struct{}

func (netResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

func (s *Service) Create(ctx context.Context, rawURL, alias string, expiry *time.Time) (domain.Link, error) {
	if err := domain.ValidateAlias(alias); err != nil {
		return domain.Link{}, err
	}
	if err := domain.ValidateURL(ctx, rawURL, s.resolver); err != nil {
		return domain.Link{}, err
	}
	if expiry != nil && !expiry.After(time.Now()) {
		return domain.Link{}, fmt.Errorf("expiry must be in the future")
	}
	tries := 5
	if alias != "" {
		tries = 1
	}
	for attempt := 0; attempt < tries; attempt++ {
		code := alias
		if code == "" {
			var err error
			code, err = domain.NewCode(7)
			if err != nil {
				return domain.Link{}, err
			}
		}
		link := domain.Link{Code: code, URL: rawURL, CreatedAt: time.Now().UTC(), ExpiresAt: expiry}
		if err := s.store.CreateLink(ctx, link); err != nil {
			if errors.Is(err, store.ErrConflict) && alias == "" {
				continue
			}
			return domain.Link{}, err
		}
		return link, nil
	}
	return domain.Link{}, fmt.Errorf("could not allocate unique code: %w", store.ErrConflict)
}

func (s *Service) Get(ctx context.Context, code string) (domain.Link, error) {
	if link, found, err := s.cache.GetLink(ctx, code); err == nil {
		if s.observeCache != nil {
			s.observeCache(found)
		}
		if found {
			return link, nil
		}
	} else if s.observeCache != nil {
		s.observeCache(false)
	}
	link, err := s.store.GetLink(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Link{}, ErrNotFound
	}
	if err != nil {
		return domain.Link{}, err
	}
	ttl := time.Hour
	if link.ExpiresAt != nil {
		ttl = time.Until(*link.ExpiresAt)
	}
	_ = s.cache.SetLink(ctx, link, ttl)
	return link, nil
}

func (s *Service) Delete(ctx context.Context, code string) error {
	if err := s.store.DeleteLink(ctx, code); err != nil {
		return err
	}
	return s.cache.DeleteLink(ctx, code)
}

func (s *Service) Stats(ctx context.Context, code string) (store.Stats, error) {
	if _, err := s.Get(ctx, code); err != nil {
		return store.Stats{}, err
	}
	return s.store.GetStats(ctx, code)
}

func (s *Service) BaseURL() string { return s.baseURL }

func (s *Service) SetCacheObserver(observe func(bool)) { s.observeCache = observe }
