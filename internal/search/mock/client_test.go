package mock

import (
	"context"
	"testing"
	"time"

	"github.com/kitbuilder587/fintech-bot/internal/search"
)

func TestMockClient_Search(t *testing.T) {
	results := []search.SearchResult{
		{Title: "Test 1", URL: "https://example.com/1", Content: "Content 1"},
		{Title: "Test 2", URL: "https://example.com/2", Content: "Content 2"},
	}

	client := New().WithResults(results)

	resp, err := client.Search(context.Background(), search.SearchRequest{Query: "test"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(resp.Results) != 2 {
		t.Errorf("Search() got %d results, want 2", len(resp.Results))
	}
}

func TestMockClient_Error(t *testing.T) {
	client := New().WithError(search.ErrSearchFailed)

	_, err := client.Search(context.Background(), search.SearchRequest{Query: "test"})
	if err != search.ErrSearchFailed {
		t.Errorf("Search() error = %v, want ErrSearchFailed", err)
	}
}

func TestMockClient_Delay(t *testing.T) {
	client := New().
		WithResults([]search.SearchResult{{Title: "Test"}}).
		WithDelay(50 * time.Millisecond)

	start := time.Now()
	_, err := client.Search(context.Background(), search.SearchRequest{Query: "test"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if elapsed < 50*time.Millisecond {
		t.Errorf("Search() elapsed = %v, want >= 50ms", elapsed)
	}
}

func TestMockClient_ContextCancellation(t *testing.T) {
	client := New().
		WithResults([]search.SearchResult{{Title: "Test"}}).
		WithDelay(1 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Search(ctx, search.SearchRequest{Query: "test"})
	if err != context.DeadlineExceeded {
		t.Errorf("Search() error = %v, want context.DeadlineExceeded", err)
	}
}
