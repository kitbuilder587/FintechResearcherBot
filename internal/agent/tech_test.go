package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/kitbuilder587/fintech-bot/internal/llm/mock"
	"github.com/kitbuilder587/fintech-bot/internal/search"
	"go.uber.org/zap"
)

func TestTechAgent_CanHandle(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()
	agent := NewTechAgent(mockLLM, logger)

	tests := []struct {
		name        string
		question    string
		shouldMatch bool
		minScore    float64
	}{
		{
			name:        "matches API keyword",
			question:    "How does the API work?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches integration keyword",
			question:    "How to integrate with banks?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches security keyword",
			question:    "What security measures are needed?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches blockchain keyword",
			question:    "How does blockchain fit in fintech?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches infrastructure keyword",
			question:    "What infrastructure is required?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches Russian keyword - protokol",
			question:    "Какой протокол используется?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches Russian keyword - integratsiya",
			question:    "Как происходит интеграция?",
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
			question:    "What are GDPR requirements?",
			shouldMatch: false,
			minScore:    0.0,
		},
		{
			name:        "matches multiple keywords",
			question:    "API security and blockchain integration",
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

func TestTechAgent_Process(t *testing.T) {
	logger := zap.NewNop()

	t.Run("successful processing with insights", func(t *testing.T) {
		mockLLM := mock.New().WithResponse(`Technical analysis of fintech infrastructure.

Инсайты:
- REST API с OAuth 2.0
- Микросервисная архитектура
- Blockchain для транзакций

Sources: [S1], [S2]`)

		agent := NewTechAgent(mockLLM, logger)

		req := AgentRequest{
			Question: "What is the tech stack?",
			SearchResults: []search.SearchResult{
				{Title: "Tech Docs", URL: "https://example.com/tech", Content: "Tech info..."},
			},
			Context: "Fintech infrastructure",
		}

		resp, err := agent.Process(context.Background(), req)

		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}

		if resp.AgentName != "tech-specialist" {
			t.Errorf("AgentName = %v, expected %v", resp.AgentName, "tech-specialist")
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

		if !strings.Contains(mockLLM.LastSystem, "техническ") && !strings.Contains(mockLLM.LastSystem, "technical") && !strings.Contains(mockLLM.LastSystem, "tech") {
			t.Error("System prompt should mention technical focus")
		}
	})

	t.Run("handles empty question error", func(t *testing.T) {
		mockLLM := mock.New()
		agent := NewTechAgent(mockLLM, logger)

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
		agent := NewTechAgent(mockLLM, logger)

		req := AgentRequest{
			Question: "How does the API work?",
		}

		_, err := agent.Process(context.Background(), req)

		if err == nil {
			t.Error("Process() expected error when LLM fails")
		}
	})
}

func TestTechAgent_Name(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()
	agent := NewTechAgent(mockLLM, logger)

	if agent.Name() != "tech-specialist" {
		t.Errorf("Name() = %v, expected tech-specialist", agent.Name())
	}
}

func TestTechAgent_Expertise(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()
	agent := NewTechAgent(mockLLM, logger)

	expertise := agent.Expertise()

	if len(expertise) == 0 {
		t.Error("Expertise() should not be empty")
	}

	hasTech := false
	for _, exp := range expertise {
		expLower := strings.ToLower(exp)
		if strings.Contains(expLower, "api") || strings.Contains(expLower, "tech") || strings.Contains(expLower, "security") {
			hasTech = true
			break
		}
	}

	if !hasTech {
		t.Error("Expertise should include tech-related topics")
	}
}
