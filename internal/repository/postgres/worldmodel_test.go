package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/repository"
)

func TestWorldModelRepo_CreateFact(t *testing.T) {
	repo := repository.NewMockWorldModelRepository()
	ctx := context.Background()

	fact := &domain.Fact{
		ID:          "fact-123",
		UserID:      1,
		Content:     "Klarna is a Swedish fintech company",
		SourceURL:   "https://example.com/klarna",
		Confidence:  0.95,
		ExtractedAt: time.Now(),
	}

	err := repo.CreateFact(ctx, fact)
	if err != nil {
		t.Fatalf("CreateFact failed: %v", err)
	}

	err = repo.CreateFact(ctx, fact)
	if err != domain.ErrDuplicateSource {
		t.Errorf("expected ErrDuplicateSource, got %v", err)
	}

	facts, err := repo.GetFactsByUser(ctx, 1, 10)
	if err != nil {
		t.Fatalf("GetFactsByUser failed: %v", err)
	}
	if len(facts) != 1 {
		t.Errorf("expected 1 fact, got %d", len(facts))
	}
	if facts[0].Content != fact.Content {
		t.Errorf("expected content %q, got %q", fact.Content, facts[0].Content)
	}
}

func TestWorldModelRepo_GetFactsBySession(t *testing.T) {
	repo := repository.NewMockWorldModelRepository()
	ctx := context.Background()

	fact1 := &domain.Fact{
		ID:      "fact-1",
		UserID:  1,
		Content: "Fact 1",
	}
	fact2 := &domain.Fact{
		ID:      "fact-2",
		UserID:  1,
		Content: "Fact 2",
	}

	_ = repo.CreateFact(ctx, fact1)
	_ = repo.CreateFact(ctx, fact2)

	session := &domain.ResearchSession{
		ID:       "session-1",
		UserID:   1,
		Question: "Test question",
	}
	_ = repo.CreateSession(ctx, session)

	err := repo.AddFactToSession(ctx, session.ID, fact1.ID)
	if err != nil {
		t.Fatalf("AddFactToSession failed: %v", err)
	}

	facts, err := repo.GetFactsBySession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetFactsBySession failed: %v", err)
	}
	if len(facts) != 1 {
		t.Errorf("expected 1 fact, got %d", len(facts))
	}
	if facts[0].ID != fact1.ID {
		t.Errorf("expected fact ID %q, got %q", fact1.ID, facts[0].ID)
	}
}

func TestWorldModelRepo_SearchFacts(t *testing.T) {
	repo := repository.NewMockWorldModelRepository()
	ctx := context.Background()

	fact1 := &domain.Fact{
		ID:      "fact-1",
		UserID:  1,
		Content: "Klarna is a fintech company from Sweden",
	}
	fact2 := &domain.Fact{
		ID:      "fact-2",
		UserID:  1,
		Content: "Apple produces iPhones",
	}
	fact3 := &domain.Fact{
		ID:      "fact-3",
		UserID:  2,
		Content: "Klarna was founded in 2005",
	}

	_ = repo.CreateFact(ctx, fact1)
	_ = repo.CreateFact(ctx, fact2)
	_ = repo.CreateFact(ctx, fact3)

	facts, err := repo.SearchFacts(ctx, 1, "klarna")
	if err != nil {
		t.Fatalf("SearchFacts failed: %v", err)
	}
	if len(facts) != 1 {
		t.Errorf("expected 1 fact, got %d", len(facts))
	}
	if facts[0].ID != fact1.ID {
		t.Errorf("expected fact ID %q, got %q", fact1.ID, facts[0].ID)
	}

	facts, err = repo.SearchFacts(ctx, 1, "fintech")
	if err != nil {
		t.Fatalf("SearchFacts failed: %v", err)
	}
	if len(facts) != 1 {
		t.Errorf("expected 1 fact, got %d", len(facts))
	}
}

