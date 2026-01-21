package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"
)

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

func NewChatRequest(model, system, prompt string) ChatRequest {
	return ChatRequest{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: system},
			{Role: "user", Content: prompt},
		},
	}
}

func HandleHTTPError(statusCode int, body []byte, logger *zap.Logger, provider string) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return ErrAuthFailed
	case http.StatusTooManyRequests:
		return ErrRateLimit
	default:
		logger.Error(provider+" request failed",
			zap.Int("status", statusCode),
			zap.String("body", string(body)),
		)
		return fmt.Errorf("%w: status %d", ErrRequestFailed, statusCode)
	}
}

func ParseChatResponse(body []byte) (*ChatResponse, error) {
	var resp ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &resp, nil
}

func ExtractContent(resp *ChatResponse) (string, error) {
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return "", ErrEmptyResponse
	}
	return resp.Choices[0].Message.Content, nil
}

func DoRequest(client *http.Client, req *http.Request) ([]byte, int, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	return body, resp.StatusCode, nil
}
