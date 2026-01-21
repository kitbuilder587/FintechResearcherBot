package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/kitbuilder587/fintech-bot/internal/llm/mock"
	"github.com/kitbuilder587/fintech-bot/internal/search"
	"go.uber.org/zap"
)

func TestTrendsAgent_CanHandle(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()
	agent := NewTrendsAgent(mockLLM, logger)

	tests := []struct {
		name        string
		question    string
		shouldMatch bool
		minScore    float64
	}{
		{
			name:        "matches trend keyword",
			question:    "What are the current trends?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches startup keyword",
			question:    "Which startups are leading?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches innovation keyword",
			question:    "What innovations are emerging?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches future keyword",
			question:    "What is the future of fintech?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches emerging keyword",
			question:    "What emerging technologies matter?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches AI keyword",
			question:    "How is AI changing fintech?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches machine learning keyword",
			question:    "Machine learning in payments",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches Russian keyword - trend",
			question:    "Какой тренд в финтехе?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches Russian keyword - startup",
			question:    "Какой стартап лидирует?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "no match for market question",
			question:    "What is the market valuation?",
			shouldMatch: false,
			minScore:    0.0,
		},
		{
			name:        "no match for regulatory question",
			question:    "What license is required?",
			shouldMatch: false,
			minScore:    0.0,
		},
		{
			name:        "matches multiple keywords",
			question:    "AI and machine learning trends in startups",
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

func TestTrendsAgent_Process(t *testing.T) {
	logger := zap.NewNop()

	t.Run("successful processing with insights", func(t *testing.T) {
		mockLLM := mock.New().WithResponse(`Trends analysis in fintech innovation.

Инсайты:
- AI автоматизирует кредитный скоринг
- Embedded finance растет на 30%
- Необанки захватывают молодежь

Sources: [S1]`)

		agent := NewTrendsAgent(mockLLM, logger)

		req := AgentRequest{
			Question: "What are the fintech trends?",
			SearchResults: []search.SearchResult{
				{Title: "Trends Report", URL: "https://example.com/trends", Content: "Trends info..."},
			},
			Context: "Fintech innovation landscape",
		}

		resp, err := agent.Process(context.Background(), req)

		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}

		if resp.AgentName != "trends-analyst" {
			t.Errorf("AgentName = %v, expected %v", resp.AgentName, "trends-analyst")
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

		if !strings.Contains(mockLLM.LastSystem, "инновац") && !strings.Contains(mockLLM.LastSystem, "trend") && !strings.Contains(mockLLM.LastSystem, "innovation") {
			t.Error("System prompt should mention trends/innovation focus")
		}
	})

	t.Run("handles empty question error", func(t *testing.T) {
		mockLLM := mock.New()
		agent := NewTrendsAgent(mockLLM, logger)

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
		agent := NewTrendsAgent(mockLLM, logger)

		req := AgentRequest{
			Question: "What are the trends?",
		}

		_, err := agent.Process(context.Background(), req)

		if err == nil {
			t.Error("Process() expected error when LLM fails")
		}
	})
}

func TestTrendsAgent_Name(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()
	agent := NewTrendsAgent(mockLLM, logger)

	if agent.Name() != "trends-analyst" {
		t.Errorf("Name() = %v, expected trends-analyst", agent.Name())
	}
}

func TestTrendsAgent_Expertise(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()
	agent := NewTrendsAgent(mockLLM, logger)

	expertise := agent.Expertise()

	if len(expertise) == 0 {
		t.Error("Expertise() should not be empty")
	}

	hasTrends := false
	for _, exp := range expertise {
		expLower := strings.ToLower(exp)
		if strings.Contains(expLower, "trend") || strings.Contains(expLower, "innovation") || strings.Contains(expLower, "startup") {
			hasTrends = true
			break
		}
	}

	if !hasTrends {
		t.Error("Expertise should include trends-related topics")
	}
}
