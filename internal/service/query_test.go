package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/cache/memory"
	"github.com/kitbuilder587/fintech-bot/internal/domain"
	llmMock "github.com/kitbuilder587/fintech-bot/internal/llm/mock"
	"github.com/kitbuilder587/fintech-bot/internal/repository"
	"github.com/kitbuilder587/fintech-bot/internal/search"
	searchMock "github.com/kitbuilder587/fintech-bot/internal/search/mock"
)

type MockCritic struct {
	Results     []*domain.CriticResult
	Errors      []error
	CallCount   int
	LastAnswer  string
	LastSources []search.SearchResult
}

func NewMockCritic() *MockCritic {
	return &MockCritic{
		Results: []*domain.CriticResult{},
		Errors:  []error{},
	}
}

func (m *MockCritic) WithApproved() *MockCritic {
	m.Results = append(m.Results, &domain.CriticResult{
		Approved:   true,
		Issues:     nil,
		Confidence: 0.95,
	})
	return m
}

func (m *MockCritic) WithRejected(issues []string) *MockCritic {
	m.Results = append(m.Results, &domain.CriticResult{
		Approved:    false,
		Issues:      issues,
		Suggestions: []string{"Please fix these issues"},
		Confidence:  0.7,
	})
	return m
}

func (m *MockCritic) WithError(err error) *MockCritic {
	m.Errors = append(m.Errors, err)
	return m
}

func (m *MockCritic) Review(ctx context.Context, answer string, sources []search.SearchResult, question string) (*domain.CriticResult, error) {
	m.LastAnswer = answer
	m.LastSources = sources
	callIdx := m.CallCount
	m.CallCount++

	if callIdx < len(m.Errors) && m.Errors[callIdx] != nil {
		return nil, m.Errors[callIdx]
	}
	if callIdx < len(m.Results) {
		return m.Results[callIdx], nil
	}
	return &domain.CriticResult{Approved: true, Confidence: 0.9}, nil
}

func TestQueryService_Process(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name    string
		req     *domain.QueryRequest
		setup   func(*repository.MockSourceRepository, *searchMock.Client, *llmMock.Client)
		wantErr error
	}{
		{"ok", &domain.QueryRequest{UserID: 1, Text: "What are fintech trends?", Strategy: domain.QuickStrategy()},
			func(sr *repository.MockSourceRepository, sc *searchMock.Client, lc *llmMock.Client) {
				sr.Create(context.Background(), &domain.Source{UserID: 1, URL: "https://mckinsey.com/fintech", Name: "McKinsey"})
				sc.Results = []search.SearchResult{{Title: "Fintech Trends", URL: "https://mckinsey.com/trends", Content: "Content about trends"}}
				lc.Response = `{"queries": ["fintech trends 2025"]}`
			}, nil},
		{"empty query", &domain.QueryRequest{UserID: 1, Text: ""}, func(sr *repository.MockSourceRepository, sc *searchMock.Client, lc *llmMock.Client) {}, domain.ErrEmptyQuery},
		{"no sources", &domain.QueryRequest{UserID: 1, Text: "Test query", Strategy: domain.QuickStrategy()}, func(sr *repository.MockSourceRepository, sc *searchMock.Client, lc *llmMock.Client) {}, domain.ErrNoSources},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceRepo := repository.NewMockSourceRepository()
			searchClient := searchMock.New()
			llmClient := llmMock.New()
			cacheClient := memory.New()

			tt.setup(sourceRepo, searchClient, llmClient)

			svc := NewQueryService(QueryServiceDeps{
				Sources: sourceRepo,
				LLM:     llmClient,
				Search:  searchClient,
				Cache:   cacheClient,
				Logger:  logger,
				Config: QueryConfig{
					MaxSearchQueries:   3,
					MaxResultsPerQuery: 5,
					CacheTTL:           time.Hour,
					SearchTimeout:      10 * time.Second,
				},
			})

			resp, err := svc.Process(context.Background(), tt.req)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Process() unexpected error = %v", err)
				return
			}

			if resp == nil {
				t.Error("Process() returned nil response")
			}
		})
	}
}

