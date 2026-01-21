package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/kitbuilder587/fintech-bot/internal/llm/mock"
	"github.com/kitbuilder587/fintech-bot/internal/search"
	"go.uber.org/zap"
)

func TestRegulatoryAgent_CanHandle(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()
	agent := NewRegulatoryAgent(mockLLM, logger)

	tests := []struct {
		name        string
		question    string
		shouldMatch bool
		minScore    float64
	}{
		{
			name:        "matches regulation keyword",
			question:    "What regulations apply to fintech?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches compliance keyword",
			question:    "How to ensure compliance?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches license keyword",
			question:    "What license is required?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches law keyword",
			question:    "What law governs this?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches legal keyword",
			question:    "What are the legal requirements?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches GDPR keyword",
			question:    "What are GDPR requirements?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches PSD2 keyword",
			question:    "How does PSD2 affect us?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches Russian keyword - CB (Central Bank)",
			question:    "Какие требования ЦБ?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches Russian keyword - zakon",
			question:    "Какой закон регулирует?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "matches Russian keyword - litsenziya",
			question:    "Нужна ли лицензия?",
			shouldMatch: true,
			minScore:    0.5,
		},
		{
			name:        "no match for market question",
			question:    "What is the market size?",
			shouldMatch: false,
			minScore:    0.0,
		},
		{
			name:        "no match for tech question",
			question:    "How to implement blockchain?",
			shouldMatch: false,
			minScore:    0.0,
		},
		{
			name:        "matches multiple keywords",
			question:    "GDPR compliance and licensing requirements",
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

func TestRegulatoryAgent_Process(t *testing.T) {
	logger := zap.NewNop()

	t.Run("successful processing with insights", func(t *testing.T) {
		mockLLM := mock.New().WithResponse(`Legal analysis of fintech regulations.

Инсайты:
- GDPR требует защиты данных
- Лицензия ЦБ обязательна
- PSD2 открывает API банков

Sources: [S1]`)

		agent := NewRegulatoryAgent(mockLLM, logger)

		req := AgentRequest{
			Question: "What regulations apply?",
			SearchResults: []search.SearchResult{
				{Title: "Regulation Guide", URL: "https://example.com/legal", Content: "Legal info..."},
			},
			Context: "Fintech compliance",
		}

		resp, err := agent.Process(context.Background(), req)

		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}

		if resp.AgentName != "regulatory-expert" {
			t.Errorf("AgentName = %v, expected %v", resp.AgentName, "regulatory-expert")
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

		if !strings.Contains(mockLLM.LastSystem, "юридич") && !strings.Contains(mockLLM.LastSystem, "legal") && !strings.Contains(mockLLM.LastSystem, "regulat") {
			t.Error("System prompt should mention legal/regulatory focus")
		}
	})

	t.Run("handles empty question error", func(t *testing.T) {
		mockLLM := mock.New()
		agent := NewRegulatoryAgent(mockLLM, logger)

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
		agent := NewRegulatoryAgent(mockLLM, logger)

		req := AgentRequest{
			Question: "What regulations?",
		}

		_, err := agent.Process(context.Background(), req)

		if err == nil {
			t.Error("Process() expected error when LLM fails")
		}
	})
}

func TestRegulatoryAgent_Name(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()
	agent := NewRegulatoryAgent(mockLLM, logger)

	if agent.Name() != "regulatory-expert" {
		t.Errorf("Name() = %v, expected regulatory-expert", agent.Name())
	}
}

func TestRegulatoryAgent_Expertise(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()
	agent := NewRegulatoryAgent(mockLLM, logger)

	expertise := agent.Expertise()

	if len(expertise) == 0 {
		t.Error("Expertise() should not be empty")
	}

	hasRegulatory := false
	for _, exp := range expertise {
		expLower := strings.ToLower(exp)
		if strings.Contains(expLower, "regulat") || strings.Contains(expLower, "compliance") || strings.Contains(expLower, "legal") {
			hasRegulatory = true
			break
		}
	}

	if !hasRegulatory {
		t.Error("Expertise should include regulatory-related topics")
	}
}
