package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	llmMock "github.com/kitbuilder587/fintech-bot/internal/llm/mock"
	"github.com/kitbuilder587/fintech-bot/internal/search"
)

func TestCriticService_Approved(t *testing.T) {
	logger := zap.NewNop()
	llmClient := llmMock.New()
	llmClient.Response = `{"approved": true, "issues": [], "confidence": 0.95}`

	config := domain.CriticConfig{
		MaxRetries: 3,
		StrictMode: false,
	}

	svc := NewCriticService(llmClient, logger, config)

	sources := []search.SearchResult{
		{Title: "Source 1", URL: "https://example.com/1", Content: "Content 1"},
	}

	result, err := svc.Review(context.Background(), "Test answer", sources, "Test question")
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}

	if result == nil {
		t.Fatal("nil result")
	}
	if !result.Approved {
		t.Error("expected Approved = true")
	}
	if len(result.Issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(result.Issues))
	}
	if result.Confidence != 0.95 {
		t.Errorf("Confidence = %f, want 0.95", result.Confidence)
	}
}

func TestCriticService_Rejected(t *testing.T) {
	logger := zap.NewNop()
	llmClient := llmMock.New()
	llmClient.Response = `{"approved": false, "issues": ["unsupported claim"], "confidence": 0.8}`

	config := domain.CriticConfig{
		MaxRetries: 3,
		StrictMode: false,
	}

	svc := NewCriticService(llmClient, logger, config)

	sources := []search.SearchResult{
		{Title: "Source 1", URL: "https://example.com/1", Content: "Content 1"},
	}

	result, err := svc.Review(context.Background(), "Test answer with unsupported claim", sources, "Test question")
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
	if result.Approved {
		t.Error("expected Approved = false")
	}
	if len(result.Issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(result.Issues))
	}
	if len(result.Issues) > 0 && result.Issues[0] != "unsupported claim" {
		t.Errorf("issue = %q, want 'unsupported claim'", result.Issues[0])
	}
	if result.Confidence != 0.8 {
		t.Errorf("Confidence = %f, want 0.8", result.Confidence)
	}
}

func TestCriticService_MalformedJSON(t *testing.T) {
	logger := zap.NewNop()
	llmClient := llmMock.New()
	llmClient.Response = "I think this answer is good"

	config := domain.CriticConfig{
		MaxRetries: 3,
		StrictMode: false,
	}

	svc := NewCriticService(llmClient, logger, config)

	sources := []search.SearchResult{
		{Title: "Source 1", URL: "https://example.com/1", Content: "Content 1"},
	}

	result, err := svc.Review(context.Background(), "Test answer", sources, "Test question")
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}

	if result == nil {
		t.Fatal("nil result")
	}
	if !result.Approved {
		t.Error("expected Approved = true on malformed JSON")
	}
	if result.Confidence >= 0.5 {
		t.Errorf("Confidence = %f, want < 0.5", result.Confidence)
	}
	foundParseError := false
	for _, issue := range result.Issues {
		if strings.Contains(issue, "critic_parse_failed") {
			foundParseError = true
			break
		}
	}
	if !foundParseError {
		t.Error("expected 'critic_parse_failed' in issues")
	}
}

func TestCriticService_LLMError(t *testing.T) {
	logger := zap.NewNop()
	llmClient := llmMock.New()
	llmClient.Error = errors.New("LLM request failed")

	config := domain.CriticConfig{
		MaxRetries: 3,
		StrictMode: false,
	}

	svc := NewCriticService(llmClient, logger, config)

	sources := []search.SearchResult{
		{Title: "Source 1", URL: "https://example.com/1", Content: "Content 1"},
	}

	_, err := svc.Review(context.Background(), "Test answer", sources, "Test question")
	if err == nil {
		t.Error("expected error")
	}
}

// проверяем формирование промпта
func TestCriticService_BuildPrompt(t *testing.T) {
	logger := zap.NewNop()
	llmClient := llmMock.New()

	config := domain.CriticConfig{
		MaxRetries: 3,
		StrictMode: false,
	}

	svc := NewCriticService(llmClient, logger, config)

	sources := []search.SearchResult{
		{Title: "Source One", URL: "https://example.com/1", Content: "Content one"},
		{Title: "Source Two", URL: "https://example.com/2", Content: "Content two"},
	}

	answer := "This is the analyst's answer"
	question := "What is the main topic?"

	prompt := svc.buildPrompt(answer, sources, question)

	if !strings.Contains(prompt, answer) {
		t.Error("prompt should contain answer")
	}
	if !strings.Contains(prompt, question) {
		t.Error("prompt should contain question")
	}
	if !strings.Contains(prompt, "Source One") || !strings.Contains(prompt, "Source Two") {
		t.Error("prompt should contain source titles")
	}
	if !strings.Contains(prompt, "https://example.com/1") {
		t.Error("prompt should contain source URL")
	}
	if !strings.Contains(prompt, "[S1]") || !strings.Contains(prompt, "[S2]") {
		t.Error("prompt should contain markers [S1], [S2]")
	}
}

func TestCriticService_Suggestions(t *testing.T) {
	logger := zap.NewNop()
	llmClient := llmMock.New()
	llmClient.Response = `{"approved": true, "issues": [], "suggestions": ["add more details", "include examples"], "confidence": 0.85}`

	config := domain.CriticConfig{
		MaxRetries: 3,
		StrictMode: false,
	}

	svc := NewCriticService(llmClient, logger, config)

	sources := []search.SearchResult{
		{Title: "Source 1", URL: "https://example.com/1", Content: "Content 1"},
	}

	result, err := svc.Review(context.Background(), "Test answer", sources, "Test question")
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}

	if result == nil {
		t.Fatal("nil result")
	}
	if !result.Approved {
		t.Error("expected Approved = true")
	}
	if len(result.Suggestions) != 2 {
		t.Errorf("expected 2 suggestions, got %d", len(result.Suggestions))
	}
	if len(result.Suggestions) >= 2 {
		if result.Suggestions[0] != "add more details" {
			t.Errorf("first suggestion = %q, want 'add more details'", result.Suggestions[0])
		}
		if result.Suggestions[1] != "include examples" {
			t.Errorf("second suggestion = %q, want 'include examples'", result.Suggestions[1])
		}
	}
}

func TestCriticService_EmptySources(t *testing.T) {
	logger := zap.NewNop()
	llmClient := llmMock.New()
	llmClient.Response = `{"approved": true, "issues": [], "confidence": 0.9}`

	config := domain.CriticConfig{
		MaxRetries: 3,
		StrictMode: false,
	}

	svc := NewCriticService(llmClient, logger, config)
	result, err := svc.Review(context.Background(), "Test answer", []search.SearchResult{}, "Test question")
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
}

func TestCriticService_ContextCanceled(t *testing.T) {
	logger := zap.NewNop()
	llmClient := llmMock.New()
	llmClient.Response = `{"approved": true, "issues": [], "confidence": 0.9}`

	config := domain.CriticConfig{
		MaxRetries: 3,
		StrictMode: false,
	}

	svc := NewCriticService(llmClient, logger, config)

	sources := []search.SearchResult{
		{Title: "Source 1", URL: "https://example.com/1", Content: "Content 1"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.Review(ctx, "Test answer", sources, "Test question")
	if err == nil {
		t.Error("expected error on canceled context")
	}
}
