package searxng

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/search"
)

type Config struct {
	BaseURL string
	Timeout time.Duration
}

type Client struct {
	baseURL string
	client  *http.Client
	logger  *zap.Logger
}

func New(cfg Config, logger *zap.Logger) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://searxng:8080"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		client:  &http.Client{Timeout: cfg.Timeout},
		logger:  logger,
	}
}

type searxngResponse struct {
	Query        string          `json:"query"`
	Results      []searxngResult `json:"results"`
	SearchTime   float64         `json:"search_time"`
	ResponseTime float64         `json:"response_time"`
}

type searxngResult struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Content       string  `json:"content"`
	Score         float64 `json:"score"`
	PublishedDate string  `json:"publishedDate"`
	PublishedAt   string  `json:"published_date"`
}

func (c *Client) Search(ctx context.Context, req search.SearchRequest) (*search.SearchResponse, error) {
	if req.MaxResults == 0 {
		req.MaxResults = 5
	}

	endpoint, err := url.Parse(c.baseURL + "/search")
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}

	params := endpoint.Query()
	params.Set("q", buildQuery(req.Query, req.IncludeDomains, req.ExcludeDomains))
	params.Set("format", "json")
	if req.TimeRange != "" {
		params.Set("time_range", req.TimeRange)
	}
	if req.Topic != "" {
		params.Set("categories", req.Topic)
	}
	endpoint.RawQuery = params.Encode()

	var lastErr error
	backoff := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	for attempt := 0; attempt <= len(backoff); attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff[attempt-1]):
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		httpReq.Header.Set("Accept", "application/json")

		resp, err := c.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("do request: %w", err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response: %w", err)
			continue
		}

		switch resp.StatusCode {
		case http.StatusOK:
			var searxngResp searxngResponse
			if err := json.Unmarshal(respBody, &searxngResp); err != nil {
				return nil, fmt.Errorf("unmarshal response: %w", err)
			}
			if len(searxngResp.Results) == 0 {
				return nil, search.ErrEmptyResults
			}
			return c.toSearchResponse(&searxngResp, req.Query, req.MaxResults), nil
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, search.ErrUnauthorized
		case http.StatusTooManyRequests:
			return nil, search.ErrRateLimit
		case http.StatusBadRequest:
			return nil, search.ErrInvalidRequest
		default:
			if resp.StatusCode >= 500 {
				lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
				continue
			}
			return nil, fmt.Errorf("%w: status %d", search.ErrSearchFailed, resp.StatusCode)
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", search.ErrSearchFailed, lastErr)
	}
	return nil, search.ErrSearchFailed
}

func (c *Client) toSearchResponse(resp *searxngResponse, query string, maxResults int) *search.SearchResponse {
	rankedResults := rankResults(resp.Results, query)
	resultsCount := len(rankedResults)
	if maxResults > 0 && resultsCount > maxResults {
		resultsCount = maxResults
	}

	results := make([]search.SearchResult, 0, resultsCount)
	for i := 0; i < resultsCount; i++ {
		r := rankedResults[i]
		publishedDate := r.PublishedDate
		if publishedDate == "" {
			publishedDate = r.PublishedAt
		}
		results = append(results, search.SearchResult{
			Title:         r.Title,
			URL:           r.URL,
			Content:       r.Content,
			Score:         r.Score,
			PublishedDate: publishedDate,
		})
	}

	responseTime := resp.ResponseTime
	if responseTime == 0 {
		responseTime = resp.SearchTime
	}

	return &search.SearchResponse{
		Query:        resp.Query,
		Results:      results,
		ResponseTime: responseTime,
	}
}

type rankedResult struct {
	result searxngResult
	score  float64
	index  int
}

func rankResults(results []searxngResult, query string) []searxngResult {
	terms := significantTerms(query)
	if len(terms) == 0 || len(results) < 2 {
		return results
	}

	ranked := make([]rankedResult, 0, len(results))
	for i, result := range results {
		ranked = append(ranked, rankedResult{
			result: result,
			score:  localRelevanceScore(result, terms),
			index:  i,
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].index < ranked[j].index
		}
		return ranked[i].score > ranked[j].score
	})

	ordered := make([]searxngResult, 0, len(ranked))
	for _, item := range ranked {
		ordered = append(ordered, item.result)
	}
	return ordered
}