func TestQueryService_CacheHit(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	llmClient := llmMock.New()
	cacheClient := memory.New()

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://example.com",
		Name:   "Example",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://example.com/1", Content: "Content"},
	}

	svc := NewQueryService(QueryServiceDeps{
		Sources: sourceRepo,
		LLM:     llmClient,
		Search:  searchClient,
		Cache:   cacheClient,
		Logger:  logger,
	})

	_, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID:   1,
		Text:     "test query",
		Strategy: domain.QuickStrategy(),
	})
	if err != nil {
		t.Fatalf("First Process() error = %v", err)
	}

	searchClient.Error = search.ErrSearchFailed

	_, err = svc.Process(context.Background(), &domain.QueryRequest{
		UserID:   1,
		Text:     "test query",
		Strategy: domain.QuickStrategy(),
	})
	if err != nil {
		t.Errorf("Second Process() should use cache, got error = %v", err)
	}
}

func TestQueryService_CriticApproved(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	llmClient := llmMock.New()
	cacheClient := memory.New()
	mockCritic := NewMockCritic().WithApproved()

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://example.com",
		Name:   "Example",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://example.com/1", Content: "Content about fintech"},
	}
	llmClient.Response = `{"queries": ["fintech trends"]}`

	svc := NewQueryService(QueryServiceDeps{
		Sources: sourceRepo,
		LLM:     llmClient,
		Search:  searchClient,
		Cache:   cacheClient,
		Logger:  logger,
		Critic:  mockCritic,
		Config: QueryConfig{
			MaxSearchQueries:   3,
			MaxResultsPerQuery: 5,
			CacheTTL:           time.Hour,
			SearchTimeout:      10 * time.Second,
		},
	})

	strategy := domain.StandardStrategy()
	strategy.UseCritic = true

	resp, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID: 1, Text: "What are fintech trends?", Strategy: strategy,
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if mockCritic.CallCount != 1 {
		t.Errorf("Critic called %d times, want 1", mockCritic.CallCount)
	}
}

func TestQueryService_CriticRejectedThenApproved(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	cacheClient := memory.New()

	llmCallCount := 0
	llmClient := &trackingLLMClient{
		responses: []string{
			`{"queries": ["fintech trends"]}`,
			"Initial answer with some issues",
			"Improved answer after fixing issues [S1] citation",
		},
		callCount: &llmCallCount,
	}

	mockCritic := NewMockCritic().
		WithRejected([]string{"Missing source citations", "Answer too vague"}).
		WithApproved()

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://example.com",
		Name:   "Example",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://example.com/1", Content: "Content about fintech trends"},
	}

	svc := NewQueryService(QueryServiceDeps{
		Sources: sourceRepo,
		LLM:     llmClient,
		Search:  searchClient,
		Cache:   cacheClient,
		Logger:  logger,
		Critic:  mockCritic,
		CriticConfig: domain.CriticConfig{
			MaxRetries: 3,
			StrictMode: false,
		},
		Config: QueryConfig{
			MaxSearchQueries:   3,
			MaxResultsPerQuery: 5,
			CacheTTL:           time.Hour,
			SearchTimeout:      10 * time.Second,
		},
	})

	strategy := domain.StandardStrategy()
	strategy.UseCritic = true

	resp, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID:   1,
		Text:     "What are fintech trends?",
		Strategy: strategy,
	})

	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if mockCritic.CallCount != 2 {
		t.Errorf("Critic called %d times, want 2", mockCritic.CallCount)
	}
	if llmCallCount != 3 {
		t.Errorf("LLM called %d times, want 3", llmCallCount)
	}
}

