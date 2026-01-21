package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/llm/mock"
	"go.uber.org/zap"
)

type mockAgent struct {
	name       string
	expertise  []string
	canHandle  float64
	response   *AgentResponse
	err        error
	delay      time.Duration
	processCnt int
}

func newMockAgent(name string, canHandle float64) *mockAgent {
	return &mockAgent{
		name:      name,
		expertise: []string{name + " expertise"},
		canHandle: canHandle,
		response: &AgentResponse{
			AgentName:  name,
			Content:    "Response from " + name,
			Confidence: canHandle,
			SourceRefs: []string{"[S1]"},
			Insights:   []string{name + " insight"},
		},
	}
}

func (m *mockAgent) Name() string {
	return m.name
}

func (m *mockAgent) Expertise() []string {
	return m.expertise
}

func (m *mockAgent) CanHandle(question string) float64 {
	return m.canHandle
}

func (m *mockAgent) Process(ctx context.Context, req AgentRequest) (*AgentResponse, error) {
	m.processCnt++

	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.delay):
		}
	}

	if m.err != nil {
		return nil, m.err
	}

	return m.response, nil
}

func (m *mockAgent) withError(err error) *mockAgent {
	m.err = err
	return m
}

func (m *mockAgent) withDelay(d time.Duration) *mockAgent {
	m.delay = d
	return m
}

func (m *mockAgent) withResponse(resp *AgentResponse) *mockAgent {
	m.response = resp
	return m
}

func TestCoordinator_SelectAgents(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()

	t.Run("selects agents with score >= minConfidence", func(t *testing.T) {
		agents := []Agent{
			newMockAgent("high", 0.8),
			newMockAgent("medium", 0.5),
			newMockAgent("low", 0.2),
		}

		coord := NewCoordinator(agents, mockLLM, logger)
		coord.minConfidence = 0.3

		selected := coord.selectAgents("test question", 4)

		if len(selected) != 2 {
			t.Errorf("selectAgents() selected %d agents, expected 2", len(selected))
		}

		names := make(map[string]bool)
		for _, a := range selected {
			names[a.Name()] = true
		}

		if !names["high"] || !names["medium"] {
			t.Error("selectAgents() should select high and medium agents")
		}
		if names["low"] {
			t.Error("selectAgents() should not select low confidence agent")
		}
	})

	t.Run("respects maxAgents limit", func(t *testing.T) {
		agents := []Agent{
			newMockAgent("a1", 0.9),
			newMockAgent("a2", 0.8),
			newMockAgent("a3", 0.7),
			newMockAgent("a4", 0.6),
		}

		coord := NewCoordinator(agents, mockLLM, logger)

		selected := coord.selectAgents("test", 2)

		if len(selected) != 2 {
			t.Errorf("selectAgents() selected %d agents, expected 2 (maxAgents)", len(selected))
		}
	})

	t.Run("selects all agents when none meet threshold", func(t *testing.T) {
		agents := []Agent{
			newMockAgent("a1", 0.1),
			newMockAgent("a2", 0.2),
		}

		coord := NewCoordinator(agents, mockLLM, logger)
		coord.minConfidence = 0.5

		selected := coord.selectAgents("test", 4)

		if len(selected) != 2 {
			t.Errorf("selectAgents() selected %d agents, expected all 2 when none meet threshold", len(selected))
		}
	})

	t.Run("returns empty when no agents", func(t *testing.T) {
		coord := NewCoordinator([]Agent{}, mockLLM, logger)

		selected := coord.selectAgents("test", 4)

		if len(selected) != 0 {
			t.Errorf("selectAgents() selected %d agents, expected 0", len(selected))
		}
	})

	t.Run("sorts by confidence descending", func(t *testing.T) {
		agents := []Agent{
			newMockAgent("low", 0.3),
			newMockAgent("high", 0.9),
			newMockAgent("medium", 0.6),
		}

		coord := NewCoordinator(agents, mockLLM, logger)
		coord.minConfidence = 0.2

		selected := coord.selectAgents("test", 2)

		if len(selected) != 2 {
			t.Fatalf("selectAgents() selected %d agents, expected 2", len(selected))
		}

		if selected[0].Name() != "high" {
			t.Errorf("First selected agent should be 'high', got %q", selected[0].Name())
		}
		if selected[1].Name() != "medium" {
			t.Errorf("Second selected agent should be 'medium', got %q", selected[1].Name())
		}
	})
}

