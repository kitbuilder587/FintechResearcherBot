package domain

import (
	"errors"
	"testing"
)

func TestCriticResult_HasCriticalIssues(t *testing.T) {
	tests := []struct {
		name   string
		result CriticResult
		want   bool
	}{
		{
			name: "empty issues list returns false",
			result: CriticResult{
				Approved: true,
				Issues:   []string{},
			},
			want: false,
		},
		{
			name: "nil issues list returns false",
			result: CriticResult{
				Approved: true,
				Issues:   nil,
			},
			want: false,
		},
		{
			name: "has issues returns true",
			result: CriticResult{
				Approved: false,
				Issues:   []string{"source not cited"},
			},
			want: true,
		},
		{
			name: "multiple issues returns true",
			result: CriticResult{
				Approved: false,
				Issues:   []string{"source not cited", "factual error"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasCriticalIssues(); got != tt.want {
				t.Errorf("CriticResult.HasCriticalIssues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCriticResult_NeedsRevision(t *testing.T) {
	tests := []struct {
		name   string
		result CriticResult
		want   bool
	}{
		{
			name: "approved with no issues returns false",
			result: CriticResult{
				Approved:    true,
				Issues:      []string{},
				Suggestions: []string{},
			},
			want: false,
		},
		{
			name: "not approved with issues returns true",
			result: CriticResult{
				Approved: false,
				Issues:   []string{"missing citation"},
			},
			want: true,
		},
		{
			name: "approved with suggestions returns true",
			result: CriticResult{
				Approved:    true,
				Issues:      []string{},
				Suggestions: []string{"add more details"},
			},
			want: true,
		},
		{
			name: "approved with issues returns true",
			result: CriticResult{
				Approved:    true,
				Issues:      []string{"minor formatting issue"},
				Suggestions: []string{},
			},
			want: true,
		},
		{
			name: "nil issues and suggestions returns false when approved",
			result: CriticResult{
				Approved:    true,
				Issues:      nil,
				Suggestions: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.NeedsRevision(); got != tt.want {
				t.Errorf("CriticResult.NeedsRevision() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCriticResult_NeedsRevision_StrictMode(t *testing.T) {
	tests := []struct {
		name       string
		result     CriticResult
		strictMode bool
		want       bool
	}{
		{
			name: "approved with issues in strict mode returns true",
			result: CriticResult{
				Approved:    true,
				Issues:      []string{"minor issue"},
				Suggestions: []string{},
			},
			strictMode: true,
			want:       true,
		},
		{
			name: "approved with no issues in strict mode returns false",
			result: CriticResult{
				Approved:    true,
				Issues:      []string{},
				Suggestions: []string{},
			},
			strictMode: true,
			want:       false,
		},
		{
			name: "approved with suggestions in strict mode returns true",
			result: CriticResult{
				Approved:    true,
				Issues:      []string{},
				Suggestions: []string{"could be better"},
			},
			strictMode: true,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.NeedsRevisionStrict(tt.strictMode); got != tt.want {
				t.Errorf("CriticResult.NeedsRevisionStrict() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCriticConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  CriticConfig
		wantErr error
	}{
		{
			name: "valid config with MaxRetries=3",
			config: CriticConfig{
				MaxRetries: 3,
				StrictMode: false,
			},
			wantErr: nil,
		},
		{
			name: "valid config with MaxRetries=0",
			config: CriticConfig{
				MaxRetries: 0,
				StrictMode: false,
			},
			wantErr: nil,
		},
		{
			name: "valid config with MaxRetries=10",
			config: CriticConfig{
				MaxRetries: 10,
				StrictMode: true,
			},
			wantErr: nil,
		},
		{
			name: "invalid config with negative MaxRetries",
			config: CriticConfig{
				MaxRetries: -1,
				StrictMode: false,
			},
			wantErr: ErrInvalidMaxRetries,
		},
		{
			name: "invalid config with MaxRetries > 10",
			config: CriticConfig{
				MaxRetries: 100,
				StrictMode: false,
			},
			wantErr: ErrMaxRetriesExceeded,
		},
		{
			name: "invalid config with MaxRetries=11",
			config: CriticConfig{
				MaxRetries: 11,
				StrictMode: false,
			},
			wantErr: ErrMaxRetriesExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("CriticConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
