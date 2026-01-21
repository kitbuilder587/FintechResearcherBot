package agent

import (
	"testing"

	"github.com/kitbuilder587/fintech-bot/internal/search"
)

func TestAgentType_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		agentType AgentType
		expected  bool
	}{
		{"market", AgentMarket, true},
		{"regulatory", AgentRegulatory, true},
		{"tech", AgentTech, true},
		{"trends", AgentTrends, true},
		{"unknown", AgentType("unknown"), false},
		{"empty", AgentType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.agentType.IsValid()
			if result != tt.expected {
				t.Errorf("AgentType(%q).IsValid() = %v, expected %v", tt.agentType, result, tt.expected)
			}
		})
	}
}

// проверяем keyword matching
func TestBaseAgent_CanHandle(t *testing.T) {
	tests := []struct {
		name     string
		keywords []string
		question string
		minScore float64
		maxScore float64
	}{
		{"market match", []string{"market", "revenue"}, "What is the market size?", 0.5, 1.0},
		{"no match", []string{"market", "revenue"}, "How to implement API?", 0.0, 0.3},
		{"regulation", []string{"regulation", "compliance"}, "GDPR requirements", 0.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &BaseAgent{
				name:      "test-agent",
				expertise: []string{"testing"},
				keywords:  tt.keywords,
			}

			score := agent.CanHandle(tt.question)

			if score < tt.minScore {
				t.Errorf("CanHandle(%q) = %v, expected >= %v", tt.question, score, tt.minScore)
			}
			if score > tt.maxScore {
				t.Errorf("CanHandle(%q) = %v, expected <= %v", tt.question, score, tt.maxScore)
			}
		})
	}
}

func TestAgentRequest_Validate(t *testing.T) {
	validResults := []search.SearchResult{
		{Title: "Test", URL: "https://example.com", Content: "test content"},
	}

	tests := []struct {
		name        string
		request     AgentRequest
		expectError bool
	}{
		{"ok", AgentRequest{Question: "valid", SearchResults: validResults}, false},
		{"empty question", AgentRequest{Question: "", SearchResults: validResults}, true},
		{"empty results", AgentRequest{Question: "valid", SearchResults: []search.SearchResult{}}, false},
		{"nil results", AgentRequest{Question: "valid", SearchResults: nil}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()

			if tt.expectError && err == nil {
				t.Errorf("Validate() expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Validate() expected nil, got %v", err)
			}
		})
	}
}

func TestAgentResponse_Validate(t *testing.T) {
	tests := []struct {
		name        string
		response    AgentResponse
		expectError bool
	}{
		{"ok", AgentResponse{AgentName: "market", Content: "answer", Confidence: 0.8}, false},
		{"empty content", AgentResponse{AgentName: "market", Content: "", Confidence: 0.8}, true},
		{"confidence > 1", AgentResponse{AgentName: "market", Content: "answer", Confidence: 1.5}, true},
		{"negative confidence", AgentResponse{AgentName: "market", Content: "answer", Confidence: -0.1}, true},
		{"confidence = 1", AgentResponse{AgentName: "market", Content: "answer", Confidence: 1.0}, false},
		{"confidence = 0", AgentResponse{AgentName: "market", Content: "answer", Confidence: 0.0}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.response.Validate()

			if tt.expectError && err == nil {
				t.Errorf("Validate() expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Validate() expected nil, got %v", err)
			}
		})
	}
}

func TestBaseAgent_Name(t *testing.T) {
	agent := &BaseAgent{
		name: "market-analyst",
	}

	if got := agent.Name(); got != "market-analyst" {
		t.Errorf("Name() = %v, expected %v", got, "market-analyst")
	}
}

func TestBaseAgent_Expertise(t *testing.T) {
	expertise := []string{"market analysis", "revenue forecasting"}
	agent := &BaseAgent{
		expertise: expertise,
	}

	got := agent.Expertise()
	if len(got) != len(expertise) {
		t.Errorf("Expertise() length = %v, expected %v", len(got), len(expertise))
	}

	for i, exp := range expertise {
		if got[i] != exp {
			t.Errorf("Expertise()[%d] = %v, expected %v", i, got[i], exp)
		}
	}
}