func TestCoordinator_RunParallel(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()

	t.Run("runs all agents in parallel", func(t *testing.T) {
		agents := []Agent{
			newMockAgent("a1", 0.8).withDelay(50 * time.Millisecond),
			newMockAgent("a2", 0.7).withDelay(50 * time.Millisecond),
			newMockAgent("a3", 0.6).withDelay(50 * time.Millisecond),
		}

		coord := NewCoordinator(nil, mockLLM, logger)

		req := AgentRequest{
			Question: "test question",
			Strategy: domain.QuickStrategy(),
		}

		start := time.Now()
		responses := coord.runParallel(context.Background(), agents, req)
		elapsed := time.Since(start)

		if elapsed > 100*time.Millisecond {
			t.Errorf("runParallel took %v, expected ~50ms for parallel execution", elapsed)
		}

		if len(responses) != 3 {
			t.Errorf("runParallel returned %d responses, expected 3", len(responses))
		}

		for _, a := range agents {
			ma := a.(*mockAgent)
			if ma.processCnt != 1 {
				t.Errorf("Agent %s was called %d times, expected 1", ma.name, ma.processCnt)
			}
		}
	})

	t.Run("returns responses from all successful agents", func(t *testing.T) {
		agents := []Agent{
			newMockAgent("market", 0.8),
			newMockAgent("tech", 0.7),
		}

		coord := NewCoordinator(nil, mockLLM, logger)

		req := AgentRequest{
			Question: "test",
			Strategy: domain.StandardStrategy(),
		}

		responses := coord.runParallel(context.Background(), agents, req)

		if len(responses) != 2 {
			t.Errorf("Expected 2 responses, got %d", len(responses))
		}

		names := make(map[string]bool)
		for _, r := range responses {
			names[r.AgentName] = true
		}

		if !names["market"] || !names["tech"] {
			t.Error("Expected responses from both market and tech agents")
		}
	})
}

