package telegram

import (
	"strings"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
)

// /quick, /deep, /research -> соответствующая стратегия
// обычный текст -> defaultStrategy
func ParseQueryCommand(text string, defaultStrategy domain.Strategy) (question string, strategy domain.Strategy) {
	text = strings.TrimSpace(text)

	if text == "" {
		return "", defaultStrategy
	}

	if !strings.HasPrefix(text, "/") {
		return text, defaultStrategy
	}

	parts := strings.SplitN(text, " ", 2)
	command := strings.ToLower(parts[0])

	var rest string
	if len(parts) > 1 {
		rest = normalizeSpaces(parts[1])
	}

	switch command {
	case "/quick":
		return rest, domain.QuickStrategy()
	case "/deep":
		return rest, domain.DeepStrategy()
	case "/research":
		return rest, domain.StandardStrategy()
	default:
		return text, defaultStrategy
	}
}

func normalizeSpaces(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
