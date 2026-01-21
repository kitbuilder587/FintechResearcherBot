package agent

import (
	"testing"

	"github.com/kitbuilder587/fintech-bot/internal/llm/mock"
	"go.uber.org/zap"
)

func TestNewAllAgents(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()

	agents := NewAllAgents(mockLLM, logger)

	if len(agents) != 4 {
		t.Errorf("NewAllAgents() returned %d agents, expected 4", len(agents))
	}

	expectedNames := map[string]bool{
		"market-analyst":    false,
		"regulatory-expert": false,
		"tech-specialist":   false,
		"trends-analyst":    false,
	}

	for _, agent := range agents {
		name := agent.Name()
		if _, exists := expectedNames[name]; !exists {
			t.Errorf("Unexpected agent name: %s", name)
		}
		expectedNames[name] = true
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("Missing agent: %s", name)
		}
	}
}

func TestNewAllAgents_AgentsAreUsable(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()

	agents := NewAllAgents(mockLLM, logger)

	for _, agent := range agents {
		name := agent.Name()
		if name == "" {
			t.Error("Agent name should not be empty")
		}

		expertise := agent.Expertise()
		if len(expertise) == 0 {
			t.Errorf("Agent %s should have expertise", name)
		}

		score := agent.CanHandle("test question")
		if score < 0 || score > 1 {
			t.Errorf("Agent %s CanHandle returned invalid score: %v", name, score)
		}
	}
}

func TestNewAllAgents_EachAgentHandlesDifferentQuestions(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()

	agents := NewAllAgents(mockLLM, logger)

	testCases := []struct {
		question       string
		expectedLeader string
	}{
		{
			question:       "What is the market size and revenue?",
			expectedLeader: "market-analyst",
		},
		{
			question:       "GDPR compliance and licensing requirements",
			expectedLeader: "regulatory-expert",
		},
		{
			question:       "API integration and security measures",
			expectedLeader: "tech-specialist",
		},
		{
			question:       "AI and machine learning startup trends",
			expectedLeader: "trends-analyst",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.question, func(t *testing.T) {
			var maxScore float64
			var leader string

			for _, agent := range agents {
				score := agent.CanHandle(tc.question)
				if score > maxScore {
					maxScore = score
					leader = agent.Name()
				}
			}

			if leader != tc.expectedLeader {
				t.Errorf("For question %q, expected %s to lead, but got %s", tc.question, tc.expectedLeader, leader)
			}

			if maxScore < 0.5 {
				t.Errorf("For question %q, max score was %v, expected >= 0.5", tc.question, maxScore)
			}
		})
	}
}

func TestNewAllAgents_NilLogger(t *testing.T) {
	mockLLM := mock.New()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NewAllAgents panicked with nil logger: %v", r)
		}
	}()

	agents := NewAllAgents(mockLLM, nil)

	if len(agents) != 4 {
		t.Errorf("NewAllAgents() with nil logger returned %d agents, expected 4", len(agents))
	}
}
