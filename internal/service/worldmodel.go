package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/llm"
	"github.com/kitbuilder587/fintech-bot/internal/repository"
	"github.com/kitbuilder587/fintech-bot/internal/search"
)

const MaxContextSize = 2000

const ExtractionSystemPrompt = `You are a fact extraction assistant. Extract key facts and named entities from research answers.
Always respond with valid JSON only, no markdown formatting.`

const ExtractionUserPromptTemplate = `Extract key facts and entities from this research answer.

Answer:
%s

%sResponse format (JSON):
{
  "facts": [
    {"content": "...", "source_url": "...", "confidence": 0.9}
  ],
  "entities": [
    {"name": "Klarna", "type": "company", "attributes": {"founded": "2005"}}
  ]
}

Entity types: company, person, concept, product, market`

type WorldModelService struct {
	repo   repository.WorldModelRepository
	llm    llm.Client
	logger *zap.Logger
}

type KnowledgeSummary struct {
	TotalFacts     int
	TotalEntities  int
	RecentSessions []domain.ResearchSession
	TopEntities    []domain.Entity
}

type extractionResponse struct {
	Facts    []extractedFact   `json:"facts"`
	Entities []extractedEntity `json:"entities"`
}

type extractedFact struct {
	Content    string  `json:"content"`
	SourceURL  string  `json:"source_url"`
	Confidence float64 `json:"confidence"`
}

type extractedEntity struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Attributes map[string]string `json:"attributes"`
}

func NewWorldModelService(repo repository.WorldModelRepository, llmClient llm.Client, logger *zap.Logger) *WorldModelService {
	return &WorldModelService{
		repo:   repo,
		llm:    llmClient,
		logger: logger,
	}
}

func (s *WorldModelService) ExtractAndStore(ctx context.Context, userID int64, answer string, sources []search.SearchResult, question string, strategy domain.Strategy) error {
	session := &domain.ResearchSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		Question:  question,
		Strategy:  strategy.Type.String(),
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateSession(ctx, session); err != nil {
		return err
	}

	prompt := s.buildExtractionPrompt(answer, sources)
	response, err := s.llm.CompleteWithSystem(ctx, ExtractionSystemPrompt, prompt)
	if err != nil {
		return fmt.Errorf("LLM extraction: %w", err)
	}

	extracted, err := s.parseExtractionResponse(response)
	if err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	for _, f := range extracted.Facts {
		if err := s.saveFact(ctx, userID, session.ID, f); err != nil {
			s.logger.Warn("failed to save fact",
				zap.Error(err),
				zap.String("content", f.Content),
			)
		}
	}

	for _, e := range extracted.Entities {
		if err := s.saveEntity(ctx, userID, session.ID, e); err != nil {
			s.logger.Warn("failed to save entity",
				zap.Error(err),
				zap.String("name", e.Name),
			)
		}
	}

	s.logger.Info("extracted and stored world model data",
		zap.Int64("user_id", userID),
		zap.Int("facts", len(extracted.Facts)),
		zap.Int("entities", len(extracted.Entities)),
	)

	return nil
}

func (s *WorldModelService) GetRelevantContext(ctx context.Context, userID int64, question string) (string, error) {
	if question == "" {
		return "", nil
	}

	keywords := extractKeywords(question)
	if len(keywords) == 0 {
		return "", nil
	}

	factSet := make(map[string]domain.Fact)
	for _, keyword := range keywords {
		if len(keyword) < 3 { // skip very short words
			continue
		}
		facts, err := s.repo.SearchFacts(ctx, userID, keyword)
		if err != nil {
			s.logger.Warn("search facts failed", zap.Error(err), zap.String("keyword", keyword))
			continue
		}
		for _, f := range facts {
			factSet[f.ID] = f
		}
	}

	if len(factSet) == 0 {
		return "", nil
	}

	var facts []domain.Fact
	for _, f := range factSet {
		facts = append(facts, f)
	}

	var builder strings.Builder
	builder.WriteString("Relevant facts from previous research:\n")

	for _, fact := range facts {
		factLine := fmt.Sprintf("- %s", fact.Content)
		if fact.SourceURL != "" {
			factLine += fmt.Sprintf(" (source: %s)", fact.SourceURL)
		}
		factLine += "\n"

		if builder.Len()+len(factLine) > MaxContextSize {
			break
		}
		builder.WriteString(factLine)
	}

	return builder.String(), nil
}