func TestWorldModelRepo_FindFactByContent(t *testing.T) {
	repo := repository.NewMockWorldModelRepository()
	ctx := context.Background()

	content := "Exact content to find"
	fact := &domain.Fact{
		ID:      "fact-1",
		UserID:  1,
		Content: content,
	}
	_ = repo.CreateFact(ctx, fact)

	found, err := repo.FindFactByContent(ctx, 1, content)
	if err != nil {
		t.Fatalf("FindFactByContent failed: %v", err)
	}
	if found.ID != fact.ID {
		t.Errorf("expected fact ID %q, got %q", fact.ID, found.ID)
	}

	_, err = repo.FindFactByContent(ctx, 1, "non-existing content")
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	_, err = repo.FindFactByContent(ctx, 2, content)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound for wrong user, got %v", err)
	}
}

func TestWorldModelRepo_CreateEntity(t *testing.T) {
	repo := repository.NewMockWorldModelRepository()
	ctx := context.Background()

	entity := &domain.Entity{
		ID:     "entity-1",
		UserID: 1,
		Name:   "Klarna",
		Type:   domain.EntityCompany,
		Attributes: map[string]string{
			"country": "Sweden",
			"founded": "2005",
		},
	}

	err := repo.CreateEntity(ctx, entity)
	if err != nil {
		t.Fatalf("CreateEntity failed: %v", err)
	}

	entity2 := &domain.Entity{
		ID:     "entity-2",
		UserID: 1,
		Name:   "Klarna",
		Type:   domain.EntityCompany,
	}
	err = repo.CreateEntity(ctx, entity2)
	if err != domain.ErrDuplicateSource {
		t.Errorf("expected ErrDuplicateSource, got %v", err)
	}

	entity3 := &domain.Entity{
		ID:     "entity-3",
		UserID: 2,
		Name:   "Klarna",
		Type:   domain.EntityCompany,
	}
	err = repo.CreateEntity(ctx, entity3)
	if err != nil {
		t.Errorf("expected no error for different user, got %v", err)
	}

	found, err := repo.GetEntityByName(ctx, 1, "Klarna")
	if err != nil {
		t.Fatalf("GetEntityByName failed: %v", err)
	}
	if found.Attributes["country"] != "Sweden" {
		t.Errorf("expected country Sweden, got %q", found.Attributes["country"])
	}
	if found.Attributes["founded"] != "2005" {
		t.Errorf("expected founded 2005, got %q", found.Attributes["founded"])
	}
}

