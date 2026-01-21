package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/kitbuilder587/fintech-bot/internal/cache"
	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/llm"
	"github.com/kitbuilder587/fintech-bot/internal/metrics"
	"github.com/kitbuilder587/fintech-bot/internal/repository"
	"github.com/kitbuilder587/fintech-bot/internal/search"
)

type Critic interface {
	Review(ctx context.Context, answer string, sources []search.SearchResult, question string) (*domain.CriticResult, error)
}

type WorldModel interface {
	GetRelevantContext(ctx context.Context, userID int64, question string) (string, error)
	ExtractAndStore(ctx context.Context, userID int64, answer string, sources []search.SearchResult, question string, strategy domain.Strategy) error
}

type CoordinatorResponse struct {
	FinalAnswer string
	AgentsUsed  []string
}

type AgentCoordinator interface {
	Process(ctx context.Context, req AgentCoordinatorRequest) (*CoordinatorResponse, error)
}

type AgentCoordinatorRequest struct {
	Question      string
	SearchResults []search.SearchResult
	Context       string
	Strategy      domain.Strategy
}

type QueryService interface {
	Process(ctx context.Context, req *domain.QueryRequest) (*domain.QueryResponse, error)
}

type QueryConfig struct {
	MaxSearchQueries   int
	MaxResultsPerQuery int
	CacheTTL           time.Duration
	SearchTimeout      time.Duration
}

// QueryServiceDeps - зависимости для QueryService.
// TODO(perf): может переделать на functional options? deps struct громоздкий
type QueryServiceDeps struct {
	Sources repository.SourceRepository
	LLM     llm.Client
	Search  search.SearchClient
	Cache   cache.Cache
	Logger  *zap.Logger
	Metrics *metrics.Metrics
	Config  QueryConfig

	// опциональные компоненты
	Critic       Critic
	CriticConfig domain.CriticConfig
	WorldModel   WorldModel
	Coordinator  AgentCoordinator
}

type queryService struct {
	sources      repository.SourceRepository
	llm          llm.Client
	search       search.SearchClient
	cache        cache.Cache
	logger       *zap.Logger
	metrics      *metrics.Metrics
	config       QueryConfig
	critic       Critic
	criticConfig domain.CriticConfig

	worldModel  WorldModel
	coordinator AgentCoordinator
}

func NewQueryService(deps QueryServiceDeps) QueryService {
	if deps.Config.MaxSearchQueries == 0 {
		deps.Config.MaxSearchQueries = 3
	}
	if deps.Config.MaxResultsPerQuery == 0 {
		deps.Config.MaxResultsPerQuery = 5
	}
	if deps.Config.CacheTTL == 0 {
		deps.Config.CacheTTL = time.Hour
	}
	if deps.Config.SearchTimeout == 0 {
		deps.Config.SearchTimeout = 30 * time.Second
	}

	if deps.CriticConfig.MaxRetries == 0 {
		deps.CriticConfig.MaxRetries = 2
	}

	return &queryService{
		sources:      deps.Sources,
		llm:          deps.LLM,
		search:       deps.Search,
		cache:        deps.Cache,
		logger:       deps.Logger,
		metrics:      deps.Metrics,
		config:       deps.Config,
		critic:       deps.Critic,
		criticConfig: deps.CriticConfig,
		worldModel:   deps.WorldModel,
		coordinator:  deps.Coordinator,
	}
}

