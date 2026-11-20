package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jesush1236/go-shortener/internal/domain"
	"github.com/jesush1236/go-shortener/internal/store"
)

func TestPostgresIntegration(t *testing.T) {
	connection := os.Getenv("INTEGRATION_DATABASE_URL")
	if connection == "" {
		t.Skip("set INTEGRATION_DATABASE_URL to run the compose-backed integration test")
	}
	ctx := context.Background()
	db, err := Open(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	code := "integration-" + time.Now().UTC().Format("150405.000000000")
	link := domain.Link{Code: code, URL: "https://example.org/integration", CreatedAt: time.Now().UTC()}
	if err := db.CreateLink(ctx, link); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetLink(ctx, code)
	if err != nil || got.URL != link.URL {
		t.Fatalf("GetLink=%+v err=%v", got, err)
	}
	click := store.Click{Code: code, At: time.Now().UTC(), Referer: "https://ref.example", Country: "US", UserAgent: "browser", UserAgentFamily: "Chrome"}
	if err := db.RecordClicks(ctx, []store.Click{click}); err != nil {
		t.Fatal(err)
	}
	stats, err := db.GetStats(ctx, code)
	if err != nil || stats.Total != 1 {
		t.Fatalf("GetStats=%+v err=%v", stats, err)
	}
	if err := db.DeleteLink(ctx, code); err != nil {
		t.Fatal(err)
	}
}
