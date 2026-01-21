package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/llm/mock"
	"github.com/kitbuilder587/fintech-bot/internal/repository"
	"github.com/kitbuilder587/fintech-bot/internal/search"
)

func TestWorldModelService_ExtractAndStore(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	t.Run("ok", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		mockLLM := mock.New().WithResponse(`{
			"facts": [
				{"content": "Klarna was founded in 2005", "source_url": "https://example.com/klarna", "confidence": 0.9},
				{"content": "Klarna is a Swedish fintech company", "source_url": "https://example.com/klarna2", "confidence": 0.85}
			],
			"entities": [
				{"name": "Klarna", "type": "company", "attributes": {"founded": "2005", "country": "Sweden"}},
				{"name": "BNPL", "type": "concept", "attributes": {"description": "Buy Now Pay Later"}}
			]
		}`)

		svc := NewWorldModelService(repo, mockLLM, logger)

		sources := []search.SearchResult{
			{Title: "Klarna Info", URL: "https://example.com/klarna", Content: "Klarna founded in 2005"},
		}
		strategy := domain.StandardStrategy()

		err := svc.ExtractAndStore(ctx, 1, "Klarna is a fintech company", sources, "What is Klarna?", strategy)
		require.NoError(t, err)

		facts, err := repo.GetFactsByUser(ctx, 1, 10)
		require.NoError(t, err)
		assert.Len(t, facts, 2)

		entities, err := repo.GetEntitiesByUser(ctx, 1)
		require.NoError(t, err)
		assert.Len(t, entities, 2)

		sessions, err := repo.GetRecentSessions(ctx, 1, 10)
		require.NoError(t, err)
		assert.Len(t, sessions, 1)
		assert.Equal(t, "What is Klarna?", sessions[0].Question)
		assert.Equal(t, strategy.Type.String(), sessions[0].Strategy)
		assert.Equal(t, 1, mockLLM.CallCount)
		assert.Contains(t, mockLLM.LastPrompt, "Klarna is a fintech company")
	})

	t.Run("dedup facts", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		existingFact := &domain.Fact{
			ID:         "existing-fact-id",
			UserID:     1,
			Content:    "Klarna was founded in 2005",
			SourceURL:  "https://old.example.com",
			Confidence: 0.8,
		}
		err := repo.CreateFact(ctx, existingFact)
		require.NoError(t, err)

		mockLLM := mock.New().WithResponse(`{
			"facts": [
				{"content": "Klarna was founded in 2005", "source_url": "https://new.example.com", "confidence": 0.9},
				{"content": "New fact about Klarna", "source_url": "https://example.com", "confidence": 0.85}
			],
			"entities": []
		}`)

		svc := NewWorldModelService(repo, mockLLM, logger)
		strategy := domain.QuickStrategy()

		err = svc.ExtractAndStore(ctx, 1, "Answer about Klarna", nil, "Question?", strategy)
		require.NoError(t, err)

		facts, err := repo.GetFactsByUser(ctx, 1, 10)
		require.NoError(t, err)
		assert.Len(t, facts, 2) // 1 existing + 1 new
	})

	t.Run("update entity", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		existingEntity := &domain.Entity{
			ID:         "existing-entity-id",
			UserID:     1,
			Name:       "Klarna",
			Type:       domain.EntityCompany,
			Attributes: map[string]string{"founded": "2005"},
		}
		err := repo.CreateEntity(ctx, existingEntity)
		require.NoError(t, err)

		mockLLM := mock.New().WithResponse(`{
			"facts": [],
			"entities": [
				{"name": "Klarna", "type": "company", "attributes": {"founded": "2005", "ceo": "Sebastian Siemiatkowski"}}
			]
		}`)

		svc := NewWorldModelService(repo, mockLLM, logger)
		strategy := domain.QuickStrategy()

		err = svc.ExtractAndStore(ctx, 1, "Answer", nil, "Question?", strategy)
		require.NoError(t, err)

		entity, err := repo.GetEntityByName(ctx, 1, "Klarna")
		require.NoError(t, err)
		assert.Equal(t, "Sebastian Siemiatkowski", entity.Attributes["ceo"])
		assert.Equal(t, "2005", entity.Attributes["founded"])
	})

	t.Run("llm error", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		mockLLM := mock.New().WithError(errors.New("LLM unavailable"))

		svc := NewWorldModelService(repo, mockLLM, logger)
		strategy := domain.QuickStrategy()

		err := svc.ExtractAndStore(ctx, 1, "Answer", nil, "Question?", strategy)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "LLM")
	})
}

