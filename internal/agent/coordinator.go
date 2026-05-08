package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/llm"
	"github.com/kitbuilder587/fintech-bot/internal/prompts"
	"go.uber.org/zap"
)

var ErrNoAgentResponses = errors.New("no agent responses received")

type CoordinatorResponse struct {
	FinalAnswer    string
	AgentResponses []AgentResponse
	AgentsUsed     []string
	ProcessingTime time.Duration
}

// Coordinator оркестрирует несколько агентов для ответа на вопрос
type Coordinator struct {
	agents        []Agent
	llm           llm.Client
	logger        *zap.Logger
	minConfidence float64
}

func NewCoordinator(agents []Agent, llmClient llm.Client, logger *zap.Logger) *Coordinator {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Coordinator{
		agents:        agents,
		llm:           llmClient,
		logger:        logger,
		minConfidence: 0.3,
	}
}

func (c *Coordinator) Process(ctx context.Context, req AgentRequest) (*CoordinatorResponse, error) {
	start := time.Now()

	if err := req.Validate(); err != nil {
		return nil, err
	}

	maxAgents := c.maxAgentsFor(req.Strategy)
	selected := c.selectAgents(req.Question, maxAgents)
	if plannerEnabled() {
		if planned, err := c.planAgents(ctx, req, maxAgents); err == nil && len(planned) > 0 {
			selected = planned
		} else if err != nil {
			c.logger.Warn("planner failed, using keyword routing", zap.Error(err))
		}
	}

	c.logger.Info("Selected agents",
		zap.Int("count", len(selected)),
		zap.String("strategy", string(req.Strategy.Type)),
	)

	responses := c.runParallel(ctx, selected, req)
	if len(responses) == 0 {
		return nil, ErrNoAgentResponses
	}

	names := make([]string, len(responses))
	for i, r := range responses {
		names[i] = r.AgentName
	}

	// для quick стратегии не синтезируем, просто берем ответ
	var answer string
	if len(responses) == 1 {
		answer = responses[0].Content
	} else {
		var err error
		answer, err = c.synthesize(ctx, responses, req.Question)
		if err != nil {
			return nil, fmt.Errorf("synthesis failed: %w", err)
		}
	}

	return &CoordinatorResponse{
		FinalAnswer:    answer,
		AgentResponses: responses,
		AgentsUsed:     names,
		ProcessingTime: time.Since(start),
	}, nil
}

// selectAgents выбирает агентов по релевантности вопросу
func (c *Coordinator) selectAgents(question string, maxAgents int) []Agent {
	if len(c.agents) == 0 {
		return nil
	}

	type scored struct {
		a     Agent
		score float64
	}
	scores := make([]scored, 0, len(c.agents))
	for _, a := range c.agents {
		scores = append(scores, scored{a, a.CanHandle(question)})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	var result []Agent
	for _, s := range scores {
		if s.score >= c.minConfidence {
			result = append(result, s.a)
		}
	}

	// если никто не прошел порог - берем всех (fallback)
	if len(result) == 0 {
		for _, s := range scores {
			result = append(result, s.a)
		}
	}

	if len(result) > maxAgents {
		result = result[:maxAgents]
	}
	return result
}

type plannerResponse struct {
	Subtasks []plannerSubtask `json:"subtasks"`
}

type plannerSubtask struct {
	Agent       string   `json:"agent"`
	Objective   string   `json:"objective"`
	SearchHints []string `json:"search_hints"`
}

func (c *Coordinator) planAgents(ctx context.Context, req AgentRequest, maxAgents int) ([]Agent, error) {
	if c.llm == nil {
		return nil, errors.New("planner llm is nil")
	}

	available := make([]string, 0, len(c.agents))
	byName := make(map[string]Agent, len(c.agents))
	for _, a := range c.agents {
		available = append(available, a.Name())
		byName[a.Name()] = a
	}

	system := "You are a cheap routing planner. Return JSON only."
	user := fmt.Sprintf(
		"Question: %s\nAvailable agents: %s\nReturn up to %d subtasks as JSON: {\"subtasks\":[{\"agent\":\"agent-name\",\"objective\":\"what to verify\",\"search_hints\":[\"query hint\"]}]}",
		req.Question,
		strings.Join(available, ", "),
		maxAgents,
	)

	raw, err := c.llm.CompleteWithSystem(ctx, system, user)
	if err != nil {
		return nil, err
	}

	var parsed plannerResponse
	if err := json.Unmarshal([]byte(extractJSONObject(raw)), &parsed); err != nil {
		return nil, err
	}

	selected := make([]Agent, 0, maxAgents)
	seen := make(map[string]bool)
	for _, subtask := range parsed.Subtasks {
		if seen[subtask.Agent] {
			continue
		}
		a, ok := byName[subtask.Agent]
		if !ok {
			continue
		}
		selected = append(selected, a)
		seen[subtask.Agent] = true
		if len(selected) >= maxAgents {
			break
		}
	}

	if len(selected) == 0 {
		return nil, errors.New("planner returned no known agents")
	}
	return selected, nil
}

func plannerEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_PLANNER_ENABLED")))
	return value == "1" || value == "true" || value == "yes"
}

func mandatesEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_MANDATES_ENABLED")))
	return value == "" || value == "1" || value == "true" || value == "yes"
}

// runParallel запускает агентов параллельно
// FIXME: было бы неплохо добавить таймаут на каждого агента отдельно
func (c *Coordinator) runParallel(ctx context.Context, agents []Agent, req AgentRequest) []AgentResponse {
	if len(agents) == 0 {
		return nil
	}

	if serialAgentsEnabled() {
		return c.runSerial(ctx, agents, req)
	}

	var mu sync.Mutex
	var results []AgentResponse
	var wg sync.WaitGroup

	for _, a := range agents {
		wg.Add(1)
		go func(agent Agent) {
			defer wg.Done()

			agentReq := req
			if mandatesEnabled() {
				agentReq.Mandate = c.buildMandate(agent, req.Question, agents)
			}

			resp, err := agent.Process(ctx, agentReq)
			if err != nil {
				c.logger.Warn("agent failed", zap.String("agent", agent.Name()), zap.Error(err))
				return
			}

			mu.Lock()
			results = append(results, *resp)
			mu.Unlock()
		}(a)
	}

	wg.Wait()
	return results
}

func (c *Coordinator) runSerial(ctx context.Context, agents []Agent, req AgentRequest) []AgentResponse {
	var results []AgentResponse
	delay := agentDelay()
	for _, a := range agents {
		agentReq := req
		if mandatesEnabled() {
			agentReq.Mandate = c.buildMandate(a, req.Question, agents)
		}

		resp, err := a.Process(ctx, agentReq)
		if err != nil {
			c.logger.Warn("agent failed", zap.String("agent", a.Name()), zap.Error(err))
			continue
		}
		results = append(results, *resp)
		if delay > 0 {
			select {
			case <-ctx.Done():
				return results
			case <-time.After(delay):
			}
		}
	}
	return results
}

func serialAgentsEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_SERIAL_EXECUTION")))
	return value == "1" || value == "true" || value == "yes"
}

func agentDelay() time.Duration {
	raw := strings.TrimSpace(os.Getenv("AGENT_REQUEST_DELAY_SECONDS"))
	if raw == "" {
		return 0
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func (c *Coordinator) buildMandate(agent Agent, question string, selected []Agent) string {
	var other []string
	for _, a := range selected {
		if a.Name() != agent.Name() {
			other = append(other, a.Name())
		}
	}

	focus := strings.Join(agent.Expertise(), ", ")
	if focus == "" {
		focus = agent.Name()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Focus only on %s aspects relevant to: %s.\n", focus, question)
	b.WriteString("Verify concrete facts, cite source markers next to factual and numeric claims, and avoid repeating generic background.\n")
	if len(other) > 0 {
		fmt.Fprintf(&b, "Do not cover areas assigned to other agents unless needed for context: %s.\n", strings.Join(other, ", "))
	}
	b.WriteString("Return only findings inside this mandate.")
	return b.String()
}

func (c *Coordinator) synthesize(ctx context.Context, responses []AgentResponse, question string) (string, error) {
	if len(responses) == 0 {
		return "", nil
	}

	var buf strings.Builder
	for i, r := range responses {
		fmt.Fprintf(&buf, "[Expert %d: %s]\n%s\n\n", i+1, r.AgentName, r.Content)
	}

	sysPrompt, err := prompts.Render("synthesizer_system.tmpl", struct {
		Responses string
	}{
		Responses: buf.String(),
	})
	if err != nil {
		return "", err
	}

	userPrompt, err := prompts.Render("synthesizer_user.tmpl", struct {
		Question string
	}{
		Question: question,
	})
	if err != nil {
		return "", err
	}

	return c.llm.CompleteWithSystem(ctx, sysPrompt, userPrompt)
}

func (c *Coordinator) maxAgentsFor(s domain.Strategy) int {
	switch s.Type {
	case domain.StrategyQuick:
		return 1
	case domain.StrategyStandard:
		return 2
	case domain.StrategyDeep:
		return 4
	}
	return 2
}
