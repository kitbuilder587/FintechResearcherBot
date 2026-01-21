package gigachat

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

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(authResponse{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(30 * time.Minute).UnixMilli(),
		})
	}))
	defer authServer.Close()

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
			apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer apiServer.Close()

			client := New(Config{
				ClientID:     "test-id",
				ClientSecret: "test-secret",
				AuthURL:      authServer.URL,
				BaseURL:      apiServer.URL,
				Timeout:      5 * time.Second,
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

func TestClient_TokenCaching(t *testing.T) {
	logger := zap.NewNop()
	authCalls := 0

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCalls++
		json.NewEncoder(w).Encode(authResponse{
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(30 * time.Minute).UnixMilli(),
		})
	}))
	defer authServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(llm.ChatResponse{
			Choices: []llm.Choice{
				{Message: llm.Message{Role: "assistant", Content: "response"}},
			},
		})
	}))
	defer apiServer.Close()

	client := New(Config{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		AuthURL:      authServer.URL,
		BaseURL:      apiServer.URL,
	}, logger)

	_, err := client.CompleteWithSystem(context.Background(), "system", "prompt1")
	if err != nil {
		t.Fatalf("first call error = %v", err)
	}

	_, err = client.CompleteWithSystem(context.Background(), "system", "prompt2")
	if err != nil {
		t.Fatalf("second call error = %v", err)
	}

	if authCalls != 1 {
		t.Errorf("auth calls = %d, want 1", authCalls)
	}
}
