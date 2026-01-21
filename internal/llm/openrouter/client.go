package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/llm"
)

type Config struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout time.Duration
}

type Client struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
	logger  *zap.Logger
}

func New(cfg Config, logger *zap.Logger) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://openrouter.ai/api/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "deepseek/deepseek-chat"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	return &Client{
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		baseURL: cfg.BaseURL,
		client:  &http.Client{Timeout: cfg.Timeout},
		logger:  logger,
	}
}

type openRouterResponse struct {
	llm.ChatResponse
	Error *apiError `json:"error,omitempty"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

func (c *Client) CompleteWithSystem(ctx context.Context, system, prompt string) (string, error) {
	req := llm.NewChatRequest(c.model, system, prompt)

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("HTTP-Referer", "https://github.com/kitbuilder587/fintech-bot")
	httpReq.Header.Set("X-Title", "Fintech Research Bot")

	respBody, statusCode, err := llm.DoRequest(c.client, httpReq)
	if err != nil {
		return "", err
	}

	if statusCode != http.StatusOK {
		return "", llm.HandleHTTPError(statusCode, respBody, c.logger, "openrouter")
	}

	var chatResp openRouterResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("%w: %s", llm.ErrRequestFailed, chatResp.Error.Message)
	}

	return llm.ExtractContent(&chatResp.ChatResponse)
}
