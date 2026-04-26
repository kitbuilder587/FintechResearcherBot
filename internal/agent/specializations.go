package agent

// Конфиги специализированных агентов

import (
	"github.com/kitbuilder587/fintech-bot/internal/llm"
	"github.com/kitbuilder587/fintech-bot/internal/prompts"
	"go.uber.org/zap"
)

type agentSpec struct {
	name      string
	keywords  []string
	expertise []string
	prompt    string
}

var specs = map[AgentType]agentSpec{
	AgentTech: {
		name: "tech-specialist",
		keywords: []string{
			"API", "integrat", "security", "blockchain", "infrastructure",
			"протокол", "интеграц",
		},
		expertise: []string{
			"API design and integration",
			"security architecture",
			"blockchain technology",
			"infrastructure planning",
			"protocol implementation",
		},
		prompt: prompts.Text("agent_tech_system.md"),
	},

	AgentMarket: {
		name: "market-analyst",
		keywords: []string{
			"market", "revenue", "valuation", "competitors", "growth", "investment", "funding",
			"рынк", "выручк", "конкурент",
		},
		expertise: []string{
			"market size analysis",
			"competitive landscape",
			"M&A activity",
			"investment trends",
			"revenue forecasting",
		},
		prompt: prompts.Text("agent_market_system.md"),
	},

	AgentRegulatory: {
		name: "regulatory-expert",
		keywords: []string{
			"regulation", "compliance", "license", "law", "legal", "GDPR", "PSD2",
			"ЦБ", "закон", "лицензия",
		},
		expertise: []string{
			"regulatory compliance",
			"licensing requirements",
			"legal frameworks",
			"GDPR and data protection",
			"PSD2 and open banking",
			"Central Bank regulations",
		},
		prompt: prompts.Text("agent_regulatory_system.md"),
	},

	AgentTrends: {
		name: "trends-analyst",
		keywords: []string{
			"trend", "startup", "innovation", "future", "emerging", "AI", "machine learning",
			"тренд", "стартап",
		},
		expertise: []string{
			"startup ecosystem",
			"innovation trends",
			"emerging technologies",
			"AI and machine learning",
			"future of fintech",
		},
		prompt: prompts.Text("agent_trends_system.md"),
	},
}

// SpecializedAgent - обертка над BaseAgent с конкретной специализацией
type SpecializedAgent struct{ *BaseAgent }

// NewAgent создает агента по типу. Возвращает nil если тип неизвестен.
func NewAgent(t AgentType, llmClient llm.Client, log *zap.Logger) *SpecializedAgent {
	spec, ok := specs[t]
	if !ok {
		return nil
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &SpecializedAgent{
		BaseAgent: NewBaseAgent(spec.name, spec.expertise, spec.keywords, spec.prompt, llmClient, log),
	}
}

// FIXME: legacy функции для обратной совместимости, потом убрать
func NewTechAgent(c llm.Client, l *zap.Logger) *SpecializedAgent { return NewAgent(AgentTech, c, l) }
func NewMarketAgent(c llm.Client, l *zap.Logger) *SpecializedAgent {
	return NewAgent(AgentMarket, c, l)
}
func NewRegulatoryAgent(c llm.Client, l *zap.Logger) *SpecializedAgent {
	return NewAgent(AgentRegulatory, c, l)
}
func NewTrendsAgent(c llm.Client, l *zap.Logger) *SpecializedAgent {
	return NewAgent(AgentTrends, c, l)
}