func TestCoordinator_RunAgentsParallel_OneFailure(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()

	t.Run("continues when one agent fails", func(t *testing.T) {
		agents := []Agent{
			newMockAgent("success1", 0.8),
			newMockAgent("failed", 0.7).withError(errors.New("agent error")),
			newMockAgent("success2", 0.6),
		}

		coord := NewCoordinator(nil, mockLLM, logger)

		req := AgentRequest{
			Question: "test",
			Strategy: domain.StandardStrategy(),
		}

		responses := coord.runParallel(context.Background(), agents, req)

		if len(responses) != 2 {
			t.Errorf("Expected 2 responses (ignoring failed), got %d", len(responses))
		}

		for _, r := range responses {
			if r.AgentName == "failed" {
				t.Error("Failed agent should not be in responses")
			}
		}
	})

	t.Run("handles all agents failing", func(t *testing.T) {
		agents := []Agent{
			newMockAgent("fail1", 0.8).withError(errors.New("error1")),
			newMockAgent("fail2", 0.7).withError(errors.New("error2")),
		}

		coord := NewCoordinator(nil, mockLLM, logger)

		req := AgentRequest{
			Question: "test",
			Strategy: domain.StandardStrategy(),
		}

		responses := coord.runParallel(context.Background(), agents, req)

		if len(responses) != 0 {
			t.Errorf("Expected 0 responses when all fail, got %d", len(responses))
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		agents := []Agent{
			newMockAgent("slow", 0.8).withDelay(1 * time.Second),
		}

		coord := NewCoordinator(nil, mockLLM, logger)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		req := AgentRequest{
			Question: "test",
			Strategy: domain.StandardStrategy(),
		}

		start := time.Now()
		responses := coord.runParallel(ctx, agents, req)
		elapsed := time.Since(start)

		if elapsed > 200*time.Millisecond {
			t.Errorf("runParallel should respect context, took %v", elapsed)
		}

		if len(responses) != 0 {
			t.Errorf("Expected 0 responses after cancellation, got %d", len(responses))
		}
	})
}

func TestCoordinator_Synthesize(t *testing.T) {
	logger := zap.NewNop()

	t.Run("synthesizes multiple responses", func(t *testing.T) {
		mockLLM := mock.New().WithResponse("Synthesized answer combining all perspectives.")

		coord := NewCoordinator(nil, mockLLM, logger)

		responses := []AgentResponse{
			{AgentName: "market", Content: "Market analysis...", Confidence: 0.8},
			{AgentName: "tech", Content: "Tech analysis...", Confidence: 0.7},
		}

		result, err := coord.synthesize(context.Background(), responses, "What is the situation?")

		if err != nil {
			t.Fatalf("synthesize() error = %v", err)
		}

		if result == "" {
			t.Error("synthesize() returned empty result")
		}

		if mockLLM.CallCount != 1 {
			t.Errorf("LLM should be called once, got %d calls", mockLLM.CallCount)
		}

		if !strings.Contains(mockLLM.LastSystem, "Synthesizer") {
			t.Error("System prompt should mention Synthesizer")
		}

		if !strings.Contains(mockLLM.LastSystem, "market") {
			t.Error("System prompt should contain agent responses")
		}
		if !strings.Contains(mockLLM.LastPrompt, "What is the situation?") && !strings.Contains(mockLLM.LastSystem, "What is the situation?") {
			t.Error("Prompt should contain original question")
		}
	})

	t.Run("handles LLM error", func(t *testing.T) {
		mockLLM := mock.New().WithError(errors.New("LLM error"))

		coord := NewCoordinator(nil, mockLLM, logger)

		responses := []AgentResponse{
			{AgentName: "test", Content: "Test response", Confidence: 0.8},
		}

		_, err := coord.synthesize(context.Background(), responses, "question")

		if err == nil {
			t.Error("synthesize() should return error when LLM fails")
		}
	})

	t.Run("handles empty responses", func(t *testing.T) {
		mockLLM := mock.New()

		coord := NewCoordinator(nil, mockLLM, logger)

		result, err := coord.synthesize(context.Background(), []AgentResponse{}, "question")

		if err != nil {
			t.Fatalf("synthesize() error = %v", err)
		}

		if result != "" {
			t.Errorf("synthesize() with empty responses should return empty, got %q", result)
		}

		if mockLLM.CallCount != 0 {
			t.Error("LLM should not be called for empty responses")
		}
	})
}

func TestCoordinator_Process_QuickStrategy(t *testing.T) {
	logger := zap.NewNop()

	t.Run("uses single agent without synthesis", func(t *testing.T) {
		mockLLM := mock.New()

		agents := []Agent{
			newMockAgent("best", 0.9),
			newMockAgent("second", 0.7),
			newMockAgent("third", 0.5),
		}

		coord := NewCoordinator(agents, mockLLM, logger)

		req := AgentRequest{
			Question: "quick question",
			Strategy: domain.QuickStrategy(),
		}

		resp, err := coord.Process(context.Background(), req)

		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}

		if len(resp.AgentsUsed) != 1 {
			t.Errorf("QuickStrategy should use 1 agent, got %d", len(resp.AgentsUsed))
		}

		if resp.AgentsUsed[0] != "best" {
			t.Errorf("QuickStrategy should select best agent, got %q", resp.AgentsUsed[0])
		}

		if mockLLM.CallCount != 0 {
			t.Errorf("QuickStrategy should not call synthesis LLM, got %d calls", mockLLM.CallCount)
		}

		if !strings.Contains(resp.FinalAnswer, "best") {
			t.Error("FinalAnswer should contain the agent's response")
		}

		if len(resp.AgentResponses) != 1 {
			t.Errorf("Expected 1 agent response, got %d", len(resp.AgentResponses))
		}
	})

	t.Run("returns error for empty question", func(t *testing.T) {
		mockLLM := mock.New()
		agents := []Agent{newMockAgent("test", 0.8)}

		coord := NewCoordinator(agents, mockLLM, logger)

		req := AgentRequest{
			Question: "",
			Strategy: domain.QuickStrategy(),
		}

		_, err := coord.Process(context.Background(), req)

		if err == nil {
			t.Error("Process() should return error for empty question")
		}
	})

	t.Run("tracks processing time", func(t *testing.T) {
		mockLLM := mock.New()
		agents := []Agent{
			newMockAgent("test", 0.8).withDelay(50 * time.Millisecond),
		}

		coord := NewCoordinator(agents, mockLLM, logger)

		req := AgentRequest{
			Question: "question",
			Strategy: domain.QuickStrategy(),
		}

		resp, err := coord.Process(context.Background(), req)

		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}

		if resp.ProcessingTime < 50*time.Millisecond {
			t.Errorf("ProcessingTime should be >= 50ms, got %v", resp.ProcessingTime)
		}
	})
}

