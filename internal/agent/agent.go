package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/llm"
	"github.com/kitbuilder587/fintech-bot/internal/prompts"
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
	Mandate       string
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

func (b *BaseAgent) Name() string        { return b.name }
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

	userPrompt, err := buildUserPrompt(req)
	if err != nil {
		return nil, err
	}

	content, err := b.llmClient.CompleteWithSystem(ctx, b.systemPrompt, userPrompt)
	if err != nil {
		b.logger.Error("LLM call failed", zap.Error(err))
		return nil, fmt.Errorf("llm call failed: %w", err)
	}
	content = normalizeCitationEscapes(content)

	parsed := structuredAgentOutput{Confidence: -1}
	if structuredOutputEnabled() {
		parsed = parseStructuredAgentOutput(content)
		if parsed.Answer != "" {
			content = parsed.Answer
		}
	}

	conf := b.CanHandle(req.Question)
	if conf < 0.5 {
		conf = 0.5
	}
	if parsed.Confidence >= 0 && parsed.Confidence <= 1 {
		conf = parsed.Confidence
	}

	sourceRefs := parseSourceRefs(content)
	if len(parsed.SourceRefs) > 0 {
		sourceRefs = uniqueSourceRefs(parsed.SourceRefs)
	}
	insights := parseInsights(content)
	if len(parsed.Claims) > 0 {
		insights = parsed.Claims
	}

	return &AgentResponse{
		AgentName:  b.name,
		Content:    content,
		Confidence: conf,
		SourceRefs: sourceRefs,
		Insights:   insights,
	}, nil
}

type structuredAgentOutput struct {
	Answer     string   `json:"answer"`
	Claims     []string `json:"claims"`
	SourceRefs []string `json:"source_refs"`
	Confidence float64  `json:"confidence"`
}

func parseStructuredAgentOutput(content string) structuredAgentOutput {
	content = extractJSONObject(content)
	var out structuredAgentOutput
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return parseLenientStructuredAgentOutput(content)
	}
	return out
}

func parseLenientStructuredAgentOutput(content string) structuredAgentOutput {
	out := structuredAgentOutput{Confidence: -1}

	answerRe := regexp.MustCompile(`(?s)"answer"\s*:\s*"(.*?)"\s*,\s*"(claims|source_refs|confidence)"`)
	if match := answerRe.FindStringSubmatch(content); len(match) > 1 {
		out.Answer = cleanJSONStringFragment(match[1])
	}

	confRe := regexp.MustCompile(`"confidence"\s*:\s*([0-9.]+)`)
	if match := confRe.FindStringSubmatch(content); len(match) > 1 {
		if value, err := strconv.ParseFloat(match[1], 64); err == nil {
			out.Confidence = value
		}
	}

	out.SourceRefs = parseSourceRefs(content)
	return out
}

func cleanJSONStringFragment(value string) string {
	value = strings.ReplaceAll(value, `\n`, "\n")
	value = strings.ReplaceAll(value, `\"`, `"`)
	value = normalizeCitationEscapes(value)
	return strings.TrimSpace(value)
}

func normalizeCitationEscapes(value string) string {
	value = strings.ReplaceAll(value, `\[`, `[`)
	value = strings.ReplaceAll(value, `\]`, `]`)
	return value
}

func extractJSONObject(content string) string {
	start := strings.Index(content, "{")
	if start == -1 {
		return content
	}

	depth := 0
	for i := start; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[start : i+1]
			}
		}
	}
	return content[start:]
}

func buildUserPrompt(req AgentRequest) (string, error) {
	type sourceData struct {
		Index   int
		Title   string
		URL     string
		Content string
	}

	sources := make([]sourceData, 0, len(req.SearchResults))
	for i, r := range req.SearchResults {
		sources = append(sources, sourceData{
			Index:   i + 1,
			Title:   r.Title,
			URL:     r.URL,
			Content: r.Content,
		})
	}

	return prompts.Render("agent_user.tmpl", struct {
		Question   string
		Context    string
		Sources    []sourceData
		Mandate    string
		Structured bool
	}{
		Question:   req.Question,
		Context:    req.Context,
		Sources:    sources,
		Mandate:    req.Mandate,
		Structured: structuredOutputEnabled(),
	})
}

func structuredOutputEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_STRUCTURED_OUTPUT_ENABLED")))
	return value == "1" || value == "true" || value == "yes"
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

	refs := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 0 {
			refs = append(refs, m[0])
		}
	}
	return uniqueSourceRefs(refs)
}

func uniqueSourceRefs(items []string) []string {
	seen := make(map[string]bool)
	var refs []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			refs = append(refs, item)
		}
	}
	return refs
}
