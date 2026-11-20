package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	Registry      *prometheus.Registry
	Redirects     *prometheus.CounterVec
	Latency       prometheus.Histogram
	RateLimited   prometheus.Counter
	CacheHitRatio prometheus.Gauge
	mu            sync.Mutex
	cacheHits     float64
	cacheLookups  float64
}

func New() *Metrics {
	m := &Metrics{Registry: prometheus.NewRegistry()}
	m.Redirects = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "shortener_redirects_total", Help: "Redirect responses served."}, []string{"result"})
	m.Latency = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "shortener_redirect_duration_seconds", Help: "End-to-end redirect handler latency.", Buckets: prometheus.DefBuckets})
	m.RateLimited = prometheus.NewCounter(prometheus.CounterOpts{Name: "shortener_rate_limited_total", Help: "Requests rejected by the creation rate limiter."})
	m.CacheHitRatio = prometheus.NewGauge(prometheus.GaugeOpts{Name: "shortener_cache_hit_ratio", Help: "Observed link-cache hit ratio."})
	m.Registry.MustRegister(m.Redirects, m.Latency, m.RateLimited, m.CacheHitRatio)
	return m
}

func (m *Metrics) ObserveCache(hit bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheLookups++
	if hit {
		m.cacheHits++
	}
	if m.cacheLookups > 0 {
		m.CacheHitRatio.Set(m.cacheHits / m.cacheLookups)
	}
}