// тесты обработки разных JSON ответов
func TestWorldModelService_ExtractAndStore_MalformedJSON(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	t.Run("malformed json", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		mockLLM := mock.New().WithResponse(`{invalid json`)

		svc := NewWorldModelService(repo, mockLLM, logger)
		strategy := domain.QuickStrategy()

		err := svc.ExtractAndStore(ctx, 1, "Answer", nil, "Question?", strategy)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parse")
	})

	t.Run("empty response", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		mockLLM := mock.New().WithResponse(`{"facts": [], "entities": []}`)

		svc := NewWorldModelService(repo, mockLLM, logger)
		strategy := domain.QuickStrategy()

		err := svc.ExtractAndStore(ctx, 1, "Answer", nil, "Question?", strategy)
		assert.NoError(t, err)

		sessions, err := repo.GetRecentSessions(ctx, 1, 10)
		require.NoError(t, err)
		assert.Len(t, sessions, 1)
	})

	t.Run("markdown code block", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		mockLLM := mock.New().WithResponse("```json\n{\"facts\": [{\"content\": \"Test fact\", \"source_url\": \"\", \"confidence\": 0.9}], \"entities\": []}\n```")

		svc := NewWorldModelService(repo, mockLLM, logger)
		strategy := domain.QuickStrategy()

		err := svc.ExtractAndStore(ctx, 1, "Answer", nil, "Question?", strategy)
		require.NoError(t, err)

		facts, err := repo.GetFactsByUser(ctx, 1, 10)
		require.NoError(t, err)
		assert.Len(t, facts, 1)
	})

	t.Run("skip invalid entity", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		mockLLM := mock.New().WithResponse(`{
			"facts": [],
			"entities": [
				{"name": "ValidEntity", "type": "company", "attributes": {}},
				{"name": "InvalidEntity", "type": "unknown_type", "attributes": {}}
			]
		}`)

		svc := NewWorldModelService(repo, mockLLM, logger)
		strategy := domain.QuickStrategy()

		err := svc.ExtractAndStore(ctx, 1, "Answer", nil, "Question?", strategy)
		require.NoError(t, err)

		entities, err := repo.GetEntitiesByUser(ctx, 1)
		require.NoError(t, err)
		assert.Len(t, entities, 1)
		assert.Equal(t, "ValidEntity", entities[0].Name)
	})
}

func TestWorldModelService_GetRelevantContext(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	t.Run("ok", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		facts := []*domain.Fact{
			{ID: "1", UserID: 1, Content: "Klarna was founded in 2005", SourceURL: "https://example.com", Confidence: 0.9},
			{ID: "2", UserID: 1, Content: "Klarna offers BNPL services", SourceURL: "https://example.com", Confidence: 0.85},
			{ID: "3", UserID: 1, Content: "Bitcoin is a cryptocurrency", SourceURL: "https://crypto.com", Confidence: 0.8},
		}
		for _, f := range facts {
			err := repo.CreateFact(ctx, f)
			require.NoError(t, err)
		}

		mockLLM := mock.New()
		svc := NewWorldModelService(repo, mockLLM, logger)

		context, err := svc.GetRelevantContext(ctx, 1, "Tell me about Klarna")
		require.NoError(t, err)

		assert.Contains(t, context, "Klarna was founded in 2005")
		assert.Contains(t, context, "Klarna offers BNPL services")
		assert.NotContains(t, context, "Bitcoin")
	})

	t.Run("limit size", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		for i := 0; i < 50; i++ {
			fact := &domain.Fact{
				ID:         fmt.Sprintf("fact-%d", i),
				UserID:     1,
				Content:    fmt.Sprintf("This is a long fact number %d about Klarna with lots of detailed information that will take up space", i),
				SourceURL:  "https://example.com",
				Confidence: 0.9,
			}
			err := repo.CreateFact(ctx, fact)
			require.NoError(t, err)
		}

		mockLLM := mock.New()
		svc := NewWorldModelService(repo, mockLLM, logger)

		context, err := svc.GetRelevantContext(ctx, 1, "Tell me about Klarna")
		require.NoError(t, err)
		assert.LessOrEqual(t, len(context), MaxContextSize)
	})

	t.Run("no relevant facts", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		fact := &domain.Fact{
			ID:         "1",
			UserID:     1,
			Content:    "Apple makes iPhones",
			SourceURL:  "https://apple.com",
			Confidence: 0.9,
		}
		err := repo.CreateFact(ctx, fact)
		require.NoError(t, err)

		mockLLM := mock.New()
		svc := NewWorldModelService(repo, mockLLM, logger)

		context, err := svc.GetRelevantContext(ctx, 1, "Tell me about Klarna")
		require.NoError(t, err)
		assert.Empty(t, context)
	})
}

