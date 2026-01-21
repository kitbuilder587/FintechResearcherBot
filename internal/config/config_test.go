package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr error
	}{
		{
			name: "valid config",
			envVars: map[string]string{
				"TELEGRAM_BOT_TOKEN": "test_token",
				"DATABASE_URL":       "postgres://localhost:5432/test",
			},
			wantErr: nil,
		},
		{
			name: "missing telegram token",
			envVars: map[string]string{
				"DATABASE_URL": "postgres://localhost:5432/test",
			},
			wantErr: ErrMissingToken,
		},
		{
			name: "missing database url",
			envVars: map[string]string{
				"TELEGRAM_BOT_TOKEN": "test_token",
			},
			wantErr: ErrMissingDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnvVars()

			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			defer clearEnvVars()

			cfg, err := Load()

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Load() unexpected error = %v", err)
				return
			}

			if cfg == nil {
				t.Error("Load() returned nil config")
			}
		})
	}
}

func TestDefaults(t *testing.T) {
	clearEnvVars()
	os.Setenv("TELEGRAM_BOT_TOKEN", "test_token")
	os.Setenv("DATABASE_URL", "postgres://localhost:5432/test")
	defer clearEnvVars()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %v, want %v", cfg.Log.Level, "info")
	}
	if cfg.Timeouts.Source.Seconds() != 15 {
		t.Errorf("Timeouts.Source = %v, want 15s", cfg.Timeouts.Source)
	}
	if cfg.Timeouts.Total.Seconds() != 60 {
		t.Errorf("Timeouts.Total = %v, want 60s", cfg.Timeouts.Total)
	}
	if cfg.LLM.Provider != "mock" {
		t.Errorf("LLM.Provider = %v, want mock", cfg.LLM.Provider)
	}
}

func TestGetEnvIntOrDefault(t *testing.T) {
	tests := []struct {
		name       string
		envValue   string
		defaultVal int
		want       int
	}{
		{"valid int", "42", 10, 42},
		{"empty string", "", 10, 10},
		{"invalid int", "abc", 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TEST_INT", tt.envValue)
			defer os.Unsetenv("TEST_INT")

			got := getEnvIntOrDefault("TEST_INT", tt.defaultVal)
			if got != tt.want {
				t.Errorf("getEnvIntOrDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultStrategy(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{
			name:     "default when not set",
			envValue: "",
			want:     "standard",
		},
		{
			name:     "quick strategy from env",
			envValue: "quick",
			want:     "quick",
		},
		{
			name:     "deep strategy from env",
			envValue: "deep",
			want:     "deep",
		},
		{
			name:     "standard strategy from env",
			envValue: "standard",
			want:     "standard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnvVars()
			os.Setenv("TELEGRAM_BOT_TOKEN", "test_token")
			os.Setenv("DATABASE_URL", "postgres://localhost:5432/test")
			if tt.envValue != "" {
				os.Setenv("DEFAULT_STRATEGY", tt.envValue)
			}
			defer clearEnvVars()

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.DefaultStrategy != tt.want {
				t.Errorf("DefaultStrategy = %v, want %v", cfg.DefaultStrategy, tt.want)
			}
		})
	}
}

func TestValidate_InvalidStrategy(t *testing.T) {
	cfg := &Config{
		Telegram: TelegramConfig{
			Token: "test_token",
		},
		Database: DatabaseConfig{
			URL: "postgres://localhost:5432/test",
		},
		DefaultStrategy: "invalid",
	}

	err := cfg.Validate()
	if err != ErrInvalidStrategy {
		t.Errorf("Validate() error = %v, want %v", err, ErrInvalidStrategy)
	}
}

func TestValidate_ValidStrategies(t *testing.T) {
	strategies := []string{"quick", "standard", "deep"}

	for _, strategy := range strategies {
		t.Run(strategy, func(t *testing.T) {
			cfg := &Config{
				Telegram: TelegramConfig{
					Token: "test_token",
				},
				Database: DatabaseConfig{
					URL: "postgres://localhost:5432/test",
				},
				DefaultStrategy: strategy,
			}

			err := cfg.Validate()
			if err != nil {
				t.Errorf("Validate() error = %v for strategy %s", err, strategy)
			}
		})
	}
}

func clearEnvVars() {
	envVars := []string{
		"TELEGRAM_BOT_TOKEN",
		"DATABASE_URL",
		"LLM_PROVIDER",
		"OPENROUTER_API_KEY",
		"OPENROUTER_MODEL",
		"GIGACHAT_CLIENT_ID",
		"GIGACHAT_CLIENT_SECRET",
		"TAVILY_API_KEY",
		"LOG_LEVEL",
		"SOURCE_TIMEOUT_SEC",
		"TOTAL_TIMEOUT_SEC",
		"CACHE_TYPE",
		"CACHE_TTL_SEC",
		"DEFAULT_STRATEGY",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}
}
