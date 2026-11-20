package httpapi

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jesush1236/go-shortener/internal/domain"
	"github.com/jesush1236/go-shortener/internal/links"
	"github.com/jesush1236/go-shortener/internal/metrics"
	"github.com/jesush1236/go-shortener/internal/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed ui/index.html
var uiFiles embed.FS

type Options struct {
	Links   *links.Service
	Limiter rateLimiter
	Clicks  clickRecorder
	APIKey  string
	Metrics *metrics.Metrics
	Logger  *slog.Logger
}

// These tiny interfaces make request behavior easy to exercise without infrastructure.
type rateLimiter interface {
	Allow(context.Context, string) (bool, time.Duration, error)
}
type clickRecorder interface{ Record(store.Click) bool }

type Handler struct{ options Options }

func New(options Options) *Handler {
	if options.Metrics == nil {
		options.Metrics = metrics.New()
	}
	if options.Links != nil {
		options.Links.SetCacheObserver(options.Metrics.ObserveCache)
	}
	return &Handler{options: options}
}

func (h *Handler) Handler() http.Handler {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data, _ := uiFiles.ReadFile("ui/index.html")
		_, _ = w.Write(data)
	})
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Handle("/metrics", promhttp.HandlerFor(h.options.Metrics.Registry, promhttp.HandlerOpts{}))
	r.Post("/api/links", h.createLink)
	r.Get("/api/links/{code}/stats", h.stats)
	r.Delete("/api/links/{code}", h.deleteLink)
	r.Get("/{code}", h.redirect)
	return r
}

type createRequest struct {
	URL    string     `json:"url"`
	Alias  string     `json:"alias"`
	Expiry *time.Time `json:"expiry,omitempty"`
}

func (h *Handler) createLink(w http.ResponseWriter, r *http.Request) {
	ip := remoteIP(r.RemoteAddr)
	allowed, retry, err := h.options.Limiter.Allow(r.Context(), ip)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "rate limiter unavailable")
		return
	}
	if !allowed {
		h.options.Metrics.RateLimited.Inc()
		seconds := int(retry.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	var request createRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeError(w, http.StatusBadRequest, "request must contain one JSON object")
		return
	}
	link, err := h.options.Links.Create(r.Context(), request.URL, request.Alias, request.Expiry)
	if errors.Is(err, domain.ErrInvalidURL) || errors.Is(err, domain.ErrInvalidAlias) || (err != nil && strings.Contains(err.Error(), "expiry must")) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create link")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"code": link.Code, "short_url": strings.TrimRight(h.options.Links.BaseURL(), "/") + "/" + link.Code})
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	result := "found"
	defer func() {
		h.options.Metrics.Redirects.WithLabelValues(result).Inc()
		h.options.Metrics.Latency.Observe(time.Since(started).Seconds())
	}()
	code := chi.URLParam(r, "code")
	link, err := h.options.Links.Get(r.Context(), code)
	if errors.Is(err, links.ErrNotFound) || errors.Is(err, pgx.ErrNoRows) {
		result = "not_found"
		http.NotFound(w, r)
		return
	}
	if err != nil {
		result = "error"
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	if link.ExpiresAt != nil && !link.ExpiresAt.After(time.Now()) {
		result = "expired"
		http.Error(w, "link expired", http.StatusGone)
		return
	}
	country := r.Header.Get("CF-IPCountry")
	if country == "" {
		country = r.Header.Get("X-Country")
	}
	if len(country) > 16 {
		country = ""
	}
	if h.options.Clicks != nil {
		h.options.Clicks.Record(store.Click{Code: code, At: time.Now().UTC(), Referer: r.Referer(), Country: country, UserAgent: r.UserAgent(), UserAgentFamily: userAgentFamily(r.UserAgent())})
	}
	http.Redirect(w, r, link.URL, http.StatusFound)
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.options.Links.Stats(r.Context(), chi.URLParam(r, "code"))
	if errors.Is(err, links.ErrNotFound) {
		writeError(w, http.StatusNotFound, "link not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) deleteLink(w http.ResponseWriter, r *http.Request) {
	provided := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if h.options.APIKey == "" || len(provided) != len(h.options.APIKey) || subtle.ConstantTimeCompare([]byte(provided), []byte(h.options.APIKey)) != 1 {
		writeError(w, http.StatusUnauthorized, "valid API key required")
		return
	}
	code := chi.URLParam(r, "code")
	if _, err := h.options.Links.Get(r.Context(), code); errors.Is(err, links.ErrNotFound) {
		writeError(w, http.StatusNotFound, "link not found")
		return
	}
	if err := h.options.Links.Delete(r.Context(), code); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete link")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func remoteIP(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return remote
	}
	return host
}
func userAgentFamily(ua string) string {
	switch {
	case strings.Contains(ua, "Edg/"):
		return "Edge"
	case strings.Contains(ua, "Firefox/"):
		return "Firefox"
	case strings.Contains(ua, "Chrome/"):
		return "Chrome"
	case strings.Contains(ua, "Safari/"):
		return "Safari"
	case ua == "":
		return "Unknown"
	default:
		return "Other"
	}
}
