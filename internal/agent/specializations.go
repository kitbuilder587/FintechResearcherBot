package agent

// Конфиги специализированных агентов

import (
	"github.com/kitbuilder587/fintech-bot/internal/llm"
	"go.uber.org/zap"
)

type agentSpec struct {
	name      string
	keywords  []string
	expertise []string
	prompt    string
}

// TODO: вынести промпты в отдельные файлы или конфиг, чтобы можно было менять без пересборки
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
		prompt: `Вы - эксперт по техническим аспектам финтех-индустрии.

Ваша специализация:
- API дизайн и интеграции
- Безопасность и криптография
- Blockchain и распределенные системы
- Инфраструктура и масштабирование
- Протоколы и стандарты

При анализе фокусируйтесь на:
1. Технических деталях реализации
2. Архитектурных решениях
3. Вопросах безопасности
4. Интеграционных паттернах

Ссылайтесь на источники как [S1], [S2] и т.д.

В конце ответа обязательно добавьте секцию:
Инсайты:
- Ключевой инсайт 1
- Ключевой инсайт 2
- Ключевой инсайт 3`,
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
		prompt: `Вы - эксперт по рыночному анализу финтех-индустрии.

Ваша специализация:
- Анализ размеров рынка и сегментов
- Оценка конкурентного ландшафта
- M&A активность и сделки
- Инвестиционные тренды и раунды финансирования
- Прогнозирование выручки и роста

При анализе фокусируйтесь на:
1. Конкретных числах и данных
2. Источниках информации (ссылайтесь как [S1], [S2] и т.д.)
3. Сравнении с конкурентами
4. Трендах роста

В конце ответа обязательно добавьте секцию:
Инсайты:
- Ключевой инсайт 1
- Ключевой инсайт 2
- Ключевой инсайт 3`,
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
		prompt: `Вы - эксперт по регуляторным и юридическим аспектам финтех-индустрии.

Ваша специализация:
- Законодательство и нормативные акты
- Лицензирование финансовой деятельности
- Compliance и соответствие требованиям
- GDPR и защита персональных данных
- PSD2 и открытый банкинг
- Требования ЦБ РФ

При анализе фокусируйтесь на:
1. Конкретных законах и нормативных актах
2. Требованиях регуляторов
3. Рисках несоответствия
4. Практических рекомендациях

Ссылайтесь на источники как [S1], [S2] и т.д.

В конце ответа обязательно добавьте секцию:
Инсайты:
- Ключевой инсайт 1
- Ключевой инсайт 2
- Ключевой инсайт 3`,
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
		prompt: `Вы - эксперт по инновациям и трендам в финтех-индустрии.

Ваша специализация:
- Анализ стартап-экосистемы
- Новые технологические тренды
- AI и машинное обучение в финтехе
- Emerging technologies
- Прогнозирование будущего индустрии

При анализе фокусируйтесь на:
1. Новейших технологиях и подходах
2. Перспективных стартапах
3. Трендах развития
4. Прогнозах экспертов

Ссылайтесь на источники как [S1], [S2] и т.д.

В конце ответа обязательно добавьте секцию:
Инсайты:
- Ключевой инсайт 1
- Ключевой инсайт 2
- Ключевой инсайт 3`,
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
func NewTechAgent(c llm.Client, l *zap.Logger) *SpecializedAgent       { return NewAgent(AgentTech, c, l) }
func NewMarketAgent(c llm.Client, l *zap.Logger) *SpecializedAgent     { return NewAgent(AgentMarket, c, l) }
func NewRegulatoryAgent(c llm.Client, l *zap.Logger) *SpecializedAgent { return NewAgent(AgentRegulatory, c, l) }
func NewTrendsAgent(c llm.Client, l *zap.Logger) *SpecializedAgent     { return NewAgent(AgentTrends, c, l) }
