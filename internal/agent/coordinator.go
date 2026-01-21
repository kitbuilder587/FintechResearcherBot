package agent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/llm"
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

// runParallel запускает агентов параллельно
// FIXME: было бы неплохо добавить таймаут на каждого агента отдельно
func (c *Coordinator) runParallel(ctx context.Context, agents []Agent, req AgentRequest) []AgentResponse {
	if len(agents) == 0 {
		return nil
	}

	var mu sync.Mutex
	var results []AgentResponse
	var wg sync.WaitGroup

	for _, a := range agents {
		wg.Add(1)
		go func(agent Agent) {
			defer wg.Done()

			resp, err := agent.Process(ctx, req)
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

func (c *Coordinator) synthesize(ctx context.Context, responses []AgentResponse, question string) (string, error) {
	if len(responses) == 0 {
		return "", nil
	}

	var buf strings.Builder
	for i, r := range responses {
		fmt.Fprintf(&buf, "[Expert %d: %s]\n%s\n\n", i+1, r.AgentName, r.Content)
	}

	sysPrompt := `Ты - Synthesizer, синтезируешь ответы нескольких экспертов в один связный текст.

Ответы экспертов:
` + buf.String() + `

Отвечай на русском. Объедини точки зрения экспертов, выдели где они согласны, а где расходятся.
Сохраняй ссылки на источники [S1], [S2] и т.д. Структура: сначала общая картина, потом детали, в конце выводы.`

	return c.llm.CompleteWithSystem(ctx, sysPrompt, "User question: "+question)
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
