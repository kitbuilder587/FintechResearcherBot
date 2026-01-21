package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/llm"
	"github.com/kitbuilder587/fintech-bot/internal/search"
)

const CriticSystemPrompt = `You are a critical reviewer for financial research answers.

Your task: Evaluate if the answer is accurate, complete, and well-sourced.

Check for:
1. ACCURACY: Are all claims supported by the provided sources?
2. COMPLETENESS: Does it fully answer the question?
3. HALLUCINATIONS: Are there any facts not from sources?
4. STRUCTURE: Is it well-organized?

Response format (JSON only):
{
  "approved": true/false,
  "issues": ["issue1", "issue2"],
  "suggestions": ["suggestion1"],
  "confidence": 0.0-1.0
}`

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
	var sb strings.Builder

	sb.WriteString("=== ORIGINAL QUESTION ===\n")
	sb.WriteString(question)
	sb.WriteString("\n\n")

	sb.WriteString("=== SOURCES PROVIDED ===\n")
	if len(sources) == 0 {
		sb.WriteString("No sources provided.\n")
	} else {
		for i, src := range sources {
			fmt.Fprintf(&sb, "[S%d] %s (%s)\n", i+1, src.Title, src.URL)
			content := src.Content
			if len(content) > 1500 {
				content = content[:1500] + "..."
			}
			fmt.Fprintf(&sb, "Content: %s\n\n", content)
		}
	}

	sb.WriteString("=== ANSWER TO REVIEW ===\n")
	sb.WriteString(answer)
	sb.WriteString("\n\n")

	sb.WriteString("=== INSTRUCTIONS ===\n")
	sb.WriteString("Please evaluate the answer above. Check if all claims are supported by the sources, ")
	sb.WriteString("if the answer is complete, and if there are any hallucinations or unsupported facts. ")
	sb.WriteString("Respond with JSON only.")

	return sb.String()
}

func (s *CriticService) parseResponse(llmResponse string) *domain.CriticResult {
	jsonStr := extractJSON(llmResponse)

	var result struct {
		Approved    bool     `json:"approved"`
		Issues      []string `json:"issues"`
		Suggestions []string `json:"suggestions"`
		Confidence  float64  `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		s.logger.Warn("failed to parse critic response as JSON",
			zap.Error(err),
			zap.String("response", llmResponse),
		)

		return &domain.CriticResult{
			Approved:    true,
			Issues:      []string{"critic_parse_failed: could not parse LLM response"},
			Suggestions: nil,
			Confidence:  0.3,
		}
	}

	return &domain.CriticResult{
		Approved:    result.Approved,
		Issues:      result.Issues,
		Suggestions: result.Suggestions,
		Confidence:  result.Confidence,
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