func (s *queryService) Process(ctx context.Context, req *domain.QueryRequest) (*domain.QueryResponse, error) {
	startTime := time.Now()

	if s.metrics != nil {
		s.metrics.IncRequestsInFlight()
		defer s.metrics.DecRequestsInFlight()
	}

	if err := req.Validate(); err != nil {
		if s.metrics != nil {
			s.metrics.RecordRequest("query", "validation_error", time.Since(startTime))
		}
		return nil, err
	}
	req.Sanitize()

	if req.Strategy.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.Strategy.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	s.logger.Info("processing query",
		zap.Int64("user_id", req.UserID),
		zap.Int("query_length", len(req.Text)),
		zap.String("strategy_type", string(req.Strategy.Type)),
		zap.Int("strategy_max_queries", req.Strategy.MaxQueries),
		zap.Int("strategy_max_results", req.Strategy.MaxResults),
		zap.Bool("strategy_use_critic", req.Strategy.UseCritic),
	)

	var worldContext string
	if s.worldModel != nil {
		worldContext, _ = s.worldModel.GetRelevantContext(ctx, req.UserID, req.Text)
		if worldContext != "" {
			s.logger.Debug("using world model context",
				zap.Int64("user_id", req.UserID),
				zap.Int("context_length", len(worldContext)),
			)
		}
	}

	userSources, err := s.sources.ListByUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	if len(userSources) == 0 {
		return nil, domain.ErrNoSources
	}

	domains := make([]string, 0, len(userSources))
	trustMap := make(map[string]domain.TrustLevel)
	for _, src := range userSources {
		d := src.Domain()
		if d != "" {
			domains = append(domains, d)
			trustMap[d] = src.TrustLevel
		}
	}

	// расширяем запрос через LLM
	maxQueries := req.Strategy.MaxQueries
	if maxQueries <= 0 {
		maxQueries = 3
	}
	searchQueries, err := s.expandQuery(ctx, req.Text, maxQueries)
	if err != nil {
		s.logger.Warn("query expansion failed, using original", zap.Error(err))
		searchQueries = []string{req.Text}
	}

	// ищем параллельно с кешированием
	maxResults := req.Strategy.MaxResults
	if maxResults <= 0 {
		maxResults = 15
	}
	results, err := s.searchWithCache(ctx, searchQueries, domains, maxResults)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if len(results) == 0 {
		return nil, domain.ErrNoResults
	}

	// мультиагентный анализ (если настроен координатор)
	var answer string
	if s.coordinator != nil {
		coordResp, coordErr := s.coordinator.Process(ctx, AgentCoordinatorRequest{
			Question:      req.Text,
			SearchResults: results,
			Context:       worldContext,
			Strategy:      req.Strategy,
		})
		if coordErr != nil {
			s.logger.Warn("coordinator processing failed, falling back to analyze",
				zap.Error(coordErr),
			)
		} else if coordResp != nil && coordResp.FinalAnswer != "" {
			answer = coordResp.FinalAnswer
			s.logger.Debug("using coordinator answer",
				zap.Int("agents_used", len(coordResp.AgentsUsed)),
			)
		}
	}

	// fallback если координатор не вернул ответ
	if answer == "" {
		var err error
		answer, err = s.analyze(ctx, req.Text, results)
		if err != nil {
			return nil, err
		}
	}

	// критик проверяет ответ (опционально)
	if s.critic != nil && req.Strategy.UseCritic {
		answer = s.reviewWithCritic(ctx, answer, results, req.Text)
	}

	response := &domain.QueryResponse{
		Text:    answer,
		Sources: s.toSourceRefs(results, trustMap),
	}

	s.logger.Info("query processed",
		zap.Int64("user_id", req.UserID),
		zap.Int("sources_used", len(results)),
	)

	if s.metrics != nil {
		s.metrics.RecordRequest("query", "success", time.Since(startTime))
	}

	// в фоне сохраняем в world model
	if s.worldModel != nil {
		go func() {
			if err := s.worldModel.ExtractAndStore(context.Background(), req.UserID, answer, results, req.Text, req.Strategy); err != nil {
				s.logger.Warn("failed to save to world model",
					zap.Error(err),
					zap.Int64("user_id", req.UserID),
				)
			}
		}()
	}

	return response, nil
}

func (s *queryService) expandQuery(ctx context.Context, userQuery string, maxQueries int) ([]string, error) {
	systemPrompt := fmt.Sprintf(`You are a search query optimizer for financial and technology research.

Task: Generate 1-%d optimal web search queries.

Rules:
1. Queries in ENGLISH (sources are English)
2. Use keywords, not full sentences
3. Add year "2024" or "2025" for current topics
4. Split complex questions into sub-topics
5. Simple questions need only 1 query

Response format (JSON only):
{"queries": ["query1", "query2"]}`, maxQueries)

	userPrompt := fmt.Sprintf("User question: %s", userQuery)

	response, err := s.llm.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	var result struct {
		Queries []string `json:"queries"`
	}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return []string{userQuery}, nil
	}

	if len(result.Queries) == 0 {
		return []string{userQuery}, nil
	}

	if len(result.Queries) > maxQueries {
		result.Queries = result.Queries[:maxQueries]
	}

	return result.Queries, nil
}

