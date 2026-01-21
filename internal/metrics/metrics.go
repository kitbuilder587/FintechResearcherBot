package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RequestsInFlight prometheus.Gauge

	LLMRequestsTotal   *prometheus.CounterVec
	LLMRequestDuration *prometheus.HistogramVec

	SearchRequestsTotal   *prometheus.CounterVec
	SearchRequestDuration *prometheus.HistogramVec

	CacheHitsTotal   prometheus.Counter
	CacheMissesTotal prometheus.Counter

	RateLimitHitsTotal *prometheus.CounterVec

	ActiveUsersTotal prometheus.Gauge
}

func New() *Metrics {
	m := &Metrics{
		RequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fintech_bot_requests_total",
				Help: "Total number of requests processed",
			},
			[]string{"type", "status"},
		),
		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fintech_bot_request_duration_seconds",
				Help:    "Request duration in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
			},
			[]string{"type"},
		),
		RequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "fintech_bot_requests_in_flight",
				Help: "Number of requests currently being processed",
			},
		),

		LLMRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fintech_bot_llm_requests_total",
				Help: "Total number of LLM API requests",
			},
			[]string{"provider", "status"},
		),
		LLMRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fintech_bot_llm_request_duration_seconds",
				Help:    "LLM request duration in seconds",
				Buckets: []float64{0.5, 1, 2, 5, 10, 30, 60},
			},
			[]string{"provider"},
		),

		SearchRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fintech_bot_search_requests_total",
				Help: "Total number of search API requests",
			},
			[]string{"status"},
		),
		SearchRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fintech_bot_search_request_duration_seconds",
				Help:    "Search request duration in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
			},
			[]string{},
		),

		CacheHitsTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "fintech_bot_cache_hits_total",
				Help: "Total number of cache hits",
			},
		),
		CacheMissesTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "fintech_bot_cache_misses_total",
				Help: "Total number of cache misses",
			},
		),

		RateLimitHitsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fintech_bot_rate_limit_hits_total",
				Help: "Total number of rate limit hits",
			},
			[]string{"user_id"},
		),

		ActiveUsersTotal: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "fintech_bot_active_users",
				Help: "Number of active users in the last hour",
			},
		),
	}

	return m
}

func Handler() http.Handler {
	return promhttp.Handler()
}

func (m *Metrics) RecordRequest(reqType, status string, duration time.Duration) {
	m.RequestsTotal.WithLabelValues(reqType, status).Inc()
	m.RequestDuration.WithLabelValues(reqType).Observe(duration.Seconds())
}

func (m *Metrics) RecordLLMRequest(provider, status string, duration time.Duration) {
	m.LLMRequestsTotal.WithLabelValues(provider, status).Inc()
	m.LLMRequestDuration.WithLabelValues(provider).Observe(duration.Seconds())
}

func (m *Metrics) RecordSearchRequest(status string, duration time.Duration) {
	m.SearchRequestsTotal.WithLabelValues(status).Inc()
	m.SearchRequestDuration.WithLabelValues().Observe(duration.Seconds())
}

func (m *Metrics) RecordCacheHit() {
	m.CacheHitsTotal.Inc()
}

func (m *Metrics) RecordCacheMiss() {
	m.CacheMissesTotal.Inc()
}

func (m *Metrics) RecordRateLimitHit(userID string) {
	m.RateLimitHitsTotal.WithLabelValues(userID).Inc()
}

func (m *Metrics) SetActiveUsers(count float64) {
	m.ActiveUsersTotal.Set(count)
}

func (m *Metrics) IncRequestsInFlight() {
	m.RequestsInFlight.Inc()
}

func (m *Metrics) DecRequestsInFlight() {
	m.RequestsInFlight.Dec()
}
