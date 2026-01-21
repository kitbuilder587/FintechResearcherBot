package config

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
		want  zapcore.Level
	}{
		{"debug lowercase", "debug", zapcore.DebugLevel},
		{"debug uppercase", "DEBUG", zapcore.DebugLevel},
		{"info lowercase", "info", zapcore.InfoLevel},
		{"info uppercase", "INFO", zapcore.InfoLevel},
		{"warn lowercase", "warn", zapcore.WarnLevel},
		{"warning", "warning", zapcore.WarnLevel},
		{"error lowercase", "error", zapcore.ErrorLevel},
		{"error uppercase", "ERROR", zapcore.ErrorLevel},
		{"invalid string", "invalid", zapcore.InfoLevel},
		{"empty string", "", zapcore.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLogLevel(tt.level)
			if got != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name    string
		cfg     LogConfig
		wantErr bool
	}{
		{
			name:    "debug level",
			cfg:     LogConfig{Level: "debug"},
			wantErr: false,
		},
		{
			name:    "info level",
			cfg:     LogConfig{Level: "info"},
			wantErr: false,
		},
		{
			name:    "warn level",
			cfg:     LogConfig{Level: "warn"},
			wantErr: false,
		},
		{
			name:    "error level",
			cfg:     LogConfig{Level: "error"},
			wantErr: false,
		},
		{
			name:    "default level (empty)",
			cfg:     LogConfig{Level: ""},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLogger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && logger == nil {
				t.Error("NewLogger() returned nil logger")
			}
			if logger != nil {
				logger.Sync()
			}
		})
	}
}
