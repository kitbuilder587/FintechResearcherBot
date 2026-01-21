package tavily

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
			response: tavilyResponse{
				Query: "test query",
				Results: []tavilyResult{
					{Title: "Test", URL: "https://example.com", Content: "Content", Score: 0.9},
				},
				ResponseTime: 1.5,
			},
			statusCode: http.StatusOK,
			wantErr:    nil,
		},
		{
			name: "empty results",
			response: tavilyResponse{
				Query:   "test query",
				Results: []tavilyResult{},
			},
			statusCode: http.StatusOK,
			wantErr:    search.ErrEmptyResults,
		},
		{
			name:       "unauthorized",
			response:   map[string]string{"error": "unauthorized"},
			statusCode: http.StatusUnauthorized,
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
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := New(Config{
				APIKey:  "test-key",
				BaseURL: server.URL,
				Timeout: 5 * time.Second,
			}, logger)

			req := search.SearchRequest{
				Query:      "test query",
				MaxResults: 5,
			}

			resp, err := client.Search(context.Background(), req)

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
				t.Error("Search() returned nil response")
			}
		})
	}
}

func TestClient_Search_IncludeDomains(t *testing.T) {
	logger := zap.NewNop()

	var receivedDomains []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req tavilyRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedDomains = req.IncludeDomains

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tavilyResponse{
			Query:   req.Query,
			Results: []tavilyResult{{Title: "Test", URL: "https://mckinsey.com", Content: "Content"}},
		})
	}))
	defer server.Close()

	client := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	}, logger)

	_, err := client.Search(context.Background(), search.SearchRequest{
		Query:          "fintech",
		IncludeDomains: []string{"mckinsey.com", "gartner.com"},
	})

	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(receivedDomains) != 2 {
		t.Errorf("include_domains = %v, want 2 domains", receivedDomains)
	}
}

func TestClient_Search_Timeout(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	client := New(Config{
		APIKey:  "test-key",
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
