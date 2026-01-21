package telegram

import (
	"testing"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
)

func TestParseQueryCommand(t *testing.T) {
	tests := []struct {
		name             string
		text             string
		defaultStrategy  domain.Strategy
		wantQuestion     string
		wantStrategyType domain.StrategyType
	}{
		{
			name:             "/quick command",
			text:             "/quick тест",
			defaultStrategy:  domain.StandardStrategy(),
			wantQuestion:     "тест",
			wantStrategyType: domain.StrategyQuick,
		},
		{
			name:             "/deep command",
			text:             "/deep тест",
			defaultStrategy:  domain.StandardStrategy(),
			wantQuestion:     "тест",
			wantStrategyType: domain.StrategyDeep,
		},
		{
			name:             "/research command",
			text:             "/research тест",
			defaultStrategy:  domain.QuickStrategy(),
			wantQuestion:     "тест",
			wantStrategyType: domain.StrategyStandard,
		},
		{
			name:             "plain text with standard default",
			text:             "просто текст",
			defaultStrategy:  domain.StandardStrategy(),
			wantQuestion:     "просто текст",
			wantStrategyType: domain.StrategyStandard,
		},
		{
			name:             "plain text with deep default",
			text:             "просто текст",
			defaultStrategy:  domain.DeepStrategy(),
			wantQuestion:     "просто текст",
			wantStrategyType: domain.StrategyDeep,
		},
		{
			name:             "/quick without question",
			text:             "/quick",
			defaultStrategy:  domain.StandardStrategy(),
			wantQuestion:     "",
			wantStrategyType: domain.StrategyQuick,
		},
		{
			name:             "empty string",
			text:             "",
			defaultStrategy:  domain.StandardStrategy(),
			wantQuestion:     "",
			wantStrategyType: domain.StrategyStandard,
		},
		{
			name:             "/quick with extra spaces",
			text:             "/quick   много пробелов  ",
			defaultStrategy:  domain.StandardStrategy(),
			wantQuestion:     "много пробелов",
			wantStrategyType: domain.StrategyQuick,
		},
		{
			name:             "/QUICK uppercase (case insensitive)",
			text:             "/QUICK тест",
			defaultStrategy:  domain.StandardStrategy(),
			wantQuestion:     "тест",
			wantStrategyType: domain.StrategyQuick,
		},
		{
			name:             "unknown command treated as plain text",
			text:             "/unknown тест",
			defaultStrategy:  domain.StandardStrategy(),
			wantQuestion:     "/unknown тест",
			wantStrategyType: domain.StrategyStandard,
		},
		{
			name:             "/Deep mixed case",
			text:             "/Deep тест",
			defaultStrategy:  domain.StandardStrategy(),
			wantQuestion:     "тест",
			wantStrategyType: domain.StrategyDeep,
		},
		{
			name:             "/RESEARCH uppercase",
			text:             "/RESEARCH тест",
			defaultStrategy:  domain.QuickStrategy(),
			wantQuestion:     "тест",
			wantStrategyType: domain.StrategyStandard,
		},
		{
			name:             "whitespace only",
			text:             "   ",
			defaultStrategy:  domain.StandardStrategy(),
			wantQuestion:     "",
			wantStrategyType: domain.StrategyStandard,
		},
		{
			name:             "/quick with only spaces after",
			text:             "/quick   ",
			defaultStrategy:  domain.StandardStrategy(),
			wantQuestion:     "",
			wantStrategyType: domain.StrategyQuick,
		},
		{
			name:             "multiword question",
			text:             "/deep как работает искусственный интеллект",
			defaultStrategy:  domain.StandardStrategy(),
			wantQuestion:     "как работает искусственный интеллект",
			wantStrategyType: domain.StrategyDeep,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			question, strategy := ParseQueryCommand(tt.text, tt.defaultStrategy)

			if question != tt.wantQuestion {
				t.Errorf("ParseQueryCommand() question = %q, want %q", question, tt.wantQuestion)
			}

			if strategy.Type != tt.wantStrategyType {
				t.Errorf("ParseQueryCommand() strategy.Type = %v, want %v", strategy.Type, tt.wantStrategyType)
			}
		})
	}
}

func TestParseQueryCommand_StrategyParams(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		wantStrategy domain.Strategy
	}{
		{
			name:         "/quick returns QuickStrategy params",
			text:         "/quick test",
			wantStrategy: domain.QuickStrategy(),
		},
		{
			name:         "/deep returns DeepStrategy params",
			text:         "/deep test",
			wantStrategy: domain.DeepStrategy(),
		},
		{
			name:         "/research returns StandardStrategy params",
			text:         "/research test",
			wantStrategy: domain.StandardStrategy(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, strategy := ParseQueryCommand(tt.text, domain.StandardStrategy())

			if strategy.MaxQueries != tt.wantStrategy.MaxQueries {
				t.Errorf("Strategy.MaxQueries = %v, want %v", strategy.MaxQueries, tt.wantStrategy.MaxQueries)
			}
			if strategy.MaxResults != tt.wantStrategy.MaxResults {
				t.Errorf("Strategy.MaxResults = %v, want %v", strategy.MaxResults, tt.wantStrategy.MaxResults)
			}
			if strategy.MaxAnalysisIterations != tt.wantStrategy.MaxAnalysisIterations {
				t.Errorf("Strategy.MaxAnalysisIterations = %v, want %v", strategy.MaxAnalysisIterations, tt.wantStrategy.MaxAnalysisIterations)
			}
			if strategy.UseCritic != tt.wantStrategy.UseCritic {
				t.Errorf("Strategy.UseCritic = %v, want %v", strategy.UseCritic, tt.wantStrategy.UseCritic)
			}
			if strategy.TimeoutSeconds != tt.wantStrategy.TimeoutSeconds {
				t.Errorf("Strategy.TimeoutSeconds = %v, want %v", strategy.TimeoutSeconds, tt.wantStrategy.TimeoutSeconds)
			}
		})
	}
}
