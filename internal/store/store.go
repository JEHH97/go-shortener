package store

import (
	"context"
	"errors"
	"time"

	"github.com/jesush1236/go-shortener/internal/domain"
)

var ErrConflict = errors.New("link code already exists")

type Click struct {
	Code, Referer, Country, UserAgent, UserAgentFamily string
	At                                                 time.Time
}

type DayCount struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}
type NamedCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}
type Stats struct {
	Total      int64        `json:"total_clicks"`
	PerDay     []DayCount   `json:"clicks_per_day"`
	Referers   []NamedCount `json:"top_referers"`
	Countries  []NamedCount `json:"countries"`
	UserAgents []NamedCount `json:"user_agents"`
}

type LinkStore interface {
	CreateLink(context.Context, domain.Link) error
	GetLink(context.Context, string) (domain.Link, error)
	DeleteLink(context.Context, string) error
	GetStats(context.Context, string) (Stats, error)
	RecordClicks(context.Context, []Click) error
}

type LinkCache interface {
	GetLink(context.Context, string) (domain.Link, bool, error)
	SetLink(context.Context, domain.Link, time.Duration) error
	DeleteLink(context.Context, string) error
}
