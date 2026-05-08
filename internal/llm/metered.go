package llm

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kitbuilder587/fintech-bot/internal/metrics"
)

type MeteredClient struct {
	next     Client
	metrics  *metrics.Metrics
	provider string
	model    string
}

func WithMetrics(next Client, m *metrics.Metrics, provider, model string) Client {
	if next == nil || m == nil {
		return next
	}
	return &MeteredClient{
		next:     next,
		metrics:  m,
		provider: provider,
		model:    model,
	}
}

func (c *MeteredClient) CompleteWithSystem(ctx context.Context, system, prompt string) (string, error) {
	start := time.Now()

	if usageClient, ok := c.next.(UsageClient); ok {
		content, usage, err := usageClient.CompleteWithSystemUsage(ctx, system, prompt)
		if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.TotalTokens == 0 {
			usage = Usage{
				InputTokens:  estimateTokenCount(system) + estimateTokenCount(prompt),
				OutputTokens: estimateTokenCount(content),
			}
			usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		}
		c.record(ctx, err, time.Since(start), usage)
		return content, err
	}

	content, err := c.next.CompleteWithSystem(ctx, system, prompt)
	usage := Usage{
		InputTokens:  estimateTokenCount(system) + estimateTokenCount(prompt),
		OutputTokens: estimateTokenCount(content),
	}
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	c.record(ctx, err, time.Since(start), usage)
	return content, err
}

func (c *MeteredClient) record(ctx context.Context, err error, duration time.Duration, usage Usage) {
	status := "success"
	if err != nil {
		status = "error"
	}

	c.metrics.RecordLLMRequest(c.provider, status, duration)
	if usage.CostUSD == 0 {
		usage.CostUSD = estimateCostUSD(usage)
	}
	c.metrics.RecordLLMUsage(c.provider, c.model, usage.InputTokens, usage.OutputTokens, usage.TotalTokens, usage.CostUSD)
	RecordUsageFromContext(ctx, usage)
}

func estimateCostUSD(usage Usage) float64 {
	inputPrice, _ := strconv.ParseFloat(os.Getenv("LLM_INPUT_PRICE_PER_1M"), 64)
	outputPrice, _ := strconv.ParseFloat(os.Getenv("LLM_OUTPUT_PRICE_PER_1M"), 64)
	return (float64(usage.InputTokens)*inputPrice + float64(usage.OutputTokens)*outputPrice) / 1_000_000
}

func estimateTokenCount(text string) int {
	words := len(strings.Fields(text))
	if words == 0 {
		return 0
	}
	tokens := int(float64(words) * 1.3)
	if tokens < 1 {
		return 1
	}
	return tokens
}
