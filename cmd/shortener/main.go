package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jesush1236/go-shortener/internal/clicks"
	"github.com/jesush1236/go-shortener/internal/config"
	"github.com/jesush1236/go-shortener/internal/httpapi"
	"github.com/jesush1236/go-shortener/internal/limiter"
	"github.com/jesush1236/go-shortener/internal/links"
	"github.com/jesush1236/go-shortener/internal/metrics"
	"github.com/jesush1236/go-shortener/internal/store/postgres"
	"github.com/jesush1236/go-shortener/internal/store/redisstore"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		healthcheck()
		return
	}

	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	ctx := context.Background()
	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	cache, err := redisstore.Open(ctx, cfg.RedisURL, cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		logger.Error("redis connection failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = cache.Close() }()
	worker := clicks.New(db, cfg.ClickBuffer, cfg.ClickWorkers, cfg.ClickBatch, logger)
	worker.Start()
	defer worker.Stop(10 * time.Second)
	service := links.New(db, cache, cfg.BaseURL)
	api := httpapi.New(httpapi.Options{
		Links: service, Limiter: limiter.New(cache.Client(), cfg.RateLimit, cfg.RateWindow),
		Clicks: worker, APIKey: cfg.APIKey, Metrics: metrics.New(), Logger: logger,
	})
	server := &http.Server{Addr: cfg.Address, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		logger.Info("server listening", "address", cfg.Address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()
	stop, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	<-stop.Done()
	shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	if err := server.Shutdown(shutdown); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

func healthcheck() {
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get("http://127.0.0.1:8080/healthz")
	if err != nil {
		os.Exit(1)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
