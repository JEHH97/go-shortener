package clicks

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jesush1236/go-shortener/internal/store"
)

type Worker struct {
	store   store.LinkStore
	queue   chan store.Click
	workers int
	batch   int
	logger  *slog.Logger
	start   sync.Once
	stop    sync.Once
	wg      sync.WaitGroup
}

func New(source store.LinkStore, buffer, workers, batch int, logger *slog.Logger) *Worker {
	if buffer < 1 {
		buffer = 1
	}
	if workers < 1 {
		workers = 1
	}
	if batch < 1 {
		batch = 1
	}
	return &Worker{store: source, queue: make(chan store.Click, buffer), workers: workers, batch: batch, logger: logger}
}

func (w *Worker) Start() {
	w.start.Do(func() {
		for i := 0; i < w.workers; i++ {
			w.wg.Add(1)
			go w.run()
		}
	})
}

func (w *Worker) Record(click store.Click) bool {
	select {
	case w.queue <- click:
		return true
	default:
		if w.logger != nil {
			w.logger.Warn("click queue full; dropping event", "code", click.Code)
		}
		return false
	}
}

func (w *Worker) run() {
	defer w.wg.Done()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	batch := make([]store.Click, 0, w.batch)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := w.store.RecordClicks(ctx, batch)
		cancel()
		if err != nil && w.logger != nil {
			w.logger.Error("click batch write failed", "count", len(batch), "error", err)
		}
		batch = batch[:0]
	}
	for {
		select {
		case click, ok := <-w.queue:
			if !ok {
				flush()
				return
			}
			batch = append(batch, click)
			if len(batch) >= w.batch {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (w *Worker) Stop(timeout time.Duration) {
	w.stop.Do(func() { close(w.queue) })
	done := make(chan struct{})
	go func() { w.wg.Wait(); close(done) }()
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	select {
	case <-done:
	case <-time.After(timeout):
		if w.logger != nil {
			w.logger.Error("click worker flush timed out")
		}
	}
}
