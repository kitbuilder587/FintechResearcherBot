package metrics

import (
	"net/http"
	"time"

	"github.com/kitbuilder587/fintech-bot/internal/eval"
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
	LLMTokensTotal     *prometheus.CounterVec
	LLMCostUSDTotal    *prometheus.CounterVec

	SearchRequestsTotal   *prometheus.CounterVec
	SearchRequestDuration *prometheus.HistogramVec

	CacheHitsTotal   prometheus.Counter
	CacheMissesTotal prometheus.Counter

	RateLimitHitsTotal *prometheus.CounterVec

	CriticConfidence     *prometheus.HistogramVec
	AnswerCitationValid  *prometheus.HistogramVec
	AnswerCitationCover  *prometheus.HistogramVec
	AnswerUncitedNumbers *prometheus.HistogramVec
	EvalFaithfulness     *prometheus.HistogramVec
	EvalAnswerRelevance  *prometheus.HistogramVec

	ActiveUsersTotal prometheus.Gauge
}

func New() *Metrics {
	return NewWithRegisterer(prometheus.DefaultRegisterer)
}

func NewWithRegisterer(registerer prometheus.Registerer) *Metrics {
	factory := promauto.With(registerer)

	m := &Metrics{
		RequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fintech_bot_requests_total",
				Help: "Total number of requests processed",
			},
			[]string{"type", "status"},
		),
		RequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fintech_bot_request_duration_seconds",
				Help:    "Request duration in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
			},
			[]string{"type"},
		),
		RequestsInFlight: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "fintech_bot_requests_in_flight",
				Help: "Number of requests currently being processed",
			},
		),

		LLMRequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fintech_bot_llm_requests_total",
				Help: "Total number of LLM API requests",
			},
			[]string{"provider", "status"},
		),
		LLMRequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fintech_bot_llm_request_duration_seconds",
				Help:    "LLM request duration in seconds",
				Buckets: []float64{0.5, 1, 2, 5, 10, 30, 60},
			},
			[]string{"provider"},
		),
		LLMTokensTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fintech_bot_llm_tokens_total",
				Help: "Total LLM tokens used",
			},
			[]string{"provider", "model", "type"},
		),
		LLMCostUSDTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fintech_bot_llm_cost_usd_total",
				Help: "Estimated LLM cost in USD",
			},
			[]string{"provider", "model"},
		),

		SearchRequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fintech_bot_search_requests_total",
				Help: "Total number of search API requests",
			},
			[]string{"status"},
		),
		SearchRequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fintech_bot_search_request_duration_seconds",
				Help:    "Search request duration in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
			},
			[]string{},
		),

		CacheHitsTotal: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "fintech_bot_cache_hits_total",
				Help: "Total number of cache hits",
			},
		),
		CacheMissesTotal: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "fintech_bot_cache_misses_total",
				Help: "Total number of cache misses",
			},
		),

		RateLimitHitsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fintech_bot_rate_limit_hits_total",
				Help: "Total number of rate limit hits",
			},
			[]string{"user_id"},
		),

		CriticConfidence: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fintech_bot_critic_confidence",
				Help:    "Critic confidence score",
				Buckets: prometheus.LinearBuckets(0, 0.1, 11),
			},
			[]string{"strategy", "approved"},
		),
		AnswerCitationValid: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fintech_bot_answer_citation_validity",
				Help:    "Share of answer citations pointing to existing sources",
				Buckets: prometheus.LinearBuckets(0, 0.1, 11),
			},
			[]string{"strategy"},
		),
		AnswerCitationCover: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fintech_bot_answer_citation_coverage",
				Help:    "Share of numeric claims followed by citations",
				Buckets: prometheus.LinearBuckets(0, 0.1, 11),
			},
			[]string{"strategy"},
		),
		AnswerUncitedNumbers: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fintech_bot_answer_uncited_numerics",
				Help:    "Number of numeric claims without nearby citations",
				Buckets: []float64{0, 1, 2, 3, 5, 10, 20},
			},
			[]string{"strategy"},
		),
		EvalFaithfulness: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fintech_bot_eval_faithfulness",
				Help:    "Offline Ragas faithfulness score",
				Buckets: prometheus.LinearBuckets(0, 0.1, 11),
			},
			[]string{"dataset"},
		),
		EvalAnswerRelevance: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fintech_bot_eval_answer_relevance",
				Help:    "Offline Ragas answer relevance score",
				Buckets: prometheus.LinearBuckets(0, 0.1, 11),
			},
			[]string{"dataset"},
		),

		ActiveUsersTotal: factory.NewGauge(
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

func (m *Metrics) RecordLLMUsage(provider, model string, inputTokens, outputTokens, totalTokens int, costUSD float64) {
	if inputTokens > 0 {
		m.LLMTokensTotal.WithLabelValues(provider, model, "input").Add(float64(inputTokens))
	}
	if outputTokens > 0 {
		m.LLMTokensTotal.WithLabelValues(provider, model, "output").Add(float64(outputTokens))
	}
	if totalTokens == 0 {
		totalTokens = inputTokens + outputTokens
	}
	if totalTokens > 0 {
		m.LLMTokensTotal.WithLabelValues(provider, model, "total").Add(float64(totalTokens))
	}
	if costUSD > 0 {
		m.LLMCostUSDTotal.WithLabelValues(provider, model).Add(costUSD)
	}
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

func (m *Metrics) RecordCriticConfidence(strategy string, approved bool, confidence float64) {
	m.CriticConfidence.WithLabelValues(strategy, boolLabel(approved)).Observe(confidence)
}

func (m *Metrics) RecordAnswerQuality(strategy string, q eval.AnswerQuality) {
	m.AnswerCitationValid.WithLabelValues(strategy).Observe(q.CitationValidity)
	m.AnswerCitationCover.WithLabelValues(strategy).Observe(q.CitationCoverage)
	m.AnswerUncitedNumbers.WithLabelValues(strategy).Observe(float64(q.UncitedNumerics))
}

func (m *Metrics) RecordEvalQuality(dataset string, faithfulness, answerRelevance float64) {
	m.EvalFaithfulness.WithLabelValues(dataset).Observe(faithfulness)
	m.EvalAnswerRelevance.WithLabelValues(dataset).Observe(answerRelevance)
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

func boolLabel(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
