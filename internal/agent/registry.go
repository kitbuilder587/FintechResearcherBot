package agent

import (
	"github.com/kitbuilder587/fintech-bot/internal/llm"
	"go.uber.org/zap"
)

// NewAllAgents собирает всех специализированных агентов
func NewAllAgents(llmClient llm.Client, logger *zap.Logger) []Agent {
	if logger == nil {
		logger = zap.NewNop()
	}

	return []Agent{
		NewMarketAgent(llmClient, logger),
		NewRegulatoryAgent(llmClient, logger),
		NewTechAgent(llmClient, logger),
		NewTrendsAgent(llmClient, logger),
	}
}