func TestQueryService_CriticMaxRetries(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	cacheClient := memory.New()

	llmCallCount := 0
	llmClient := &trackingLLMClient{
		responses: []string{
			`{"queries": ["fintech trends"]}`,
			"Initial answer",
			"Improved answer 1",
			"Improved answer 2",
			"Improved answer 3",
		},
		callCount: &llmCallCount,
	}

	mockCritic := NewMockCritic().
		WithRejected([]string{"Issue 1"}).
		WithRejected([]string{"Issue 2"}).
		WithRejected([]string{"Issue 3"}).
		WithRejected([]string{"Issue 4"})

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://example.com",
		Name:   "Example",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://example.com/1", Content: "Content"},
	}

	svc := NewQueryService(QueryServiceDeps{
		Sources: sourceRepo,
		LLM:     llmClient,
		Search:  searchClient,
		Cache:   cacheClient,
		Logger:  logger,
		Critic:  mockCritic,
		CriticConfig: domain.CriticConfig{
			MaxRetries: 2,
			StrictMode: false,
		},
		Config: QueryConfig{
			MaxSearchQueries:   3,
			MaxResultsPerQuery: 5,
			CacheTTL:           time.Hour,
			SearchTimeout:      10 * time.Second,
		},
	})

	strategy := domain.StandardStrategy()
	strategy.UseCritic = true

	resp, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID:   1,
		Text:     "What are fintech trends?",
		Strategy: strategy,
	})

	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if mockCritic.CallCount != 3 {
		t.Errorf("Critic called %d times, want 3", mockCritic.CallCount)
	}
	if llmCallCount != 4 {
		t.Errorf("LLM called %d times, want 4", llmCallCount)
	}
}

func TestQueryService_NoCritic(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	llmClient := llmMock.New()
	cacheClient := memory.New()

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://example.com",
		Name:   "Example",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://example.com/1", Content: "Content"},
	}
	llmClient.Response = `{"queries": ["test query"]}`

	svc := NewQueryService(QueryServiceDeps{
		Sources: sourceRepo,
		LLM:     llmClient,
		Search:  searchClient,
		Cache:   cacheClient,
		Logger:  logger,
		Critic:  nil,
		Config: QueryConfig{
			MaxSearchQueries:   3,
			MaxResultsPerQuery: 5,
			CacheTTL:           time.Hour,
			SearchTimeout:      10 * time.Second,
		},
	})

	strategy := domain.StandardStrategy()
	strategy.UseCritic = true

	resp, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID: 1, Text: "Test query", Strategy: strategy,
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
}

func TestQueryService_CriticDisabled(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	llmClient := llmMock.New()
	cacheClient := memory.New()
	mockCritic := NewMockCritic().WithApproved()

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://example.com",
		Name:   "Example",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://example.com/1", Content: "Content"},
	}
	llmClient.Response = `{"queries": ["test query"]}`

	svc := NewQueryService(QueryServiceDeps{
		Sources: sourceRepo,
		LLM:     llmClient,
		Search:  searchClient,
		Cache:   cacheClient,
		Logger:  logger,
		Critic:  mockCritic,
		Config: QueryConfig{
			MaxSearchQueries:   3,
			MaxResultsPerQuery: 5,
			CacheTTL:           time.Hour,
			SearchTimeout:      10 * time.Second,
		},
	})

	strategy := domain.QuickStrategy()
	strategy.UseCritic = false

	_, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID: 1, Text: "Test query", Strategy: strategy,
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if mockCritic.CallCount != 0 {
		t.Errorf("Critic called %d times, want 0", mockCritic.CallCount)
	}
}

func TestQueryService_CriticError(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	llmClient := llmMock.New()
	cacheClient := memory.New()

	mockCritic := NewMockCritic().WithError(errors.New("critic service unavailable"))

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://example.com",
		Name:   "Example",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://example.com/1", Content: "Content"},
	}
	llmClient.Response = `{"queries": ["test query"]}`

	svc := NewQueryService(QueryServiceDeps{
		Sources: sourceRepo,
		LLM:     llmClient,
		Search:  searchClient,
		Cache:   cacheClient,
		Logger:  logger,
		Critic:  mockCritic,
		Config: QueryConfig{
			MaxSearchQueries:   3,
			MaxResultsPerQuery: 5,
			CacheTTL:           time.Hour,
			SearchTimeout:      10 * time.Second,
		},
	})

	strategy := domain.StandardStrategy()
	strategy.UseCritic = true

	resp, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID: 1, Text: "Test query", Strategy: strategy,
	})
	if err != nil {
		t.Fatalf("Process() should not fail on critic error, got error = %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if mockCritic.CallCount != 1 {
		t.Errorf("Critic called %d times, want 1", mockCritic.CallCount)
	}
}