func localRelevanceScore(result searxngResult, terms []string) float64 {
	titleTerms := tokenSet(result.Title)
	bodyTerms := tokenSet(result.Title + " " + result.Content + " " + result.URL)
	score := result.Score * 0.001
	matches := 0

	for _, term := range terms {
		if titleTerms[term] {
			score += 3
		}
		if bodyTerms[term] {
			score += 1
			matches++
		}
	}
	for _, term := range terms {
		if domainAnchorTerms[term] && !bodyTerms[term] {
			score -= 6
		}
	}

	title := strings.ToLower(result.Title)
	if score < 5 && (strings.Contains(title, "definition") || strings.Contains(title, "meaning") || strings.Contains(title, "dictionary")) {
		score -= 3
	}
	if matches <= 1 && genericReferenceTitle(title) {
		score -= 8
	}

	return score
}

func genericReferenceTitle(title string) bool {
	return strings.Contains(title, " - wikipedia") ||
		strings.Contains(title, " | wikipedia") ||
		strings.Contains(title, "definition") ||
		strings.Contains(title, "meaning") ||
		strings.Contains(title, "dictionary")
}

func tokenSet(text string) map[string]bool {
	tokens := tokenize(text)
	set := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		set[token] = true
	}
	return set
}

func significantTerms(text string) []string {
	rawTokens := tokenize(text)
	seen := make(map[string]bool, len(rawTokens))
	terms := make([]string, 0, len(rawTokens))
	for _, token := range rawTokens {
		if queryStopWords[token] || len([]rune(token)) < 3 {
			continue
		}
		if !seen[token] {
			seen[token] = true
			terms = append(terms, token)
		}
	}
	return terms
}

func tokenize(text string) []string {
	parts := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		token := normalizeToken(part)
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func normalizeToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if len(token) > 4 && strings.HasSuffix(token, "ies") {
		return token[:len(token)-3] + "y"
	}
	if len(token) > 4 && strings.HasSuffix(token, "es") {
		return token[:len(token)-2]
	}
	if len(token) > 3 && strings.HasSuffix(token, "s") {
		return token[:len(token)-1]
	}
	return token
}

var queryStopWords = map[string]bool{
	"about": true, "after": true, "against": true, "and": true, "are": true,
	"been": true, "being": true, "can": true, "does": true, "for": true,
	"from": true, "have": true, "how": true, "into": true, "main": true,
	"most": true, "over": true, "should": true, "that": true, "the": true,
	"their": true, "these": true, "this": true, "those": true, "through": true,
	"what": true, "when": true, "where": true, "which": true, "while": true,
	"with": true, "without": true, "define": true,
	"какие": true, "какой": true, "какая": true, "какое": true, "как": true,
	"или": true, "для": true, "при": true, "что": true, "это": true,
	"влияют": true, "основные": true, "ключевые": true,
}

var domainAnchorTerms = map[string]bool{
	"bnpl": true, "cbdc": true, "cybersecurity": true, "fintech": true,
	"neobank": true, "psd2": true, "stablecoin": true,
}

func buildQuery(query string, includeDomains []string, excludeDomains []string) string {
	parts := []string{query}
	includeParts := make([]string, 0, len(includeDomains))
	for _, domain := range includeDomains {
		if domain = strings.TrimSpace(domain); domain != "" {
			includeParts = append(includeParts, "site:"+domain)
		}
	}
	if len(includeParts) == 1 {
		parts = append(parts, includeParts[0])
	} else if len(includeParts) > 1 {
		parts = append(parts, "("+strings.Join(includeParts, " OR ")+")")
	}
	for _, domain := range excludeDomains {
		if domain = strings.TrimSpace(domain); domain != "" {
			parts = append(parts, "-site:"+domain)
		}
	}
	return strings.Join(parts, " ")
}