func TestWorldModelRepo_UpdateEntity(t *testing.T) {
	repo := repository.NewMockWorldModelRepository()
	ctx := context.Background()

	entity := &domain.Entity{
		ID:     "entity-1",
		UserID: 1,
		Name:   "Klarna",
		Type:   domain.EntityCompany,
		Attributes: map[string]string{
			"country": "Sweden",
		},
	}
	_ = repo.CreateEntity(ctx, entity)

	entity.Attributes["founded"] = "2005"
	entity.Attributes["employees"] = "5000"
	entity.LastSeenAt = time.Now()

	err := repo.UpdateEntity(ctx, entity)
	if err != nil {
		t.Fatalf("UpdateEntity failed: %v", err)
	}

	found, err := repo.GetEntityByName(ctx, 1, "Klarna")
	if err != nil {
		t.Fatalf("GetEntityByName failed: %v", err)
	}
	if found.Attributes["founded"] != "2005" {
		t.Errorf("expected founded 2005, got %q", found.Attributes["founded"])
	}
	if found.Attributes["employees"] != "5000" {
		t.Errorf("expected employees 5000, got %q", found.Attributes["employees"])
	}

	nonExisting := &domain.Entity{
		ID:     "non-existing",
		UserID: 1,
		Name:   "NonExisting",
		Type:   domain.EntityCompany,
	}
	err = repo.UpdateEntity(ctx, nonExisting)
	if err != domain.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestWorldModelRepo_CreateSession(t *testing.T) {
	repo := repository.NewMockWorldModelRepository()
	ctx := context.Background()

	session := &domain.ResearchSession{
		ID:       "session-1",
		UserID:   1,
		Question: "What is Klarna?",
		Strategy: "deep_research",
	}

	err := repo.CreateSession(ctx, session)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	fact := &domain.Fact{
		ID:      "fact-1",
		UserID:  1,
		Content: "Klarna is a fintech company",
	}
	entity := &domain.Entity{
		ID:     "entity-1",
		UserID: 1,
		Name:   "Klarna",
		Type:   domain.EntityCompany,
	}
	_ = repo.CreateFact(ctx, fact)
	_ = repo.CreateEntity(ctx, entity)

	err = repo.AddFactToSession(ctx, session.ID, fact.ID)
	if err != nil {
		t.Fatalf("AddFactToSession failed: %v", err)
	}
	err = repo.AddEntityToSession(ctx, session.ID, entity.ID)
	if err != nil {
		t.Fatalf("AddEntityToSession failed: %v", err)
	}

	sessions, err := repo.GetRecentSessions(ctx, 1, 10)
	if err != nil {
		t.Fatalf("GetRecentSessions failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Question != session.Question {
		t.Errorf("expected question %q, got %q", session.Question, sessions[0].Question)
	}

	facts, err := repo.GetFactsBySession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetFactsBySession failed: %v", err)
	}
	if len(facts) != 1 {
		t.Errorf("expected 1 fact in session, got %d", len(facts))
	}
}

func TestWorldModelRepo_GetEntitiesByUser(t *testing.T) {
	repo := repository.NewMockWorldModelRepository()
	ctx := context.Background()

	entity1 := &domain.Entity{
		ID:     "entity-1",
		UserID: 1,
		Name:   "Klarna",
		Type:   domain.EntityCompany,
	}
	entity2 := &domain.Entity{
		ID:     "entity-2",
		UserID: 1,
		Name:   "BNPL",
		Type:   domain.EntityConcept,
	}
	entity3 := &domain.Entity{
		ID:     "entity-3",
		UserID: 2,
		Name:   "Apple",
		Type:   domain.EntityCompany,
	}

	_ = repo.CreateEntity(ctx, entity1)
	_ = repo.CreateEntity(ctx, entity2)
	_ = repo.CreateEntity(ctx, entity3)

	entities, err := repo.GetEntitiesByUser(ctx, 1)
	if err != nil {
		t.Fatalf("GetEntitiesByUser failed: %v", err)
	}
	if len(entities) != 2 {
		t.Errorf("expected 2 entities, got %d", len(entities))
	}

	entities, err = repo.GetEntitiesByUser(ctx, 2)
	if err != nil {
		t.Fatalf("GetEntitiesByUser failed: %v", err)
	}
	if len(entities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(entities))
	}
}

func TestWorldModelRepo_GetFactsByUser_WithLimit(t *testing.T) {
	repo := repository.NewMockWorldModelRepository()
	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		fact := &domain.Fact{
			ID:          "fact-" + string(rune('0'+i)),
			UserID:      1,
			Content:     "Fact content",
			ExtractedAt: time.Now().Add(time.Duration(i) * time.Hour),
		}
		_ = repo.CreateFact(ctx, fact)
	}

	facts, err := repo.GetFactsByUser(ctx, 1, 3)
	if err != nil {
		t.Fatalf("GetFactsByUser failed: %v", err)
	}
	if len(facts) != 3 {
		t.Errorf("expected 3 facts, got %d", len(facts))
	}

	facts, err = repo.GetFactsByUser(ctx, 1, 10)
	if err != nil {
		t.Fatalf("GetFactsByUser failed: %v", err)
	}
	if len(facts) != 5 {
		t.Errorf("expected 5 facts, got %d", len(facts))
	}
}

func TestWorldModelRepo_AddFactToSession_Idempotent(t *testing.T) {
	repo := repository.NewMockWorldModelRepository()
	ctx := context.Background()

	session := &domain.ResearchSession{
		ID:       "session-1",
		UserID:   1,
		Question: "Test",
	}
	fact := &domain.Fact{
		ID:      "fact-1",
		UserID:  1,
		Content: "Test fact",
	}

	_ = repo.CreateSession(ctx, session)
	_ = repo.CreateFact(ctx, fact)

	err := repo.AddFactToSession(ctx, session.ID, fact.ID)
	if err != nil {
		t.Fatalf("First AddFactToSession failed: %v", err)
	}

	err = repo.AddFactToSession(ctx, session.ID, fact.ID)
	if err != nil {
		t.Fatalf("Second AddFactToSession failed: %v", err)
	}

	facts, err := repo.GetFactsBySession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetFactsBySession failed: %v", err)
	}
	if len(facts) != 1 {
		t.Errorf("expected 1 fact (idempotent), got %d", len(facts))
	}
}
