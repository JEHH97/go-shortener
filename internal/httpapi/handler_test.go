package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jesush1236/go-shortener/internal/domain"
	"github.com/jesush1236/go-shortener/internal/links"
	"github.com/jesush1236/go-shortener/internal/metrics"
	"github.com/jesush1236/go-shortener/internal/store"
)

type memoryStore struct {
	values   map[string]domain.Link
	recorded []store.Click
}

func newMemoryStore() *memoryStore { return &memoryStore{values: map[string]domain.Link{}} }
func (m *memoryStore) CreateLink(_ context.Context, l domain.Link) error {
	if _, ok := m.values[l.Code]; ok {
		return store.ErrConflict
	}
	m.values[l.Code] = l
	return nil
}
func (m *memoryStore) GetLink(_ context.Context, code string) (domain.Link, error) {
	l, ok := m.values[code]
	if !ok {
		return domain.Link{}, pgx.ErrNoRows
	}
	return l, nil
}
func (m *memoryStore) DeleteLink(_ context.Context, code string) error {
	delete(m.values, code)
	return nil
}
func (m *memoryStore) GetStats(context.Context, string) (store.Stats, error) {
	return store.Stats{Total: int64(len(m.recorded))}, nil
}
func (m *memoryStore) RecordClicks(_ context.Context, c []store.Click) error {
	m.recorded = append(m.recorded, c...)
	return nil
}

type memoryCache struct{ data map[string]domain.Link }

func (m memoryCache) GetLink(_ context.Context, code string) (domain.Link, bool, error) {
	l, ok := m.data[code]
	return l, ok, nil
}
func (m memoryCache) SetLink(_ context.Context, l domain.Link, _ time.Duration) error {
	m.data[l.Code] = l
	return nil
}
func (m memoryCache) DeleteLink(_ context.Context, code string) error {
	delete(m.data, code)
	return nil
}

type allowAll struct{ allowed bool }

func (a allowAll) Allow(context.Context, string) (bool, time.Duration, error) {
	return a.allowed, time.Minute, nil
}

type clickSink struct{ clicks []store.Click }

func (c *clickSink) Record(click store.Click) bool { c.clicks = append(c.clicks, click); return true }

func testHandler() (*Handler, *memoryStore, *clickSink) {
	db := newMemoryStore()
	cache := memoryCache{data: map[string]domain.Link{}}
	service := links.New(db, cache, "http://sho.rt")
	clicks := &clickSink{}
	h := New(Options{Links: service, Limiter: allowAll{true}, Clicks: clicks, APIKey: "test-key", Metrics: metrics.New(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	return h, db, clicks
}

func TestRedirectRecordsAndCounts(t *testing.T) {
	h, db, sink := testHandler()
	db.values["abc"] = domain.Link{Code: "abc", URL: "https://example.org/page"}
	r := httptest.NewRequest(http.MethodGet, "/abc", nil)
	r.Header.Set("CF-IPCountry", "US")
	r.Header.Set("User-Agent", "Mozilla Chrome/120")
	r.Header.Set("Referer", "https://source.example/path")
	w := httptest.NewRecorder()
	h.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "https://example.org/page" {
		t.Fatalf("redirect status=%d location=%q body=%s", w.Code, w.Header().Get("Location"), w.Body)
	}
	if len(sink.clicks) != 1 || sink.clicks[0].Country != "US" || sink.clicks[0].UserAgentFamily != "Chrome" {
		t.Fatalf("click event not captured: %+v", sink.clicks)
	}
	metricsResponse := httptest.NewRecorder()
	h.Handler().ServeHTTP(metricsResponse, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(metricsResponse.Body.String(), "shortener_redirects_total") || !strings.Contains(metricsResponse.Body.String(), "shortener_redirect_duration_seconds") {
		t.Fatal("redirect metrics missing")
	}
}

func TestRedirectUnknownExpiredAndRateLimit(t *testing.T) {
	h, db, _ := testHandler()
	unknown := httptest.NewRecorder()
	h.Handler().ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if unknown.Code != 404 {
		t.Fatalf("missing status=%d", unknown.Code)
	}
	expired := time.Now().Add(-time.Hour)
	db.values["old"] = domain.Link{Code: "old", URL: "https://example.org", ExpiresAt: &expired}
	w := httptest.NewRecorder()
	h.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/old", nil))
	if w.Code != http.StatusGone {
		t.Fatalf("expired status=%d", w.Code)
	}
	h.options.Limiter = allowAll{false}
	limited := httptest.NewRecorder()
	h.Handler().ServeHTTP(limited, httptest.NewRequest(http.MethodPost, "/api/links", strings.NewReader(`{"url":"https://example.org"}`)))
	if limited.Code != http.StatusTooManyRequests || limited.Header().Get("Retry-After") == "" {
		t.Fatalf("rate limit response: %d %#v", limited.Code, limited.Header())
	}
}

func TestDeleteRequiresKeyAndDeletes(t *testing.T) {
	h, db, _ := testHandler()
	db.values["abc"] = domain.Link{Code: "abc", URL: "https://example.org"}
	bad := httptest.NewRecorder()
	h.Handler().ServeHTTP(bad, httptest.NewRequest(http.MethodDelete, "/api/links/abc", nil))
	if bad.Code != 401 {
		t.Fatalf("unauthorized status=%d", bad.Code)
	}
	r := httptest.NewRequest(http.MethodDelete, "/api/links/abc", nil)
	r.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	h.Handler().ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatalf("delete status=%d body=%s", w.Code, w.Body)
	}
}

func TestCreateRejectsBadJSONAndAlias(t *testing.T) {
	h, _, _ := testHandler()
	for _, body := range []string{"{", `{"url":"https://example.org","extra":true}`, `{"url":"https://example.org","alias":"x"}`} {
		w := httptest.NewRecorder()
		h.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/links", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Errorf("body %q status=%d response=%s", body, w.Code, w.Body)
		}
	}
}

func TestHealthAndCreateResponse(t *testing.T) {
	h, _, _ := testHandler()
	health := httptest.NewRecorder()
	h.Handler().ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != 200 {
		t.Fatalf("health=%d", health.Code)
	}
	// Avoid network DNS lookups in this handler-focused test: invalid destinations fail before persistence.
	bad := httptest.NewRecorder()
	h.Handler().ServeHTTP(bad, httptest.NewRequest(http.MethodPost, "/api/links", strings.NewReader(`{"url":"file:///etc/passwd"}`)))
	if bad.Code != 400 {
		t.Fatalf("invalid URL status=%d", bad.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(health.Body.Bytes(), &payload); err != nil || payload["status"] != "ok" {
		t.Fatalf("health JSON=%v err=%v", payload, err)
	}
}

func BenchmarkRedirectPath(b *testing.B) {
	h, db, _ := testHandler()
	db.values["abc"] = domain.Link{Code: "abc", URL: "https://example.org/page"}
	handler := h.Handler()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/abc", nil))
		if w.Code != http.StatusFound {
			b.Fatal(w.Code)
		}
	}
}
