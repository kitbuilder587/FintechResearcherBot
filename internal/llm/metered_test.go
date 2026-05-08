package llm_test

import (
	"context"
	"testing"

	"github.com/kitbuilder587/fintech-bot/internal/llm"
	"github.com/kitbuilder587/fintech-bot/internal/llm/mock"
	"github.com/kitbuilder587/fintech-bot/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestMeteredClientRecordsUsageAndEstimatedCost(t *testing.T) {
	t.Setenv("LLM_INPUT_PRICE_PER_1M", "2")
	t.Setenv("LLM_OUTPUT_PRICE_PER_1M", "4")

	m := metrics.NewWithRegisterer(prometheus.NewRegistry())
	base := mock.New()
	base.Response = "answer"
	base.Usage = llm.Usage{
		InputTokens:  10,
		OutputTokens: 5,
		TotalTokens:  15,
	}

	client := llm.WithMetrics(base, m, "openrouter", "test-model")
	got, err := client.CompleteWithSystem(context.Background(), "system", "prompt")
	if err != nil {
		t.Fatalf("CompleteWithSystem() error = %v", err)
	}
	if got != "answer" {
		t.Fatalf("answer = %q, want answer", got)
	}

	if got := counterValue(t, m.LLMRequestsTotal.WithLabelValues("openrouter", "success")); got != 1 {
		t.Fatalf("llm requests = %v, want 1", got)
	}
	if got := counterValue(t, m.LLMTokensTotal.WithLabelValues("openrouter", "test-model", "input")); got != 10 {
		t.Fatalf("input tokens = %v, want 10", got)
	}
	if got := counterValue(t, m.LLMTokensTotal.WithLabelValues("openrouter", "test-model", "output")); got != 5 {
		t.Fatalf("output tokens = %v, want 5", got)
	}
	if got := counterValue(t, m.LLMTokensTotal.WithLabelValues("openrouter", "test-model", "total")); got != 15 {
		t.Fatalf("total tokens = %v, want 15", got)
	}
	if got := counterValue(t, m.LLMCostUSDTotal.WithLabelValues("openrouter", "test-model")); got != 0.00004 {
		t.Fatalf("cost = %v, want 0.00004", got)
	}
}

func counterValue(t *testing.T, metric prometheus.Metric) float64 {
	t.Helper()

	dtoMetric := &dto.Metric{}
	if err := metric.Write(dtoMetric); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	return dtoMetric.GetCounter().GetValue()
}
