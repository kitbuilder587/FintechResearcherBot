package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/llm"
)

func TestClient_CompleteWithSystem(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name       string
		response   interface{}
		statusCode int
		wantErr    error
	}{
		{
			name: "successful completion",
			response: llm.ChatResponse{
				Choices: []llm.Choice{
					{Message: llm.Message{Role: "assistant", Content: "Test response"}},
				},
			},
			statusCode: http.StatusOK,
			wantErr:    nil,
		},
		{
			name:       "unauthorized",
			response:   map[string]string{"error": "unauthorized"},
			statusCode: http.StatusUnauthorized,
			wantErr:    llm.ErrAuthFailed,
		},
		{
			name:       "rate limit",
			response:   map[string]string{"error": "rate limit"},
			statusCode: http.StatusTooManyRequests,
			wantErr:    llm.ErrRateLimit,
		},
		{
			name: "empty response",
			response: llm.ChatResponse{
				Choices: []llm.Choice{},
			},
			statusCode: http.StatusOK,
			wantErr:    llm.ErrEmptyResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "" {
					t.Error("missing authorization header")
				}

				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := New(Config{
				APIKey:  "test-key",
				BaseURL: server.URL,
				Timeout: 5 * time.Second,
			}, logger)

			result, err := client.CompleteWithSystem(context.Background(), "system", "prompt")

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("CompleteWithSystem() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("CompleteWithSystem() unexpected error = %v", err)
				return
			}

			if result == "" {
				t.Error("CompleteWithSystem() returned empty result")
			}
		})
	}
}