func TestWorldModelService_GetRelevantContext_NoData(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	t.Run("new user", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		mockLLM := mock.New()
		svc := NewWorldModelService(repo, mockLLM, logger)

		context, err := svc.GetRelevantContext(ctx, 999, "Any question")
		require.NoError(t, err)
		assert.Empty(t, context)
	})

	t.Run("empty question", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		mockLLM := mock.New()
		svc := NewWorldModelService(repo, mockLLM, logger)

		context, err := svc.GetRelevantContext(ctx, 1, "")
		require.NoError(t, err)
		assert.Empty(t, context)
	})
}

func TestWorldModelService_GetUserKnowledge(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	t.Run("ok", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		for i := 0; i < 5; i++ {
			fact := &domain.Fact{
				ID:         fmt.Sprintf("fact-%d", i),
				UserID:     1,
				Content:    fmt.Sprintf("Fact %d", i),
				Confidence: 0.9,
			}
			err := repo.CreateFact(ctx, fact)
			require.NoError(t, err)
		}

		entities := []*domain.Entity{
			{ID: "e1", UserID: 1, Name: "Klarna", Type: domain.EntityCompany, Attributes: map[string]string{"mentions": "10"}},
			{ID: "e2", UserID: 1, Name: "BNPL", Type: domain.EntityConcept, Attributes: map[string]string{"mentions": "5"}},
			{ID: "e3", UserID: 1, Name: "Sweden", Type: domain.EntityMarket, Attributes: map[string]string{"mentions": "3"}},
		}
		for _, e := range entities {
			err := repo.CreateEntity(ctx, e)
			require.NoError(t, err)
		}
		for i := 0; i < 3; i++ {
			session := &domain.ResearchSession{
				ID:       fmt.Sprintf("session-%d", i),
				UserID:   1,
				Question: fmt.Sprintf("Question %d", i),
				Strategy: "standard",
			}
			err := repo.CreateSession(ctx, session)
			require.NoError(t, err)
		}

		mockLLM := mock.New()
		svc := NewWorldModelService(repo, mockLLM, logger)

		summary, err := svc.GetUserKnowledge(ctx, 1)
		require.NoError(t, err)

		assert.Equal(t, 5, summary.TotalFacts)
		assert.Equal(t, 3, summary.TotalEntities)
		assert.Len(t, summary.RecentSessions, 3)
		assert.NotEmpty(t, summary.TopEntities)
	})

	t.Run("new user", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		mockLLM := mock.New()
		svc := NewWorldModelService(repo, mockLLM, logger)

		summary, err := svc.GetUserKnowledge(ctx, 999)
		require.NoError(t, err)

		assert.Equal(t, 0, summary.TotalFacts)
		assert.Equal(t, 0, summary.TotalEntities)
		assert.Empty(t, summary.RecentSessions)
		assert.Empty(t, summary.TopEntities)
	})

	t.Run("limit sessions", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		for i := 0; i < 20; i++ {
			session := &domain.ResearchSession{
				ID:       fmt.Sprintf("session-%d", i),
				UserID:   1,
				Question: fmt.Sprintf("Question %d", i),
				Strategy: "standard",
			}
			err := repo.CreateSession(ctx, session)
			require.NoError(t, err)
		}

		mockLLM := mock.New()
		svc := NewWorldModelService(repo, mockLLM, logger)

		summary, err := svc.GetUserKnowledge(ctx, 1)
		require.NoError(t, err)

		assert.LessOrEqual(t, len(summary.RecentSessions), 10)
	})

	t.Run("limit entities", func(t *testing.T) {
		repo := repository.NewMockWorldModelRepository()
		for i := 0; i < 20; i++ {
			entity := &domain.Entity{
				ID:         fmt.Sprintf("entity-%d", i),
				UserID:     1,
				Name:       fmt.Sprintf("Entity %d", i),
				Type:       domain.EntityCompany,
				Attributes: map[string]string{},
			}
			err := repo.CreateEntity(ctx, entity)
			require.NoError(t, err)
		}

		mockLLM := mock.New()
		svc := NewWorldModelService(repo, mockLLM, logger)

		summary, err := svc.GetUserKnowledge(ctx, 1)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(summary.TopEntities), 10)
	})
}