type trackingLLMClient struct {
	responses []string
	callCount *int
}

func (c *trackingLLMClient) CompleteWithSystem(ctx context.Context, system, prompt string) (string, error) {
	idx := *c.callCount
	*c.callCount++
	if idx < len(c.responses) {
		return c.responses[idx], nil
	}
	return "default response", nil
}

// тест Quick стратегии
func TestQueryService_QuickStrategy(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	llmClient := llmMock.New()
	cacheClient := memory.New()
	mockCritic := NewMockCritic().WithApproved()

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://mckinsey.com/fintech",
		Name:   "McKinsey",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://mckinsey.com/1", Content: "Content"},
	}
	llmClient.Response = `{"queries": ["test query"]}`

	svc := NewQueryService(QueryServiceDeps{
		Sources: sourceRepo,
		LLM:     llmClient,
		Search:  searchClient,
		Cache:   cacheClient,
		Logger:  logger,
		Critic:  mockCritic,
	})

	req := &domain.QueryRequest{UserID: 1, Text: "Test query", Strategy: domain.QuickStrategy()}

	_, err := svc.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if mockCritic.CallCount != 0 {
		t.Errorf("Quick: critic called %d times, want 0", mockCritic.CallCount)
	}
	if searchClient.CallCount == 0 {
		t.Error("search not called")
	}
	if searchClient.LastRequest.MaxResults != 5 {
		t.Errorf("Quick maxResults = %d, want 5", searchClient.LastRequest.MaxResults)
	}
}

func TestQueryService_DeepStrategy(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	llmClient := llmMock.New()
	cacheClient := memory.New()
	mockCritic := NewMockCritic().WithApproved()

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://mckinsey.com/fintech",
		Name:   "McKinsey",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://mckinsey.com/1", Content: "Content"},
	}
	llmClient.Response = `{"queries": ["test query 1", "test query 2", "test query 3", "test query 4", "test query 5"]}`

	svc := NewQueryService(QueryServiceDeps{
		Sources: sourceRepo,
		LLM:     llmClient,
		Search:  searchClient,
		Cache:   cacheClient,
		Logger:  logger,
		Critic:  mockCritic,
	})

	req := &domain.QueryRequest{UserID: 1, Text: "Test deep query", Strategy: domain.DeepStrategy()}

	_, err := svc.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if mockCritic.CallCount == 0 {
		t.Error("Deep: critic should be called")
	}
	if searchClient.CallCount == 0 {
		t.Error("search not called")
	}
	if searchClient.LastRequest.MaxResults != 30 {
		t.Errorf("Deep maxResults = %d, want 30", searchClient.LastRequest.MaxResults)
	}
	if searchClient.CallCount < 5 {
		t.Errorf("Deep: search called %d times, want >=5", searchClient.CallCount)
	}
}

func TestQueryService_StrategyTimeout(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	llmClient := llmMock.New()
	cacheClient := memory.New()

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://example.com",
		Name:   "Example",
	})

	searchClient.Delay = 3 * time.Second
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://example.com/1", Content: "Content"},
	}
	llmClient.Response = `{"queries": ["test query"]}`

	svc := NewQueryService(QueryServiceDeps{
		Sources: sourceRepo,
		LLM:     llmClient,
		Search:  searchClient,
		Cache:   cacheClient,
		Logger:  logger,
	})

	shortTimeoutStrategy := domain.Strategy{
		Type:                  domain.StrategyQuick,
		MaxQueries:            1,
		MaxResults:            5,
		MaxAnalysisIterations: 1,
		UseCritic:             false,
		TimeoutSeconds:        1,
	}

	req := &domain.QueryRequest{UserID: 1, Text: "Test timeout", Strategy: shortTimeoutStrategy}
	_, err := svc.Process(context.Background(), req)

	if err == nil {
		t.Error("expected timeout error")
	}
	if err != context.DeadlineExceeded && !errors.Is(err, context.DeadlineExceeded) {
		if err.Error() != "context deadline exceeded" && !strings.Contains(err.Error(), "deadline") {
			t.Errorf("expected timeout, got: %v", err)
		}
	}
}

