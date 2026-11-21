package postgres

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNeonConnectionStringWithRequiredTLS(t *testing.T) {
	connection := "postgresql://demo:placeholder@ep-example.neon.tech/shortener?sslmode=require"
	config, err := pgxpool.ParseConfig(connection)
	if err != nil {
		t.Fatalf("parse Neon-style DATABASE_URL: %v", err)
	}
	if config.ConnConfig.TLSConfig == nil {
		t.Fatal("sslmode=require did not configure TLS")
	}
}