func (s *queryService) searchWithCache(ctx context.Context, queries []string, domains []string, maxResults int) ([]search.SearchResult, error) {
	ctx, cancel := context.WithTimeout(ctx, s.config.SearchTimeout)
	defer cancel()

	resultsChan := make(chan []search.SearchResult, len(queries))
	g, ctx := errgroup.WithContext(ctx)

	for _, query := range queries {
		query := query       // capture for goroutine
		maxRes := maxResults // capture for goroutine - each query gets full maxResults
		g.Go(func() error {
			results, err := s.searchSingleQuery(ctx, query, domains, maxRes)
			if err != nil {
				s.logger.Warn("search query failed",
					zap.Error(err),
					zap.String("query", query),
				)
				return nil
			}
			resultsChan <- results
			return nil
		})
	}

	g.Wait()
	close(resultsChan)

	seen := make(map[string]bool)
	var allResults []search.SearchResult

	for results := range resultsChan {
		for _, r := range results {
			if !seen[r.URL] {
				seen[r.URL] = true
				allResults = append(allResults, r)
			}
		}
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	if len(allResults) > maxResults {
		allResults = allResults[:maxResults]
	}

	return allResults, nil
}

func (s *queryService) searchSingleQuery(ctx context.Context, query string, domains []string, maxResults int) ([]search.SearchResult, error) {
	cacheKey := s.cacheKey(query, domains)

	if cached, ok := s.cache.Get(cacheKey); ok {
		if results, ok := cached.([]search.SearchResult); ok {
			if s.metrics != nil {
				s.metrics.RecordCacheHit()
			}
			return results, nil
		}
	}

	if s.metrics != nil {
		s.metrics.RecordCacheMiss()
	}

	searchStart := time.Now()
	resp, err := s.search.Search(ctx, search.SearchRequest{
		Query:          query,
		IncludeDomains: domains,
		MaxResults:     maxResults,
		SearchDepth:    "basic",
	})
	if err != nil {
		if s.metrics != nil {
			s.metrics.RecordSearchRequest("error", time.Since(searchStart))
		}
		return nil, err
	}

	if s.metrics != nil {
		s.metrics.RecordSearchRequest("success", time.Since(searchStart))
	}

	s.cache.Set(cacheKey, resp.Results, s.config.CacheTTL)

	return resp.Results, nil
}

func (s *queryService) cacheKey(query string, domains []string) string {
	normalized := s.normalizeQuery(query)
	sortedDomains := make([]string, len(domains))
	copy(sortedDomains, domains)
	sort.Strings(sortedDomains)
	data := normalized + strings.Join(sortedDomains, ",")
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("search:%x", hash[:8])
}

func (s *queryService) normalizeQuery(q string) string {
	q = strings.ToLower(q)
	q = strings.TrimSpace(q)
	return strings.Join(strings.Fields(q), " ")
}

func (s *queryService) analyze(ctx context.Context, userQuery string, results []search.SearchResult) (string, error) {
	systemPrompt := `You are an expert analyst in financial technology and banking.

Rules:
1. Answer in Russian
2. Use ONLY information from provided sources
3. Reference sources as [S1], [S2], etc.
4. If information is insufficient, say so honestly
5. Structure: key points, examples, conclusions
6. Be objective, present different viewpoints`

	var sb strings.Builder
	sb.WriteString("Sources:\n\n")

	for i, r := range results {
		fmt.Fprintf(&sb, "[S%d] %s (%s)\n", i+1, r.Title, r.URL)
		fmt.Fprintf(&sb, "Score: %.2f\n", r.Score)
		content := r.Content
		if len(content) > 2000 {
			content = content[:2000] + "..."
		}
		fmt.Fprintf(&sb, "%s\n\n", content)
	}

	sb.WriteString("---\n\n")
	fmt.Fprintf(&sb, "User question: %s", userQuery)

	return s.llm.CompleteWithSystem(ctx, systemPrompt, sb.String())
}

func (s *queryService) toSourceRefs(results []search.SearchResult, trustMap map[string]domain.TrustLevel) []domain.SourceRef {
	refs := make([]domain.SourceRef, len(results))
	for i, r := range results {
		trustLevel := domain.TrustMedium
		resultDomain := extractDomain(r.URL)
		if level, ok := trustMap[resultDomain]; ok {
			trustLevel = level
		}

		refs[i] = domain.SourceRef{
			Marker:     fmt.Sprintf("[S%d]", i+1),
			Title:      r.Title,
			URL:        r.URL,
			TrustLevel: trustLevel,
		}
	}
	return refs
}

func extractDomain(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	if idx := strings.Index(url, "/"); idx != -1 {
		url = url[:idx]
	}
	url = strings.TrimPrefix(url, "www.")
	return url
}

func (s *queryService) reviewWithCritic(ctx context.Context, answer string, sources []search.SearchResult, question string) string {
	currentAnswer := answer

	for attempt := 0; attempt <= s.criticConfig.MaxRetries; attempt++ {
		result, err := s.critic.Review(ctx, currentAnswer, sources, question)
		if err != nil {
			s.logger.Warn("critic review failed, returning current answer",
				zap.Error(err),
				zap.Int("attempt", attempt),
			)
			return currentAnswer
		}

		s.logger.Info("critic review completed",
			zap.Bool("approved", result.Approved),
			zap.Int("issues_count", len(result.Issues)),
			zap.Int("attempt", attempt),
		)

		if !result.NeedsRevisionStrict(s.criticConfig.StrictMode) {
			return currentAnswer
		}

		if attempt >= s.criticConfig.MaxRetries {
			s.logger.Info("max critic retries reached, returning last answer",
				zap.Int("max_retries", s.criticConfig.MaxRetries),
			)
			return currentAnswer
		}

		improvedAnswer, err := s.improveAnswer(ctx, currentAnswer, result, sources, question)
		if err != nil {
			s.logger.Warn("failed to improve answer, returning current",
				zap.Error(err),
			)
			return currentAnswer
		}

		currentAnswer = improvedAnswer
	}

	return currentAnswer
}

func (s *queryService) improveAnswer(ctx context.Context, currentAnswer string, criticResult *domain.CriticResult, sources []search.SearchResult, question string) (string, error) {
	systemPrompt := `You are an expert analyst in financial technology and banking.

Your task is to improve an answer based on reviewer feedback.

Rules:
1. Answer in Russian
2. Use ONLY information from provided sources
3. Reference sources as [S1], [S2], etc.
4. Fix ALL issues mentioned by the reviewer
5. Keep the good parts of the original answer
6. Be objective, present different viewpoints`

	var sb strings.Builder
	sb.WriteString("=== REVIEWER FEEDBACK ===\n")
	sb.WriteString("The answer was reviewed and has the following issues:\n\n")

	for i, issue := range criticResult.Issues {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, issue)
	}

	if len(criticResult.Suggestions) > 0 {
		sb.WriteString("\nSuggestions for improvement:\n")
		for i, suggestion := range criticResult.Suggestions {
			fmt.Fprintf(&sb, "- %s\n", suggestion)
			if i >= 2 {
				break
			}
		}
	}

	sb.WriteString("\n=== ORIGINAL ANSWER ===\n")
	sb.WriteString(currentAnswer)
	sb.WriteString("\n\n")

	sb.WriteString("=== SOURCES ===\n")
	for i, src := range sources {
		fmt.Fprintf(&sb, "[S%d] %s (%s)\n", i+1, src.Title, src.URL)
		content := src.Content
		if len(content) > 1500 {
			content = content[:1500] + "..."
		}
		fmt.Fprintf(&sb, "%s\n\n", content)
	}

	sb.WriteString("=== ORIGINAL QUESTION ===\n")
	sb.WriteString(question)
	sb.WriteString("\n\n")

	sb.WriteString("=== INSTRUCTIONS ===\n")
	sb.WriteString("Please fix these issues and provide an improved answer. ")
	sb.WriteString("Keep using only the provided sources. ")
	sb.WriteString("Make sure all claims are properly cited.")

	return s.llm.CompleteWithSystem(ctx, systemPrompt, sb.String())
}