func TestQueryService_StandardStrategy(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	llmClient := llmMock.New()
	cacheClient := memory.New()
	mockCritic := NewMockCritic().WithApproved()

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://example.com",
		Name:   "Example",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://example.com/1", Content: "Content"},
	}
	llmClient.Response = `{"queries": ["test query 1", "test query 2", "test query 3"]}`

	svc := NewQueryService(QueryServiceDeps{
		Sources: sourceRepo,
		LLM:     llmClient,
		Search:  searchClient,
		Cache:   cacheClient,
		Logger:  logger,
		Critic:  mockCritic,
	})

	req := &domain.QueryRequest{UserID: 1, Text: "Test standard query", Strategy: domain.StandardStrategy()}

	_, err := svc.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if searchClient.LastRequest.MaxResults != 15 {
		t.Errorf("Standard maxResults = %d, want 15", searchClient.LastRequest.MaxResults)
	}
	if mockCritic.CallCount == 0 {
		t.Error("Standard: critic should be called")
	}
}

type MockWorldModel struct {
	GetRelevantContextFn   func(ctx context.Context, userID int64, question string) (string, error)
	ExtractAndStoreFn      func(ctx context.Context, userID int64, answer string, sources []search.SearchResult, question string, strategy domain.Strategy) error
	GetRelevantContextResp string
	GetRelevantContextErr  error
	ExtractAndStoreErr     error
	GetRelevantContextCall int
	ExtractAndStoreCall    int
	mu                     sync.Mutex
}

func (m *MockWorldModel) GetRelevantContext(ctx context.Context, userID int64, question string) (string, error) {
	m.GetRelevantContextCall++
	if m.GetRelevantContextFn != nil {
		return m.GetRelevantContextFn(ctx, userID, question)
	}
	return m.GetRelevantContextResp, m.GetRelevantContextErr
}

func (m *MockWorldModel) ExtractAndStore(ctx context.Context, userID int64, answer string, sources []search.SearchResult, question string, strategy domain.Strategy) error {
	m.mu.Lock()
	m.ExtractAndStoreCall++
	m.mu.Unlock()
	if m.ExtractAndStoreFn != nil {
		return m.ExtractAndStoreFn(ctx, userID, answer, sources, question, strategy)
	}
	return m.ExtractAndStoreErr
}

func (m *MockWorldModel) GetExtractAndStoreCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ExtractAndStoreCall
}

type MockCoordinator struct {
	ProcessFn      func(ctx context.Context, req AgentCoordinatorRequest) (*CoordinatorResponse, error)
	ProcessResp    *CoordinatorResponse
	ProcessErr     error
	ProcessCall    int
	LastRequest    AgentCoordinatorRequest
	AgentsUsedResp []string
}

func (m *MockCoordinator) Process(ctx context.Context, req AgentCoordinatorRequest) (*CoordinatorResponse, error) {
	m.ProcessCall++
	m.LastRequest = req
	if m.ProcessFn != nil {
		return m.ProcessFn(ctx, req)
	}
	return m.ProcessResp, m.ProcessErr
}

