package config

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
)

var (
	ErrMissingToken    = errors.New("TELEGRAM_BOT_TOKEN is required")
	ErrMissingDB       = errors.New("DATABASE_URL is required")
	ErrInvalidStrategy = errors.New("invalid default strategy")
	ErrInvalidSearch   = errors.New("invalid search provider")
)

type Config struct {
	Telegram        TelegramConfig
	Database        DatabaseConfig
	LLM             LLMConfig
	Search          SearchConfig
	Log             LogConfig
	Timeouts        TimeoutConfig
	Cache           CacheConfig
	RateLimit       RateLimitConfig
	DefaultStrategy string
}

type TelegramConfig struct {
	Token string
}

type DatabaseConfig struct {
	URL string
}

type LLMConfig struct {
	Provider   string
	OpenRouter OpenRouterConfig
	GigaChat   GigaChatConfig
}

type OpenRouterConfig struct {
	APIKey  string
	Model   string
	BaseURL string
}

type GigaChatConfig struct {
	AuthKey      string
	ClientID     string
	ClientSecret string
	Scope        string
	AuthURL      string
	BaseURL      string
}

type TavilyConfig struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

type SearXNGConfig struct {
	BaseURL string
	Timeout time.Duration
}

type SearchConfig struct {
	Provider string
	Tavily   TavilyConfig
	SearXNG  SearXNGConfig
}

type LogConfig struct {
	Level string
}

type TimeoutConfig struct {
	Source time.Duration
	Total  time.Duration
}

type CacheConfig struct {
	Type string
	TTL  time.Duration
}

type RateLimitConfig struct {
	RequestsPerMinute int
}

func Load() (*Config, error) {
	cfg := &Config{
		Telegram: TelegramConfig{
			Token: os.Getenv("TELEGRAM_BOT_TOKEN"),
		},
		Database: DatabaseConfig{
			URL: os.Getenv("DATABASE_URL"),
		},
		LLM: LLMConfig{
			Provider: getEnvOrDefault("LLM_PROVIDER", "mock"),
			OpenRouter: OpenRouterConfig{
				APIKey:  os.Getenv("OPENROUTER_API_KEY"),
				Model:   getEnvOrDefault("OPENROUTER_MODEL", "deepseek/deepseek-chat"),
				BaseURL: getEnvOrDefaultAny([]string{"OPENROUTER_BASE_URL", "OPENROUTER_API_URL"}, "https://openrouter.ai/api/v1"),
			},
			GigaChat: GigaChatConfig{
				AuthKey:      os.Getenv("GIGACHAT_AUTH_KEY"),
				ClientID:     os.Getenv("GIGACHAT_CLIENT_ID"),
				ClientSecret: os.Getenv("GIGACHAT_CLIENT_SECRET"),
				Scope:        getEnvOrDefault("GIGACHAT_SCOPE", "GIGACHAT_API_PERS"),
				AuthURL:      getEnvOrDefault("GIGACHAT_AUTH_URL", "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"),
				BaseURL:      getEnvOrDefault("GIGACHAT_BASE_URL", "https://gigachat.devices.sberbank.ru/api/v1"),
			},
		},
		Search: SearchConfig{
			Provider: getEnvOrDefault("SEARCH_PROVIDER", "searxng"),
			Tavily: TavilyConfig{
				APIKey:  os.Getenv("TAVILY_API_KEY"),
				BaseURL: getEnvOrDefault("TAVILY_BASE_URL", "https://api.tavily.com"),
				Timeout: time.Duration(getEnvIntOrDefault("TAVILY_TIMEOUT_SEC", 30)) * time.Second,
			},
			SearXNG: SearXNGConfig{
				BaseURL: getEnvOrDefault("SEARXNG_BASE_URL", "http://searxng:8080"),
				Timeout: time.Duration(getEnvIntOrDefault("SEARXNG_TIMEOUT_SEC", 30)) * time.Second,
			},
		},
		Log: LogConfig{
			Level: getEnvOrDefault("LOG_LEVEL", "info"),
		},
		Timeouts: TimeoutConfig{
			Source: time.Duration(getEnvIntOrDefault("SOURCE_TIMEOUT_SEC", 15)) * time.Second,
			Total:  time.Duration(getEnvIntOrDefault("TOTAL_TIMEOUT_SEC", 60)) * time.Second,
		},
		Cache: CacheConfig{
			Type: getEnvOrDefault("CACHE_TYPE", "memory"),
			TTL:  time.Duration(getEnvIntOrDefault("CACHE_TTL_SEC", 3600)) * time.Second,
		},
		RateLimit: RateLimitConfig{
			RequestsPerMinute: getEnvIntOrDefault("RATE_LIMIT_PER_MINUTE", 10),
		},
		DefaultStrategy: getEnvOrDefault("DEFAULT_STRATEGY", "standard"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Telegram.Token == "" {
		return ErrMissingToken
	}
	if c.Database.URL == "" {
		return ErrMissingDB
	}
	if !domain.StrategyType(c.DefaultStrategy).IsValid() {
		return ErrInvalidStrategy
	}
	if c.Search.Provider != "" && c.Search.Provider != "searxng" && c.Search.Provider != "tavily" {
		return ErrInvalidSearch
	}
	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvOrDefaultAny(keys []string, defaultValue string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