func extractKeywords(question string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true,
		"about": true, "above": true, "after": true, "again": true, "all": true,
		"also": true, "and": true, "any": true, "because": true, "before": true,
		"between": true, "but": true, "by": true, "can": true, "for": true,
		"from": true, "how": true, "if": true, "in": true, "into": true,
		"it": true, "its": true, "just": true, "me": true, "more": true,
		"most": true, "no": true, "not": true, "of": true, "on": true,
		"or": true, "other": true, "out": true, "over": true, "own": true,
		"same": true, "so": true, "some": true, "such": true, "than": true,
		"that": true, "their": true, "them": true, "then": true, "there": true,
		"these": true, "they": true, "this": true, "those": true, "through": true,
		"to": true, "too": true, "under": true, "up": true, "very": true,
		"what": true, "when": true, "where": true, "which": true, "while": true,
		"who": true, "whom": true, "why": true, "with": true, "you": true,
		"your": true, "tell": true,
	}

	words := strings.Fields(strings.ToLower(question))
	var keywords []string
	for _, word := range words {
		word = strings.Trim(word, ".,?!;:\"'()[]{}")
		if word != "" && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}
	return keywords
}

func (s *WorldModelService) GetUserKnowledge(ctx context.Context, userID int64) (*KnowledgeSummary, error) {
	facts, err := s.repo.GetFactsByUser(ctx, userID, 0)
	if err != nil {
		return nil, err
	}

	entities, err := s.repo.GetEntitiesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	sessions, err := s.repo.GetRecentSessions(ctx, userID, 10)
	if err != nil {
		return nil, err
	}

	topEntities := entities
	if len(topEntities) > 10 {
		topEntities = topEntities[:10]
	}

	return &KnowledgeSummary{
		TotalFacts:     len(facts),
		TotalEntities:  len(entities),
		RecentSessions: sessions,
		TopEntities:    topEntities,
	}, nil
}

func (s *WorldModelService) buildExtractionPrompt(answer string, sources []search.SearchResult) string {
	var sourcesSection string
	if len(sources) > 0 {
		var builder strings.Builder
		builder.WriteString("Sources used:\n")
		for _, src := range sources {
			builder.WriteString(fmt.Sprintf("- %s: %s\n", src.Title, src.URL))
		}
		builder.WriteString("\n")
		sourcesSection = builder.String()
	}

	return fmt.Sprintf(ExtractionUserPromptTemplate, answer, sourcesSection)
}

func (s *WorldModelService) parseExtractionResponse(response string) (*extractionResponse, error) {
	// убираем markdown блоки если LLM их добавил
	cleaned := response
	if strings.HasPrefix(strings.TrimSpace(response), "```") {
		lines := strings.Split(response, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		cleaned = strings.Join(jsonLines, "\n")
	}

	var result extractionResponse
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *WorldModelService) saveFact(ctx context.Context, userID int64, sessionID string, f extractedFact) error {
	if strings.TrimSpace(f.Content) == "" {
		return nil
	}

	// дедупликация - проверяем есть ли уже такой факт
	existing, err := s.repo.FindFactByContent(ctx, userID, f.Content)
	if err == nil && existing != nil {
		return s.repo.AddFactToSession(ctx, sessionID, existing.ID)
	}

	fact := &domain.Fact{
		ID:          uuid.New().String(),
		UserID:      userID,
		Content:     f.Content,
		SourceURL:   f.SourceURL,
		Confidence:  f.Confidence,
		ExtractedAt: time.Now(),
	}

	if err := s.repo.CreateFact(ctx, fact); err != nil {
		return err
	}

	return s.repo.AddFactToSession(ctx, sessionID, fact.ID)
}

func (s *WorldModelService) saveEntity(ctx context.Context, userID int64, sessionID string, e extractedEntity) error {
	if strings.TrimSpace(e.Name) == "" {
		return nil
	}

	entityType := domain.EntityType(e.Type)
	if !entityType.IsValid() {
		s.logger.Debug("skipping entity with invalid type",
			zap.String("name", e.Name),
			zap.String("type", e.Type),
		)
		return nil
	}

	existing, err := s.repo.GetEntityByName(ctx, userID, e.Name)
	if err == nil && existing != nil {
		for k, v := range e.Attributes {
			existing.Attributes[k] = v
		}
		existing.LastSeenAt = time.Now()

		if err := s.repo.UpdateEntity(ctx, existing); err != nil {
			return err
		}
		return s.repo.AddEntityToSession(ctx, sessionID, existing.ID)
	}

	entity := &domain.Entity{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        e.Name,
		Type:        entityType,
		Attributes:  e.Attributes,
		FirstSeenAt: time.Now(),
		LastSeenAt:  time.Now(),
	}

	if entity.Attributes == nil {
		entity.Attributes = make(map[string]string)
	}

	if err := s.repo.CreateEntity(ctx, entity); err != nil {
		return err
	}

	return s.repo.AddEntityToSession(ctx, sessionID, entity.ID)
}