func TestQueryService_FullFlow(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	cacheClient := memory.New()
	mockCritic := NewMockCritic().WithApproved()

	llmCallCount := 0
	llmClient := &trackingLLMClient{
		responses: []string{
			`{"queries": ["fintech trends"]}`,
			"Initial answer from analyze if no coord",
		},
		callCount: &llmCallCount,
	}

	mockWorldModel := &MockWorldModel{
		GetRelevantContextResp: "Previous research: Klarna raised $1B in 2024.",
	}

	mockCoordinator := &MockCoordinator{
		ProcessResp: &CoordinatorResponse{
			FinalAnswer: "Multi-agent synthesized answer with context [S1]",
			AgentsUsed:  []string{"MarketAnalyst", "TechExpert"},
		},
	}

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://mckinsey.com/fintech",
		Name:   "McKinsey",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Fintech Trends", URL: "https://mckinsey.com/trends", Content: "Content about fintech trends"},
	}

	svc := NewQueryService(QueryServiceDeps{
		Sources:     sourceRepo,
		LLM:         llmClient,
		Search:      searchClient,
		Cache:       cacheClient,
		Logger:      logger,
		Critic:      mockCritic,
		WorldModel:  mockWorldModel,
		Coordinator: mockCoordinator,
		Config: QueryConfig{
			MaxSearchQueries:   3,
			MaxResultsPerQuery: 5,
			CacheTTL:           time.Hour,
			SearchTimeout:      10 * time.Second,
		},
	})

	strategy := domain.DeepStrategy()
	resp, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID: 1, Text: "What are the latest fintech trends?", Strategy: strategy,
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if mockWorldModel.GetRelevantContextCall == 0 {
		t.Error("WorldModel.GetRelevantContext not called")
	}
	if mockCoordinator.ProcessCall == 0 {
		t.Error("Coordinator.Process not called")
	}
	if resp.Text != "Multi-agent synthesized answer with context [S1]" {
		t.Errorf("Response = %q, want coordinator answer", resp.Text)
	}
	if mockCritic.CallCount == 0 {
		t.Error("Critic not called")
	}
	time.Sleep(100 * time.Millisecond)
	if mockWorldModel.GetExtractAndStoreCallCount() == 0 {
		t.Error("ExtractAndStore not called")
	}
}

func TestQueryService_NoWorldModel(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	llmClient := llmMock.New()
	cacheClient := memory.New()

	mockCoordinator := &MockCoordinator{
		ProcessResp: &CoordinatorResponse{
			FinalAnswer: "Answer from coordinator without world model context",
			AgentsUsed:  []string{"MarketAnalyst"},
		},
	}

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://example.com",
		Name:   "Example",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://example.com/1", Content: "Content"},
	}
	llmClient.Response = `{"queries": ["test query"]}`

	svc := NewQueryService(QueryServiceDeps{
		Sources:     sourceRepo,
		LLM:         llmClient,
		Search:      searchClient,
		Cache:       cacheClient,
		Logger:      logger,
		WorldModel:  nil,
		Coordinator: mockCoordinator,
		Config: QueryConfig{
			MaxSearchQueries:   3,
			MaxResultsPerQuery: 5,
			CacheTTL:           time.Hour,
			SearchTimeout:      10 * time.Second,
		},
	})

	resp, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID: 1, Text: "Test query", Strategy: domain.QuickStrategy(),
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if mockCoordinator.ProcessCall == 0 {
		t.Error("Coordinator not called")
	}
	if resp.Text != "Answer from coordinator without world model context" {
		t.Errorf("Response = %q, want coordinator answer", resp.Text)
	}
}

