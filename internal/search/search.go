package search

import (
	"context"
	"errors"
)

var (
	ErrUnauthorized   = errors.New("invalid API key")
	ErrRateLimit      = errors.New("rate limit exceeded")
	ErrInvalidRequest = errors.New("invalid request parameters")
	ErrSearchFailed   = errors.New("search request failed")
	ErrEmptyResults   = errors.New("no results found")
)

type SearchClient interface {
	Search(ctx context.Context, req SearchRequest) (*SearchResponse, error)
}

type SearchRequest struct {
	Query          string
	IncludeDomains []string
	ExcludeDomains []string
	MaxResults     int
	SearchDepth    string
	TimeRange      string
	Topic          string
}

type SearchResponse struct {
	Query        string
	Results      []SearchResult
	ResponseTime float64
}

type SearchResult struct {
	Title         string
	URL           string
	Content       string
	Score         float64
	PublishedDate string
}
