package searxng

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/search"
)

func TestClient_Search(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name       string
		response   interface{}
		statusCode int
		wantErr    error
	}{
		{
			name: "successful search",
			response: searxngResponse{
				Query: "test query",
				Results: []searxngResult{
					{Title: "Test", URL: "https://example.com", Content: "Content", Score: 0.9, PublishedDate: "2026-05-01"},
				},
				SearchTime: 1.5,
			},
			statusCode: http.StatusOK,
		},
		{
			name: "empty results",
			response: searxngResponse{
				Query:   "test query",
				Results: []searxngResult{},
			},
			statusCode: http.StatusOK,
			wantErr:    search.ErrEmptyResults,
		},
		{
			name:       "forbidden",
			response:   map[string]string{"error": "forbidden"},
			statusCode: http.StatusForbidden,
			wantErr:    search.ErrUnauthorized,
		},
		{
			name:       "rate limit",
			response:   map[string]string{"error": "rate limit"},
			statusCode: http.StatusTooManyRequests,
			wantErr:    search.ErrRateLimit,
		},
		{
			name:       "bad request",
			response:   map[string]string{"error": "bad request"},
			statusCode: http.StatusBadRequest,
			wantErr:    search.ErrInvalidRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/search" {
					t.Errorf("path = %v, want /search", r.URL.Path)
				}
				if r.URL.Query().Get("format") != "json" {
					t.Errorf("format = %v, want json", r.URL.Query().Get("format"))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := New(Config{
				BaseURL: server.URL,
				Timeout: 5 * time.Second,
			}, logger)

			resp, err := client.Search(context.Background(), search.SearchRequest{
				Query:      "test query",
				MaxResults: 5,
			})

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("Search() unexpected error = %v", err)
				return
			}
			if resp == nil {
				t.Fatal("Search() returned nil response")
			}
			if len(resp.Results) != 1 {
				t.Fatalf("results count = %d, want 1", len(resp.Results))
			}
			if resp.Results[0].PublishedDate != "2026-05-01" {
				t.Errorf("PublishedDate = %v, want 2026-05-01", resp.Results[0].PublishedDate)
			}
		})
	}
}

func TestClient_Search_QueryParams(t *testing.T) {
	logger := zap.NewNop()

	var receivedQuery string
	var receivedTimeRange string
	var receivedCategories string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.Query().Get("q")
		receivedTimeRange = r.URL.Query().Get("time_range")
		receivedCategories = r.URL.Query().Get("categories")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searxngResponse{
			Query:   receivedQuery,
			Results: []searxngResult{{Title: "Test", URL: "https://mckinsey.com", Content: "Content"}},
		})
	}))
	defer server.Close()

	client := New(Config{
		BaseURL: server.URL,
	}, logger)

	_, err := client.Search(context.Background(), search.SearchRequest{
		Query:          "fintech",
		IncludeDomains: []string{"mckinsey.com", "gartner.com"},
		ExcludeDomains: []string{"spam.example"},
		TimeRange:      "month",
		Topic:          "general",
	})

	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if !strings.Contains(receivedQuery, "site:mckinsey.com") {
		t.Errorf("query = %v, want site:mckinsey.com", receivedQuery)
	}
	if !strings.Contains(receivedQuery, "site:gartner.com") {
		t.Errorf("query = %v, want site:gartner.com", receivedQuery)
	}
	if !strings.Contains(receivedQuery, " OR ") {
		t.Errorf("query = %v, want OR between include domains", receivedQuery)
	}
	if !strings.Contains(receivedQuery, "-site:spam.example") {
		t.Errorf("query = %v, want -site:spam.example", receivedQuery)
	}
	if receivedTimeRange != "month" {
		t.Errorf("time_range = %v, want month", receivedTimeRange)
	}
	if receivedCategories != "general" {
		t.Errorf("categories = %v, want general", receivedCategories)
	}
}

func TestClient_Search_MaxResults(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searxngResponse{
			Query: "test",
			Results: []searxngResult{
				{Title: "One", URL: "https://example.com/1", Content: "One"},
				{Title: "Two", URL: "https://example.com/2", Content: "Two"},
			},
		})
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL}, logger)

	resp, err := client.Search(context.Background(), search.SearchRequest{
		Query:      "test",
		MaxResults: 1,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(resp.Results) != 1 {
		t.Errorf("results count = %d, want 1", len(resp.Results))
	}
}

func TestClient_Search_ReranksNaturalQuestion(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searxngResponse{
			Query: "What are the main fintech funding trends in 2025?",
			Results: []searxngResult{
				{Title: "Main Street Capital Corporation (MAIN) Stock Price", URL: "https://finance.example/main", Content: "Stock quote for Main Street Capital", Score: 10},
				{Title: "5 Key Fintech Trends for 2025: Navigating Funding Shifts", URL: "https://fintech.example/trends", Content: "Fintech funding trends in 2025", Score: 1},
			},
		})
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL}, logger)

	resp, err := client.Search(context.Background(), search.SearchRequest{
		Query:      "What are the main fintech funding trends in 2025?",
		MaxResults: 1,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if got := resp.Results[0].URL; got != "https://fintech.example/trends" {
		t.Errorf("top result = %v, want relevant fintech result", got)
	}
}

func TestClient_Search_DemotesGenericReferenceResults(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searxngResponse{
			Query: "What risks and growth drivers define the BNPL market?",
			Results: []searxngResult{
				{Title: "Risk - Wikipedia", URL: "https://en.wikipedia.org/wiki/Risk", Content: "Risk is the possibility of something bad happening.", Score: 10},
				{Title: "Aligning Supply Chain Cuts with Long-Term Growth", URL: "https://strategy.example/growth", Content: "Market growth drivers and risks for supply chains", Score: 10},
				{Title: "BNPL trends: What is shaping the BNPL market in 2026", URL: "https://bnpl.example/trends", Content: "BNPL market growth drivers and risks", Score: 1},
				{Title: "BNPL regulation update 2025: What risk leaders need to know", URL: "https://bnpl.example/risk", Content: "BNPL risk and regulation", Score: 1},
			},
		})
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL}, logger)

	resp, err := client.Search(context.Background(), search.SearchRequest{
		Query:      "What risks and growth drivers define the BNPL market?",
		MaxResults: 2,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	for _, result := range resp.Results {
		if result.URL == "https://en.wikipedia.org/wiki/Risk" {
			t.Fatalf("generic result was not demoted: %+v", resp.Results)
		}
	}
}

func TestClient_Search_Timeout(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	client := New(Config{
		BaseURL: server.URL,
		Timeout: 100 * time.Millisecond,
	}, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := client.Search(ctx, search.SearchRequest{Query: "test"})

	if err == nil {
		t.Error("Search() expected timeout error")
	}
}
