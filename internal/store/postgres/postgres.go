package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jesush1236/go-shortener/internal/domain"
	"github.com/jesush1236/go-shortener/internal/store"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, connection string) (*Store, error) {
	pool, err := pgxpool.New(ctx, connection)
	if err != nil {
		return nil, err
	}
	s := &Store{pool: pool}
	if err := s.Migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	contents, err := migrationFiles.ReadFile("migrations/001_init.sql")
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, string(contents))
	return err
}

func (s *Store) CreateLink(ctx context.Context, link domain.Link) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO links(code,destination,created_at,expires_at) VALUES($1,$2,$3,$4)`, link.Code, link.URL, link.CreatedAt, link.ExpiresAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return store.ErrConflict
	}
	return err
}

func (s *Store) GetLink(ctx context.Context, code string) (domain.Link, error) {
	var link domain.Link
	err := s.pool.QueryRow(ctx, `SELECT code,destination,created_at,expires_at FROM links WHERE code=$1`, code).Scan(&link.Code, &link.URL, &link.CreatedAt, &link.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Link{}, pgx.ErrNoRows
	}
	return link, err
}

func (s *Store) DeleteLink(ctx context.Context, code string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM links WHERE code=$1`, code)
	return err
}

func (s *Store) GetStats(ctx context.Context, code string) (store.Stats, error) {
	var stats store.Stats
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM clicks WHERE code=$1`, code).Scan(&stats.Total); err != nil {
		return stats, err
	}
	var err error
	if stats.PerDay, err = s.dayCounts(ctx, code); err != nil {
		return stats, err
	}
	if stats.Referers, err = s.namedCounts(ctx, code, "referer", 10); err != nil {
		return stats, err
	}
	if stats.Countries, err = s.namedCounts(ctx, code, "country", 50); err != nil {
		return stats, err
	}
	if stats.UserAgents, err = s.namedCounts(ctx, code, "ua_family", 50); err != nil {
		return stats, err
	}
	return stats, nil
}

func (s *Store) dayCounts(ctx context.Context, code string) ([]store.DayCount, error) {
	rows, err := s.pool.Query(ctx, `SELECT to_char(date_trunc('day',clicked_at AT TIME ZONE 'UTC'),'YYYY-MM-DD'),count(*) FROM clicks WHERE code=$1 GROUP BY 1 ORDER BY 1`, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []store.DayCount
	for rows.Next() {
		var item store.DayCount
		if err := rows.Scan(&item.Day, &item.Count); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) namedCounts(ctx context.Context, code, column string, limit int) ([]store.NamedCount, error) {
	if column != "referer" && column != "country" && column != "ua_family" {
		return nil, fmt.Errorf("unsupported aggregate field %q", column)
	}
	query := fmt.Sprintf(`SELECT %s,count(*) FROM clicks WHERE code=$1 AND %s<>'' GROUP BY 1 ORDER BY 2 DESC LIMIT $2`, column, column)
	rows, err := s.pool.Query(ctx, query, code, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []store.NamedCount
	for rows.Next() {
		var item store.NamedCount
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) RecordClicks(ctx context.Context, clicks []store.Click) error {
	if len(clicks) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	batch := &pgx.Batch{}
	for _, click := range clicks {
		batch.Queue(`INSERT INTO clicks(code,clicked_at,referer,country,user_agent,ua_family) VALUES($1,$2,$3,$4,$5,$6)`, click.Code, click.At, click.Referer, click.Country, click.UserAgent, click.UserAgentFamily)
	}
	results := tx.SendBatch(ctx, batch)
	for range clicks {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return err
		}
	}
	if err := results.Close(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

var _ store.LinkStore = (*Store)(nil)