func TestCoordinator_Process_DeepStrategy(t *testing.T) {
	logger := zap.NewNop()

	t.Run("uses up to 4 agents with synthesis", func(t *testing.T) {
		mockLLM := mock.New().WithResponse("Deep synthesized answer with all perspectives integrated.")

		agents := []Agent{
			newMockAgent("market", 0.9),
			newMockAgent("tech", 0.85),
			newMockAgent("regulatory", 0.8),
			newMockAgent("trends", 0.75),
		}

		coord := NewCoordinator(agents, mockLLM, logger)

		req := AgentRequest{
			Question: "comprehensive analysis",
			Strategy: domain.DeepStrategy(),
		}

		resp, err := coord.Process(context.Background(), req)

		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}

		if len(resp.AgentsUsed) != 4 {
			t.Errorf("DeepStrategy should use 4 agents, got %d", len(resp.AgentsUsed))
		}

		if mockLLM.CallCount != 1 {
			t.Errorf("DeepStrategy should call synthesis LLM once, got %d calls", mockLLM.CallCount)
		}

		if !strings.Contains(resp.FinalAnswer, "synthesized") {
			t.Error("FinalAnswer should be the synthesized response")
		}

		if len(resp.AgentResponses) != 4 {
			t.Errorf("Expected 4 agent responses, got %d", len(resp.AgentResponses))
		}
	})

	t.Run("handles partial agent failures in deep mode", func(t *testing.T) {
		mockLLM := mock.New().WithResponse("Synthesized from available responses.")

		agents := []Agent{
			newMockAgent("success1", 0.9),
			newMockAgent("failed", 0.85).withError(errors.New("agent error")),
			newMockAgent("success2", 0.8),
		}

		coord := NewCoordinator(agents, mockLLM, logger)

		req := AgentRequest{
			Question: "analysis with some failures",
			Strategy: domain.DeepStrategy(),
		}

		resp, err := coord.Process(context.Background(), req)

		if err != nil {
			t.Fatalf("Process() should not error on partial failures, got %v", err)
		}

		if len(resp.AgentsUsed) != 2 {
			t.Errorf("Expected 2 successful agents, got %d", len(resp.AgentsUsed))
		}

		for _, name := range resp.AgentsUsed {
			if name == "failed" {
				t.Error("Failed agent should not be in AgentsUsed")
			}
		}
	})
}

func TestCoordinator_Process_StandardStrategy(t *testing.T) {
	logger := zap.NewNop()

	t.Run("uses 2 agents with synthesis", func(t *testing.T) {
		mockLLM := mock.New().WithResponse("Standard synthesized answer.")

		agents := []Agent{
			newMockAgent("first", 0.9),
			newMockAgent("second", 0.8),
			newMockAgent("third", 0.7),
		}

		coord := NewCoordinator(agents, mockLLM, logger)

		req := AgentRequest{
			Question: "standard question",
			Strategy: domain.StandardStrategy(),
		}

		resp, err := coord.Process(context.Background(), req)

		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}

		if len(resp.AgentsUsed) != 2 {
			t.Errorf("StandardStrategy should use 2 agents, got %d", len(resp.AgentsUsed))
		}

		if mockLLM.CallCount != 1 {
			t.Errorf("StandardStrategy should call synthesis LLM, got %d calls", mockLLM.CallCount)
		}
	})
}

func TestNewCoordinator(t *testing.T) {
	t.Run("creates coordinator with default minConfidence", func(t *testing.T) {
		logger := zap.NewNop()
		mockLLM := mock.New()
		agents := []Agent{newMockAgent("test", 0.5)}

		coord := NewCoordinator(agents, mockLLM, logger)

		if coord == nil {
			t.Fatal("NewCoordinator() returned nil")
		}

		if coord.minConfidence != 0.3 {
			t.Errorf("Default minConfidence should be 0.3, got %v", coord.minConfidence)
		}

		if len(coord.agents) != 1 {
			t.Errorf("Expected 1 agent, got %d", len(coord.agents))
		}
	})

	t.Run("handles nil logger", func(t *testing.T) {
		mockLLM := mock.New()
		agents := []Agent{newMockAgent("test", 0.5)}

		coord := NewCoordinator(agents, mockLLM, nil)

		if coord == nil {
			t.Fatal("NewCoordinator() should handle nil logger")
		}
	})
}

func TestCoordinator_Process_AllAgentsFail(t *testing.T) {
	logger := zap.NewNop()
	mockLLM := mock.New()

	agents := []Agent{
		newMockAgent("fail1", 0.9).withError(errors.New("error1")),
		newMockAgent("fail2", 0.8).withError(errors.New("error2")),
	}

	coord := NewCoordinator(agents, mockLLM, logger)

	req := AgentRequest{
		Question: "question",
		Strategy: domain.StandardStrategy(),
	}

	_, err := coord.Process(context.Background(), req)

	if err == nil {
		t.Error("Process() should return error when all agents fail")
	}
}