func TestQueryService_NoCoordinator(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	cacheClient := memory.New()

	llmCallCount := 0
	llmClient := &trackingLLMClient{
		responses: []string{`{"queries": ["fintech trends"]}`, "Answer from LLM analyze fallback"},
		callCount: &llmCallCount,
	}

	mockWorldModel := &MockWorldModel{GetRelevantContextResp: "Previous context data"}

	sourceRepo.Create(context.Background(), &domain.Source{UserID: 1, URL: "https://example.com", Name: "Example"})
	searchClient.Results = []search.SearchResult{{Title: "Test", URL: "https://example.com/1", Content: "Content"}}

	svc := NewQueryService(QueryServiceDeps{
		Sources:     sourceRepo,
		LLM:         llmClient,
		Search:      searchClient,
		Cache:       cacheClient,
		Logger:      logger,
		WorldModel:  mockWorldModel,
		Coordinator: nil,
		Config: QueryConfig{
			MaxSearchQueries:   3,
			MaxResultsPerQuery: 5,
			CacheTTL:           time.Hour,
			SearchTimeout:      10 * time.Second,
		},
	})

	resp, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID: 1, Text: "Test query", Strategy: domain.QuickStrategy(),
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if mockWorldModel.GetRelevantContextCall == 0 {
		t.Error("WorldModel.GetRelevantContext not called")
	}
	if llmCallCount != 2 {
		t.Errorf("LLM called %d times, want 2", llmCallCount)
	}
	if resp.Text != "Answer from LLM analyze fallback" {
		t.Errorf("Response = %q, want fallback", resp.Text)
	}
	time.Sleep(100 * time.Millisecond)
	if mockWorldModel.GetExtractAndStoreCallCount() == 0 {
		t.Error("ExtractAndStore not called")
	}
}

func TestQueryService_QuickSingleAgent(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	llmClient := llmMock.New()
	cacheClient := memory.New()
	mockCritic := NewMockCritic().WithApproved()

	mockCoordinator := &MockCoordinator{
		ProcessResp: &CoordinatorResponse{FinalAnswer: "Quick single-agent answer", AgentsUsed: []string{"MarketAnalyst"}},
	}

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://example.com",
		Name:   "Example",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://example.com/1", Content: "Content"},
	}
	llmClient.Response = `{"queries": ["test query"]}`

	svc := NewQueryService(QueryServiceDeps{
		Sources:     sourceRepo,
		LLM:         llmClient,
		Search:      searchClient,
		Cache:       cacheClient,
		Logger:      logger,
		Critic:      mockCritic,
		Coordinator: mockCoordinator,
		Config: QueryConfig{
			MaxSearchQueries:   3,
			MaxResultsPerQuery: 5,
			CacheTTL:           time.Hour,
			SearchTimeout:      10 * time.Second,
		},
	})

	strategy := domain.QuickStrategy()
	resp, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID: 1, Text: "Quick test query", Strategy: strategy,
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if mockCritic.CallCount != 0 {
		t.Errorf("Quick: critic called %d times, want 0", mockCritic.CallCount)
	}
	if mockCoordinator.ProcessCall == 0 {
		t.Error("Coordinator not called")
	}
	if resp.Text != "Quick single-agent answer" {
		t.Errorf("Response = %q, want quick answer", resp.Text)
	}
}

func TestQueryService_FollowUpQuestion(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	llmClient := llmMock.New()
	cacheClient := memory.New()

	var capturedContexts []string
	mockCoordinator := &MockCoordinator{
		ProcessFn: func(ctx context.Context, req AgentCoordinatorRequest) (*CoordinatorResponse, error) {
			return &CoordinatorResponse{FinalAnswer: "Answer with context", AgentsUsed: []string{"TechExpert"}}, nil
		},
	}

	callNum := 0
	mockWorldModel := &MockWorldModel{
		GetRelevantContextFn: func(ctx context.Context, userID int64, question string) (string, error) {
			callNum++
			capturedContexts = append(capturedContexts, question)
			if callNum == 1 {
				return "", nil
			}
			return "Previous research found: AI payments increased 40%.", nil
		},
	}

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://example.com",
		Name:   "Example",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "AI in Fintech", URL: "https://example.com/ai", Content: "AI is transforming payments"},
	}
	llmClient.Response = `{"queries": ["AI fintech"]}`

	svc := NewQueryService(QueryServiceDeps{
		Sources:     sourceRepo,
		LLM:         llmClient,
		Search:      searchClient,
		Cache:       cacheClient,
		Logger:      logger,
		WorldModel:  mockWorldModel,
		Coordinator: mockCoordinator,
		Config: QueryConfig{
			MaxSearchQueries:   3,
			MaxResultsPerQuery: 5,
			CacheTTL:           time.Hour,
			SearchTimeout:      10 * time.Second,
		},
	})

	_, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID: 1, Text: "How is AI used in fintech?", Strategy: domain.QuickStrategy(),
	})
	if err != nil {
		t.Fatalf("First Process() error = %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	resp, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID: 1, Text: "Tell me more about AI payments", Strategy: domain.QuickStrategy(),
	})
	if err != nil {
		t.Fatalf("Follow-up Process() error = %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if mockWorldModel.GetRelevantContextCall != 2 {
		t.Errorf("WorldModel called %d times, want 2", mockWorldModel.GetRelevantContextCall)
	}
	if len(capturedContexts) != 2 {
		t.Errorf("Captured %d contexts, want 2", len(capturedContexts))
	}
	if len(capturedContexts) >= 2 && capturedContexts[1] != "Tell me more about AI payments" {
		t.Errorf("Second question = %q, want 'Tell me more about AI payments'", capturedContexts[1])
	}
}

