package tavily

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/search"
)

type Config struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
	logger  *zap.Logger
}

func New(cfg Config, logger *zap.Logger) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.tavily.com"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &Client{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		client:  &http.Client{Timeout: cfg.Timeout},
		logger:  logger,
	}
}

type tavilyRequest struct {
	APIKey            string   `json:"api_key"`
	Query             string   `json:"query"`
	IncludeDomains    []string `json:"include_domains,omitempty"`
	ExcludeDomains    []string `json:"exclude_domains,omitempty"`
	MaxResults        int      `json:"max_results,omitempty"`
	SearchDepth       string   `json:"search_depth,omitempty"`
	TimeRange         string   `json:"time_range,omitempty"`
	Topic             string   `json:"topic,omitempty"`
	IncludeAnswer     bool     `json:"include_answer"`
	IncludeRawContent bool     `json:"include_raw_content"`
}

type tavilyResponse struct {
	Query        string         `json:"query"`
	Results      []tavilyResult `json:"results"`
	ResponseTime float64        `json:"response_time"`
}

type tavilyResult struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Content       string  `json:"content"`
	Score         float64 `json:"score"`
	PublishedDate string  `json:"published_date"`
}

func (c *Client) Search(ctx context.Context, req search.SearchRequest) (*search.SearchResponse, error) {
	if req.MaxResults == 0 {
		req.MaxResults = 5
	}
	if req.SearchDepth == "" {
		req.SearchDepth = "basic"
	}

	tavilyReq := tavilyRequest{
		APIKey:            c.apiKey,
		Query:             req.Query,
		IncludeDomains:    req.IncludeDomains,
		ExcludeDomains:    req.ExcludeDomains,
		MaxResults:        req.MaxResults,
		SearchDepth:       req.SearchDepth,
		TimeRange:         req.TimeRange,
		Topic:             req.Topic,
		IncludeAnswer:     false,
		IncludeRawContent: false,
	}

	body, err := json.Marshal(tavilyReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

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

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/search", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

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
			var tavilyResp tavilyResponse
			if err := json.Unmarshal(respBody, &tavilyResp); err != nil {
				return nil, fmt.Errorf("unmarshal response: %w", err)
			}

			if len(tavilyResp.Results) == 0 {
				return nil, search.ErrEmptyResults
			}

			return c.toSearchResponse(&tavilyResp), nil

		case http.StatusUnauthorized:
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

func (c *Client) toSearchResponse(resp *tavilyResponse) *search.SearchResponse {
	results := make([]search.SearchResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = search.SearchResult{
			Title:         r.Title,
			URL:           r.URL,
			Content:       r.Content,
			Score:         r.Score,
			PublishedDate: r.PublishedDate,
		}
	}

	return &search.SearchResponse{
		Query:        resp.Query,
		Results:      results,
		ResponseTime: resp.ResponseTime,
	}
}
