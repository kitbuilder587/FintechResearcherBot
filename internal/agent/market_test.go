package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/kitbuilder587/fintech-bot/internal/llm/mock"
	"github.com/kitbuilder587/fintech-bot/internal/search"
	"go.uber.org/zap"
)

func TestMarketAgent_CanHandle(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()
	agent := NewMarketAgent(mockLLM, logger)

	tests := []struct {
		name        string
		question    string
		shouldMatch bool
		minScore    float64
	}{
		{
			name:        "matches market keyword",
			question:    "What is the market size for fintech?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches revenue keyword",
			question:    "What is the revenue of Tinkoff?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches valuation keyword",
			question:    "What is the valuation of the company?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches competitors keyword",
			question:    "Who are the main competitors?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches growth keyword",
			question:    "What is the growth rate?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches investment keyword",
			question:    "What are the investment trends?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches funding keyword",
			question:    "How much funding did they raise?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches Russian keyword - rynok",
			question:    "Какой размер рынка?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches Russian keyword - vyruchka",
			question:    "Какая выручка компании?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches Russian keyword - konkurenty",
			question:    "Кто конкуренты?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "no match for tech question",
			question:    "How does the API work?",
			shouldMatch: false,
			minScore:    0.0,
		},
		{
			name:        "no match for regulatory question",
			question:    "What are GDPR requirements?",
			shouldMatch: false,
			minScore:    0.0,
		},
		{
			name:        "matches multiple keywords",
			question:    "What is the market growth and investment trends?",
			shouldMatch: true,
			minScore:    0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := agent.CanHandle(tt.question)

			if tt.shouldMatch && score < tt.minScore {
				t.Errorf("CanHandle(%q) = %v, expected >= %v", tt.question, score, tt.minScore)
			}
			if !tt.shouldMatch && score > 0.0 {
				t.Errorf("CanHandle(%q) = %v, expected 0.0", tt.question, score)
			}
		})
	}
}

func TestMarketAgent_Process(t *testing.T) {
	logger := zap.NewNop()

	t.Run("successful processing with insights", func(t *testing.T) {
		mockLLM := mock.New().WithResponse(`Market analysis shows significant growth potential.

Инсайты:
- Рынок растет на 25% в год
- Основные игроки: Tinkoff, Сбер
- Объем рынка 500 млрд рублей

Sources: [S1], [S2]`)

		agent := NewMarketAgent(mockLLM, logger)

		req := AgentRequest{
			Question: "What is the market size?",
			SearchResults: []search.SearchResult{
				{Title: "Market Report", URL: "https://example.com/report", Content: "Market data..."},
			},
			Context: "Fintech market analysis",
		}

		resp, err := agent.Process(context.Background(), req)

		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}

		if resp.AgentName != "market-analyst" {
			t.Errorf("AgentName = %v, expected %v", resp.AgentName, "market-analyst")
		}

		if resp.Content == "" {
			t.Error("Content should not be empty")
		}

		if len(resp.Insights) != 3 {
			t.Errorf("Expected 3 insights, got %d: %v", len(resp.Insights), resp.Insights)
		}

		if resp.Confidence < 0.5 || resp.Confidence > 1.0 {
			t.Errorf("Confidence = %v, expected between 0.5 and 1.0", resp.Confidence)
		}

		if mockLLM.CallCount != 1 {
			t.Errorf("LLM CallCount = %d, expected 1", mockLLM.CallCount)
		}

		if !strings.Contains(mockLLM.LastSystem, "рыночн") && !strings.Contains(mockLLM.LastSystem, "market") {
			t.Error("System prompt should mention market analysis focus")
		}
	})

	t.Run("handles empty question error", func(t *testing.T) {
		mockLLM := mock.New()
		agent := NewMarketAgent(mockLLM, logger)

		req := AgentRequest{
			Question: "",
		}

		_, err := agent.Process(context.Background(), req)

		if err == nil {
			t.Error("Process() expected error for empty question")
		}
	})

	t.Run("handles LLM error", func(t *testing.T) {
		mockLLM := mock.New().WithError(ErrEmptyQuestion)
		agent := NewMarketAgent(mockLLM, logger)

		req := AgentRequest{
			Question: "What is the market?",
		}

		_, err := agent.Process(context.Background(), req)

		if err == nil {
			t.Error("Process() expected error when LLM fails")
		}
	})

	t.Run("processes without insights section", func(t *testing.T) {
		mockLLM := mock.New().WithResponse("Simple market analysis without insights section.")
		agent := NewMarketAgent(mockLLM, logger)

		req := AgentRequest{
			Question: "Market overview",
		}

		resp, err := agent.Process(context.Background(), req)

		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}

		if len(resp.Insights) != 0 {
			t.Errorf("Expected 0 insights when no section, got %d", len(resp.Insights))
		}
	})

	t.Run("extracts source references", func(t *testing.T) {
		mockLLM := mock.New().WithResponse("Analysis based on [S1] and [S2] data. Also [S3].")
		agent := NewMarketAgent(mockLLM, logger)

		req := AgentRequest{
			Question: "Market data",
		}

		resp, err := agent.Process(context.Background(), req)

		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}

		if len(resp.SourceRefs) != 3 {
			t.Errorf("Expected 3 source refs, got %d: %v", len(resp.SourceRefs), resp.SourceRefs)
		}
	})
}

func TestMarketAgent_Name(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()
	agent := NewMarketAgent(mockLLM, logger)

	if agent.Name() != "market-analyst" {
		t.Errorf("Name() = %v, expected market-analyst", agent.Name())
	}
}

func TestMarketAgent_Expertise(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()
	agent := NewMarketAgent(mockLLM, logger)

	expertise := agent.Expertise()

	if len(expertise) == 0 {
		t.Error("Expertise() should not be empty")
	}

	hasMarket := false
	for _, exp := range expertise {
		if strings.Contains(strings.ToLower(exp), "market") || strings.Contains(strings.ToLower(exp), "рынок") {
			hasMarket = true
			break
		}
	}

	if !hasMarket {
		t.Error("Expertise should include market-related topics")
	}
}
