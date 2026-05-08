package metrics

import (
	"testing"

	quality "github.com/kitbuilder587/fintech-bot/internal/eval"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestRecordLLMUsage(t *testing.T) {
	m := NewWithRegisterer(prometheus.NewRegistry())

	m.RecordLLMUsage("openrouter", "test-model", 10, 5, 20, 0.001)

	if got := counterValue(t, m.LLMTokensTotal.WithLabelValues("openrouter", "test-model", "input")); got != 10 {
		t.Fatalf("input tokens = %v, want 10", got)
	}
	if got := counterValue(t, m.LLMTokensTotal.WithLabelValues("openrouter", "test-model", "output")); got != 5 {
		t.Fatalf("output tokens = %v, want 5", got)
	}
	if got := counterValue(t, m.LLMTokensTotal.WithLabelValues("openrouter", "test-model", "total")); got != 20 {
		t.Fatalf("total tokens = %v, want 20", got)
	}
	if got := counterValue(t, m.LLMCostUSDTotal.WithLabelValues("openrouter", "test-model")); got != 0.001 {
		t.Fatalf("cost = %v, want 0.001", got)
	}
}

func TestRecordAnswerQuality(t *testing.T) {
	m := NewWithRegisterer(prometheus.NewRegistry())

	m.RecordAnswerQuality("standard", quality.AnswerQuality{
		CitationValidity: 0.75,
		CitationCoverage: 0.5,
		UncitedNumerics:  2,
	})

	assertHistogramSum(t, observerMetric(t, m.AnswerCitationValid.WithLabelValues("standard")), 0.75)
	assertHistogramSum(t, observerMetric(t, m.AnswerCitationCover.WithLabelValues("standard")), 0.5)
	assertHistogramSum(t, observerMetric(t, m.AnswerUncitedNumbers.WithLabelValues("standard")), 2)
}

func counterValue(t *testing.T, metric prometheus.Metric) float64 {
	t.Helper()

	dtoMetric := &dto.Metric{}
	if err := metric.Write(dtoMetric); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	return dtoMetric.GetCounter().GetValue()
}

func observerMetric(t *testing.T, observer prometheus.Observer) prometheus.Metric {
	t.Helper()

	metric, ok := observer.(prometheus.Metric)
	if !ok {
		t.Fatal("observer does not implement prometheus.Metric")
	}
	return metric
}

func assertHistogramSum(t *testing.T, metric prometheus.Metric, want float64) {
	t.Helper()

	dtoMetric := &dto.Metric{}
	if err := metric.Write(dtoMetric); err != nil {
		t.Fatalf("write metric: %v", err)
	}

	histogram := dtoMetric.GetHistogram()
	if histogram.GetSampleCount() != 1 {
		t.Fatalf("histogram count = %v, want 1", histogram.GetSampleCount())
	}
	if histogram.GetSampleSum() != want {
		t.Fatalf("histogram sum = %v, want %v", histogram.GetSampleSum(), want)
	}
}