func TestQueryService_CoordinatorEmpty(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	cacheClient := memory.New()

	llmCallCount := 0
	llmClient := &trackingLLMClient{
		responses: []string{`{"queries": ["fintech trends"]}`, "Fallback answer when coordinator returns empty"},
		callCount: &llmCallCount,
	}

	mockCoordinator := &MockCoordinator{
		ProcessResp: &CoordinatorResponse{FinalAnswer: "", AgentsUsed: []string{}},
	}

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://example.com",
		Name:   "Example",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://example.com/1", Content: "Content"},
	}

	svc := NewQueryService(QueryServiceDeps{
		Sources:     sourceRepo,
		LLM:         llmClient,
		Search:      searchClient,
		Cache:       cacheClient,
		Logger:      logger,
		Coordinator: mockCoordinator,
		Config: QueryConfig{
			MaxSearchQueries:   3,
			MaxResultsPerQuery: 5,
			CacheTTL:           time.Hour,
			SearchTimeout:      10 * time.Second,
		},
	})

	resp, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID: 1, Text: "Test query", Strategy: domain.QuickStrategy(),
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if resp.Text != "Fallback answer when coordinator returns empty" {
		t.Errorf("Response = %q, want fallback", resp.Text)
	}
	if llmCallCount != 2 {
		t.Errorf("LLM called %d times, want 2", llmCallCount)
	}
}

func TestQueryService_CoordinatorError(t *testing.T) {
	logger := zap.NewNop()

	sourceRepo := repository.NewMockSourceRepository()
	searchClient := searchMock.New()
	cacheClient := memory.New()

	llmCallCount := 0
	llmClient := &trackingLLMClient{
		responses: []string{`{"queries": ["fintech trends"]}`, "Fallback answer after coordinator error"},
		callCount: &llmCallCount,
	}

	mockCoordinator := &MockCoordinator{ProcessErr: errors.New("coordinator processing failed")}

	sourceRepo.Create(context.Background(), &domain.Source{
		UserID: 1,
		URL:    "https://example.com",
		Name:   "Example",
	})
	searchClient.Results = []search.SearchResult{
		{Title: "Test", URL: "https://example.com/1", Content: "Content"},
	}

	svc := NewQueryService(QueryServiceDeps{
		Sources:     sourceRepo,
		LLM:         llmClient,
		Search:      searchClient,
		Cache:       cacheClient,
		Logger:      logger,
		Coordinator: mockCoordinator,
		Config: QueryConfig{
			MaxSearchQueries:   3,
			MaxResultsPerQuery: 5,
			CacheTTL:           time.Hour,
			SearchTimeout:      10 * time.Second,
		},
	})

	resp, err := svc.Process(context.Background(), &domain.QueryRequest{
		UserID: 1, Text: "Test query", Strategy: domain.QuickStrategy(),
	})
	if err != nil {
		t.Fatalf("Process() should not fail on Coordinator error, got error = %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if resp.Text != "Fallback answer after coordinator error" {
		t.Errorf("Response = %q, want fallback", resp.Text)
	}
}
