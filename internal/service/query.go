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
	"github.com/kitbuilder587/fintech-bot/internal/prompts"
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

type promptSource struct {
	Index   int
	Title   string
	URL     string
	Score   float64
	Content string
}

type promptIssue struct {
	Index int
	Text  string
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

func (s *queryService) generateAnswer(ctx context.Context, question string, results []search.SearchResult, worldContext string, strategy domain.Strategy) (string, error) {
	if s.coordinator != nil {
		coordResp, coordErr := s.coordinator.Process(ctx, AgentCoordinatorRequest{
			Question:      question,
			SearchResults: results,
			Context:       worldContext,
			Strategy:      strategy,
		})
		if coordErr != nil {
			s.logger.Warn("coordinator processing failed, falling back to analyze",
				zap.Error(coordErr),
			)
		} else if coordResp != nil && coordResp.FinalAnswer != "" {
			s.logger.Debug("using coordinator answer",
				zap.Int("agents_used", len(coordResp.AgentsUsed)),
			)
			return coordResp.FinalAnswer, nil
		}
	}

	return s.analyze(ctx, question, results)
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

	answer, err := s.generateAnswer(ctx, req.Text, results, worldContext, req.Strategy)
	if err != nil {
		return nil, err
	}

	// критик проверяет ответ (опционально)
	if s.critic != nil && req.Strategy.UseCritic {
		answer, results = s.reviewWithCritic(ctx, answer, results, req.Text, domains, worldContext, req.Strategy)
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
	currentYear := time.Now().Year()
	systemPrompt, err := prompts.Render("query_expansion_system.tmpl", struct {
		MaxQueries  int
		CurrentYear int
	}{
		MaxQueries:  maxQueries,
		CurrentYear: currentYear,
	})
	if err != nil {
		return nil, err
	}

	userPrompt, err := prompts.Render("query_expansion_user.tmpl", struct {
		UserQuery string
	}{
		UserQuery: userQuery,
	})
	if err != nil {
		return nil, err
	}

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
	userPrompt, err := prompts.Render("analysis_user.tmpl", struct {
		Sources   []promptSource
		UserQuery string
	}{
		Sources:   promptSources(results, 2000),
		UserQuery: userQuery,
	})
	if err != nil {
		return "", err
	}

	return s.llm.CompleteWithSystem(ctx, prompts.Text("analysis_system.md"), userPrompt)
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

func promptSources(results []search.SearchResult, maxContent int) []promptSource {
	items := make([]promptSource, 0, len(results))
	for i, r := range results {
		content := r.Content
		if maxContent > 0 && len(content) > maxContent {
			content = content[:maxContent] + "..."
		}
		items = append(items, promptSource{
			Index:   i + 1,
			Title:   r.Title,
			URL:     r.URL,
			Score:   r.Score,
			Content: content,
		})
	}
	return items
}

func (s *queryService) searchForCritic(ctx context.Context, queries []string, currentSources []search.SearchResult, domains []string, strategy domain.Strategy) ([]search.SearchResult, bool) {
	maxQueries := strategy.MaxQueries
	if maxQueries <= 0 {
		maxQueries = s.config.MaxSearchQueries
	}
	if maxQueries <= 0 {
		maxQueries = 3
	}

	searchQueries := normalizeCriticQueries(queries, maxQueries)
	if len(searchQueries) == 0 {
		s.logger.Warn("verifier requested search without valid queries")
		return currentSources, false
	}

	maxResults := strategy.MaxResults
	if maxResults <= 0 {
		maxResults = 15
	}

	results, err := s.searchWithCache(ctx, searchQueries, domains, maxResults)
	if err != nil {
		s.logger.Warn("verifier search failed",
			zap.Error(err),
		)
		return currentSources, false
	}
	if len(results) == 0 {
		s.logger.Info("verifier search returned no results")
		return currentSources, false
	}

	merged, hasNew := mergeSearchResults(currentSources, results, maxResults)
	if !hasNew {
		s.logger.Info("verifier search returned only already known sources")
		return currentSources, false
	}

	s.logger.Info("verifier search added sources",
		zap.Int("queries", len(searchQueries)),
		zap.Int("new_results", len(results)),
		zap.Int("merged_results", len(merged)),
	)

	return merged, true
}

func normalizeCriticQueries(queries []string, maxQueries int) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(queries))
	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		key := strings.ToLower(strings.Join(strings.Fields(query), " "))
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, query)
		if maxQueries > 0 && len(result) >= maxQueries {
			break
		}
	}
	return result
}

