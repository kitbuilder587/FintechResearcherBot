package gigachat

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/llm"
)

type Config struct {
	AuthKey      string // готовый ключ авторизации (предпочтительно)
	ClientID     string // альтернатива: будет base64(id:secret)
	ClientSecret string
	Scope        string
	AuthURL      string
	BaseURL      string
	Timeout      time.Duration
}

type Client struct {
	authKey string
	scope   string
	authURL string
	baseURL string
	client  *http.Client
	logger  *zap.Logger

	mu          sync.RWMutex
	accessToken string
	tokenExpiry time.Time
}

func New(cfg Config, logger *zap.Logger) *Client {
	if cfg.AuthURL == "" {
		cfg.AuthURL = "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://gigachat.devices.sberbank.ru/api/v1"
	}
	if cfg.Scope == "" {
		cfg.Scope = "GIGACHAT_API_PERS"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	// У Сбера самоподписанный сертификат, приходится отключать проверку
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	authKey := cfg.AuthKey
	if authKey == "" && cfg.ClientID != "" && cfg.ClientSecret != "" {
		authKey = base64.StdEncoding.EncodeToString([]byte(cfg.ClientID + ":" + cfg.ClientSecret))
	}

	return &Client{
		authKey: authKey,
		scope:   cfg.Scope,
		authURL: cfg.AuthURL,
		baseURL: cfg.BaseURL,
		client:  &http.Client{Timeout: cfg.Timeout, Transport: transport},
		logger:  logger,
	}
}

type authResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   int64  `json:"expires_at"`
}

func (c *Client) CompleteWithSystem(ctx context.Context, system, prompt string) (string, error) {
	return c.completeWithRetry(ctx, system, prompt, false)
}

func (c *Client) completeWithRetry(ctx context.Context, system, prompt string, isRetry bool) (string, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return "", err
	}

	req := llm.NewChatRequest("GigaChat", system, prompt)

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	respBody, statusCode, err := llm.DoRequest(c.client, httpReq)
	if err != nil {
		return "", err
	}

	// при 401 пробуем обновить токен один раз
	if statusCode == http.StatusUnauthorized {
		if isRetry {
			return "", llm.ErrAuthFailed
		}
		c.invalidateToken()
		_, err = c.getToken(ctx)
		if err != nil {
			return "", llm.ErrAuthFailed
		}
		return c.completeWithRetry(ctx, system, prompt, true)
	}

	if statusCode != http.StatusOK {
		return "", llm.HandleHTTPError(statusCode, respBody, c.logger, "gigachat")
	}

	chatResp, err := llm.ParseChatResponse(respBody)
	if err != nil {
		return "", err
	}

	return llm.ExtractContent(chatResp)
}

func (c *Client) getToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry.Add(-5*time.Minute)) {
		token := c.accessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	return c.refreshToken(ctx)
}

func (c *Client) refreshToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// double-check после захвата лока
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry.Add(-5*time.Minute)) {
		return c.accessToken, nil
	}

	data := url.Values{}
	data.Set("scope", c.scope)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.authURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("create auth request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Basic "+c.authKey)
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("RqUID", uuid.New().String()) // Сбер требует уникальный id запроса

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("%w: %v", llm.ErrAuthFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("gigachat auth failed",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return "", llm.ErrAuthFailed
	}

	var authResp authResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", fmt.Errorf("decode auth response: %w", err)
	}

	c.accessToken = authResp.AccessToken
	c.tokenExpiry = time.UnixMilli(authResp.ExpiresAt)

	c.logger.Debug("gigachat token refreshed",
		zap.Time("expires", c.tokenExpiry),
	)

	return c.accessToken, nil
}

func (c *Client) invalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = ""
	c.tokenExpiry = time.Time{}
}
