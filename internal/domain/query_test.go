package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestQueryRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		query   QueryRequest
		wantErr error
	}{
		{"ok", QueryRequest{UserID: 123, Text: "What is Go?", Strategy: StandardStrategy()}, nil},
		{"empty", QueryRequest{UserID: 123, Text: "", Strategy: StandardStrategy()}, ErrEmptyQuery},
		{"whitespace", QueryRequest{UserID: 123, Text: "   ", Strategy: StandardStrategy()}, ErrEmptyQuery},
		{"max len", QueryRequest{UserID: 123, Text: strings.Repeat("a", MaxQueryLength), Strategy: StandardStrategy()}, nil},
		{"too long", QueryRequest{UserID: 123, Text: strings.Repeat("a", MaxQueryLength+1), Strategy: StandardStrategy()}, ErrQueryTooLong},
		{"newlines", QueryRequest{UserID: 123, Text: "Hello\nWorld", Strategy: StandardStrategy()}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.query.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("QueryRequest.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// валидация стратегий
func TestQueryRequest_Validate_WithStrategy(t *testing.T) {
	tests := []struct {
		name    string
		query   QueryRequest
		wantErr error
	}{
		{"standard", QueryRequest{UserID: 123, Text: "What is Go?", Strategy: StandardStrategy()}, nil},
		{"quick", QueryRequest{UserID: 123, Text: "What is Go?", Strategy: QuickStrategy()}, nil},
		{"deep", QueryRequest{UserID: 123, Text: "What is Go?", Strategy: DeepStrategy()}, nil},
		{"invalid type", QueryRequest{UserID: 123, Text: "What is Go?", Strategy: Strategy{Type: "invalid", MaxQueries: 1, MaxResults: 5, MaxAnalysisIterations: 1, TimeoutSeconds: 30}}, ErrInvalidStrategyType},
		{"invalid params", QueryRequest{UserID: 123, Text: "What is Go?", Strategy: Strategy{Type: StrategyQuick, MaxQueries: 0, MaxResults: 5, MaxAnalysisIterations: 1, TimeoutSeconds: 30}}, ErrInvalidMaxQueries},
		{"empty + valid strategy", QueryRequest{UserID: 123, Text: "", Strategy: StandardStrategy()}, ErrEmptyQuery},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.query.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("QueryRequest.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQueryRequest_Sanitize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no trim", "Hello World", "Hello World"},
		{"trim leading", "   Hello World", "Hello World"},
		{"trim trailing", "Hello World   ", "Hello World"},
		{"trim both", "   Hello World   ", "Hello World"},
		{"trim tabs", "\t\tHello World\t\t", "Hello World"},
		{"trim newlines", "\n\nHello World\n\n", "Hello World"},
		{"preserve internal", "  Hello   World  ", "Hello   World"},
		{"empty", "   ", ""},
		{"truncate", strings.Repeat("a", MaxQueryLength+100), strings.Repeat("a", MaxQueryLength)},
		{"truncate+trim", "  " + strings.Repeat("a", MaxQueryLength+100) + "  ", strings.Repeat("a", MaxQueryLength)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := QueryRequest{Text: tt.input}
			q.Sanitize()
			if q.Text != tt.expected {
				t.Errorf("QueryRequest.Sanitize() = %q, want %q", q.Text, tt.expected)
			}
		})
	}
}