func mergeSearchResults(existing []search.SearchResult, extra []search.SearchResult, maxResults int) ([]search.SearchResult, bool) {
	byURL := make(map[string]search.SearchResult, len(existing)+len(extra))
	existingURLs := make(map[string]bool, len(existing))
	for _, item := range existing {
		byURL[item.URL] = item
		existingURLs[item.URL] = true
	}

	for _, item := range extra {
		if current, ok := byURL[item.URL]; ok {
			if item.Score > current.Score {
				byURL[item.URL] = item
			}
			continue
		}
		byURL[item.URL] = item
	}

	merged := make([]search.SearchResult, 0, len(byURL))
	for _, item := range byURL {
		merged = append(merged, item)
	}

	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Score == merged[j].Score {
			return merged[i].URL < merged[j].URL
		}
		return merged[i].Score > merged[j].Score
	})

	if maxResults > 0 && len(merged) > maxResults {
		merged = merged[:maxResults]
	}

	for _, item := range merged {
		if !existingURLs[item.URL] {
			return merged, true
		}
	}

	return merged, false
}

func (s *queryService) reviewWithCritic(ctx context.Context, answer string, sources []search.SearchResult, question string, domains []string, worldContext string, strategy domain.Strategy) (string, []search.SearchResult) {
	currentAnswer := answer
	currentSources := sources

	for attempt := 0; attempt <= s.criticConfig.MaxRetries; attempt++ {
		result, err := s.critic.Review(ctx, currentAnswer, currentSources, question)
		if err != nil {
			s.logger.Warn("critic review failed, returning current answer",
				zap.Error(err),
				zap.Int("attempt", attempt),
			)
			return currentAnswer, currentSources
		}

		s.logger.Info("critic review completed",
			zap.Bool("approved", result.Approved),
			zap.Int("issues_count", len(result.Issues)),
			zap.Bool("needs_search", result.NeedsSearch),
			zap.Int("search_queries_count", len(result.SearchQueries)),
			zap.Int("attempt", attempt),
		)

		needsRevision := result.NeedsRevisionStrict(s.criticConfig.StrictMode)

		if result.NeedsSearch {
			if attempt >= s.criticConfig.MaxRetries {
				s.logger.Info("max critic retries reached before verifier search",
					zap.Int("max_retries", s.criticConfig.MaxRetries),
				)
				return currentAnswer, currentSources
			}

			updatedSources, ok := s.searchForCritic(ctx, result.SearchQueries, currentSources, domains, strategy)
			if ok {
				updatedAnswer, err := s.generateAnswer(ctx, question, updatedSources, worldContext, strategy)
				if err != nil {
					s.logger.Warn("failed to regenerate answer after verifier search",
						zap.Error(err),
					)
					if !needsRevision {
						return currentAnswer, currentSources
					}
				} else {
					currentAnswer = updatedAnswer
					currentSources = updatedSources
					continue
				}
			} else if !needsRevision {
				return currentAnswer, currentSources
			}
		}

		if !result.NeedsRevisionStrict(s.criticConfig.StrictMode) {
			return currentAnswer, currentSources
		}

		if attempt >= s.criticConfig.MaxRetries {
			s.logger.Info("max critic retries reached, returning last answer",
				zap.Int("max_retries", s.criticConfig.MaxRetries),
			)
			return currentAnswer, currentSources
		}

		improvedAnswer, err := s.improveAnswer(ctx, currentAnswer, result, currentSources, question)
		if err != nil {
			s.logger.Warn("failed to improve answer, returning current",
				zap.Error(err),
			)
			return currentAnswer, currentSources
		}

		currentAnswer = improvedAnswer
	}

	return currentAnswer, currentSources
}

func (s *queryService) improveAnswer(ctx context.Context, currentAnswer string, criticResult *domain.CriticResult, sources []search.SearchResult, question string) (string, error) {
	issues := make([]promptIssue, 0, len(criticResult.Issues))
	for i, issue := range criticResult.Issues {
		issues = append(issues, promptIssue{Index: i + 1, Text: issue})
	}

	suggestions := criticResult.Suggestions
	if len(suggestions) > 3 {
		suggestions = suggestions[:3]
	}

	userPrompt, err := prompts.Render("answer_improvement_user.tmpl", struct {
		Issues        []promptIssue
		Suggestions   []string
		CurrentAnswer string
		Sources       []promptSource
		Question      string
	}{
		Issues:        issues,
		Suggestions:   suggestions,
		CurrentAnswer: currentAnswer,
		Sources:       promptSources(sources, 1500),
		Question:      question,
	})
	if err != nil {
		return "", err
	}

	return s.llm.CompleteWithSystem(ctx, prompts.Text("answer_improvement_system.md"), userPrompt)
}
