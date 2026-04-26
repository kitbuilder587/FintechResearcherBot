package service

import (
	"context"
	"encoding/json"
	"strings"

	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/llm"
	"github.com/kitbuilder587/fintech-bot/internal/prompts"
	"github.com/kitbuilder587/fintech-bot/internal/search"
)

var CriticSystemPrompt = prompts.Text("critic_system.md")

type CriticService struct {
	llm    llm.Client
	logger *zap.Logger
	config domain.CriticConfig
}

func NewCriticService(llmClient llm.Client, logger *zap.Logger, config domain.CriticConfig) *CriticService {
	return &CriticService{
		llm:    llmClient,
		logger: logger,
		config: config,
	}
}

func (s *CriticService) Review(ctx context.Context, answer string, sources []search.SearchResult, question string) (*domain.CriticResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.logger.Info("reviewing answer",
		zap.Int("answer_length", len(answer)),
		zap.Int("sources_count", len(sources)),
	)

	userPrompt := s.buildPrompt(answer, sources, question)
	response, err := s.llm.CompleteWithSystem(ctx, CriticSystemPrompt, userPrompt)
	if err != nil {
		s.logger.Error("LLM review failed",
			zap.Error(err),
		)
		return nil, err
	}

	result := s.parseResponse(response)

	s.logger.Info("review completed",
		zap.Bool("approved", result.Approved),
		zap.Int("issues_count", len(result.Issues)),
		zap.Float64("confidence", result.Confidence),
	)

	return result, nil
}

func (s *CriticService) buildPrompt(answer string, sources []search.SearchResult, question string) string {
	type sourceData struct {
		Index   int
		Title   string
		URL     string
		Content string
	}

	sourceItems := make([]sourceData, 0, len(sources))
	for i, src := range sources {
		content := src.Content
		if len(content) > 1500 {
			content = content[:1500] + "..."
		}
		sourceItems = append(sourceItems, sourceData{
			Index:   i + 1,
			Title:   src.Title,
			URL:     src.URL,
			Content: content,
		})
	}

	prompt, err := prompts.Render("critic_user.tmpl", struct {
		Question string
		Sources  []sourceData
		Answer   string
	}{
		Question: question,
		Sources:  sourceItems,
		Answer:   answer,
	})
	if err != nil {
		s.logger.Error("failed to render critic prompt", zap.Error(err))
		return ""
	}

	return prompt
}

func (s *CriticService) parseResponse(llmResponse string) *domain.CriticResult {
	jsonStr := extractJSON(llmResponse)

	var result struct {
		Approved      bool     `json:"approved"`
		Issues        []string `json:"issues"`
		Suggestions   []string `json:"suggestions"`
		Confidence    float64  `json:"confidence"`
		NeedsSearch   bool     `json:"needs_search"`
		SearchQueries []string `json:"search_queries"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		s.logger.Warn("failed to parse critic response as JSON",
			zap.Error(err),
			zap.String("response", llmResponse),
		)

		return &domain.CriticResult{
			Approved:      true,
			Issues:        []string{"critic_parse_failed: could not parse LLM response"},
			Suggestions:   nil,
			Confidence:    0.3,
			NeedsSearch:   false,
			SearchQueries: nil,
		}
	}

	return &domain.CriticResult{
		Approved:      result.Approved,
		Issues:        result.Issues,
		Suggestions:   result.Suggestions,
		Confidence:    result.Confidence,
		NeedsSearch:   result.NeedsSearch,
		SearchQueries: result.SearchQueries,
	}
}

// extractJSON достает JSON из ответа LLM который может содержать текст вокруг
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return s
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return s[start:]
}
