package mock

import (
	"context"
	"sync"
	"time"

	"github.com/kitbuilder587/fintech-bot/internal/search"
)

type Client struct {
	Results []search.SearchResult
	Error   error
	Delay   time.Duration

	CallCount   int
	LastRequest search.SearchRequest
	AllRequests []search.SearchRequest

	mu sync.Mutex
}

func New() *Client {
	return &Client{}
}

func (c *Client) WithResults(results []search.SearchResult) *Client {
	c.Results = results
	return c
}

func (c *Client) WithError(err error) *Client {
	c.Error = err
	return c
}

func (c *Client) WithDelay(delay time.Duration) *Client {
	c.Delay = delay
	return c
}

func (c *Client) Search(ctx context.Context, req search.SearchRequest) (*search.SearchResponse, error) {
	c.mu.Lock()
	c.CallCount++
	c.LastRequest = req
	c.AllRequests = append(c.AllRequests, req)
	delay := c.Delay
	err := c.Error
	results := c.Results
	c.mu.Unlock()

	if delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, search.ErrEmptyResults
	}

	return &search.SearchResponse{
		Query:        req.Query,
		Results:      results,
		ResponseTime: 0.5,
	}, nil
}

func (c *Client) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CallCount = 0
	c.LastRequest = search.SearchRequest{}
	c.AllRequests = nil
}
