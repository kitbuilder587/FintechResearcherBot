package agent

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/llm"
	"github.com/kitbuilder587/fintech-bot/internal/search"
	"go.uber.org/zap"
)

var (
	ErrEmptyQuestion     = errors.New("question cannot be empty")
	ErrEmptyContent      = errors.New("response content cannot be empty")
	ErrInvalidConfidence = errors.New("confidence must be between 0.0 and 1.0")
)

type AgentType string

const (
	AgentMarket     AgentType = "market"
	AgentRegulatory AgentType = "regulatory"
	AgentTech       AgentType = "tech"
	AgentTrends     AgentType = "trends"
)

func (t AgentType) IsValid() bool {
	switch t {
	case AgentMarket, AgentRegulatory, AgentTech, AgentTrends:
		return true
	}
	return false
}

func (t AgentType) String() string { return string(t) }

type Agent interface {
	Name() string
	Expertise() []string
	CanHandle(question string) float64 // 0.0-1.0
	Process(ctx context.Context, req AgentRequest) (*AgentResponse, error)
}

type AgentRequest struct {
	Question      string
	SearchResults []search.SearchResult
	Context       string // от World Model
	Strategy      domain.Strategy
}

func (r AgentRequest) Validate() error {
	if strings.TrimSpace(r.Question) == "" {
		return ErrEmptyQuestion
	}
	return nil
}

type AgentResponse struct {
	AgentName  string
	Content    string
	Confidence float64
	SourceRefs []string
	Insights   []string
}

func (r AgentResponse) Validate() error {
	if strings.TrimSpace(r.Content) == "" {
		return ErrEmptyContent
	}
	if r.Confidence < 0 || r.Confidence > 1 {
		return ErrInvalidConfidence
	}
	return nil
}

type BaseAgent struct {
	name         string
	expertise    []string
	keywords     []string
	systemPrompt string
	llmClient    llm.Client
	logger       *zap.Logger
}

func NewBaseAgent(name string, expertise, keywords []string, systemPrompt string, llmClient llm.Client, logger *zap.Logger) *BaseAgent {
	return &BaseAgent{
		name:         name,
		expertise:    expertise,
		keywords:     keywords,
		systemPrompt: systemPrompt,
		llmClient:    llmClient,
		logger:       logger,
	}
}

func (b *BaseAgent) Name() string      { return b.name }
func (b *BaseAgent) Expertise() []string { return b.expertise }

// CanHandle возвращает уверенность что агент может обработать вопрос (по ключевым словам)
func (b *BaseAgent) CanHandle(question string) float64 {
	if len(b.keywords) == 0 {
		return 0
	}

	q := strings.ToLower(question)
	matches := 0
	for _, kw := range b.keywords {
		if strings.Contains(q, strings.ToLower(kw)) {
			matches++
		}
	}

	if matches == 0 {
		return 0
	}

	// минимум 0.5 если хоть что-то совпало
	conf := float64(matches) / float64(len(b.keywords))
	if conf < 0.5 {
		conf = 0.5
	}
	return conf
}

func (b *BaseAgent) Process(ctx context.Context, req AgentRequest) (*AgentResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	userPrompt := buildUserPrompt(req)

	content, err := b.llmClient.CompleteWithSystem(ctx, b.systemPrompt, userPrompt)
	if err != nil {
		b.logger.Error("LLM call failed", zap.Error(err))
		return nil, fmt.Errorf("llm call failed: %w", err)
	}

	conf := b.CanHandle(req.Question)
	if conf < 0.5 {
		conf = 0.5
	}

	return &AgentResponse{
		AgentName:  b.name,
		Content:    content,
		Confidence: conf,
		SourceRefs: parseSourceRefs(content),
		Insights:   parseInsights(content),
	}, nil
}

func buildUserPrompt(req AgentRequest) string {
	var sb strings.Builder

	sb.WriteString("Вопрос: ")
	sb.WriteString(req.Question)
	sb.WriteString("\n\n")

	if req.Context != "" {
		sb.WriteString("Контекст: ")
		sb.WriteString(req.Context)
		sb.WriteString("\n\n")
	}

	if len(req.SearchResults) > 0 {
		sb.WriteString("Источники:\n")
		for i, r := range req.SearchResults {
			fmt.Fprintf(&sb, "[S%d] %s\nURL: %s\nСодержание: %s\n\n", i+1, r.Title, r.URL, r.Content)
		}
	}

	return sb.String()
}

// parseInsights вытаскивает инсайты из ответа LLM
// TODO: может перейти на structured output вместо парсинга регулярками?
func parseInsights(content string) []string {
	re := regexp.MustCompile(`(?i)инсайты:\s*\n((?:[-•*]\s*.+\n?)+)`)
	match := re.FindStringSubmatch(content)
	if len(match) < 2 {
		return nil
	}

	var insights []string
	itemRe := regexp.MustCompile(`[-•*]\s*(.+)`)
	for _, m := range itemRe.FindAllStringSubmatch(match[1], -1) {
		if len(m) > 1 {
			if s := strings.TrimSpace(m[1]); s != "" {
				insights = append(insights, s)
			}
		}
	}
	return insights
}

func parseSourceRefs(content string) []string {
	re := regexp.MustCompile(`\[S(\d+)\]`)
	matches := re.FindAllStringSubmatch(content, -1)

	seen := make(map[string]bool)
	var refs []string
	for _, m := range matches {
		if len(m) > 0 && !seen[m[0]] {
			seen[m[0]] = true
			refs = append(refs, m[0])
		}
	}
	return refs
}
