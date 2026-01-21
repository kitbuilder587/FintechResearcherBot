package telegram

import (
	"fmt"
	"html"
	"strings"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
)

func FormatSourcesList(sources []domain.Source) string {
	var sb strings.Builder
	sb.WriteString("<b>Ваши источники:</b>\n\n")

	for i, s := range sources {
		trustIcon := getTrustIcon(s.TrustLevel)
		sb.WriteString(fmt.Sprintf("%d. %s %s\n   %s [%s]\n\n",
			i+1,
			trustIcon,
			html.EscapeString(s.Name),
			html.EscapeString(s.Domain()),
			s.TrustLevel,
		))
	}

	sb.WriteString(fmt.Sprintf("Всего: %d", len(sources)))
	return sb.String()
}

func FormatQueryResponse(resp *domain.QueryResponse) string {
	var sb strings.Builder
	sb.WriteString(html.EscapeString(resp.Text))

	if len(resp.Sources) > 0 {
		sb.WriteString("\n\n━━━━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString("<b>Источники:</b>\n")

		for _, src := range resp.Sources {
			trustIcon := getTrustIcon(src.TrustLevel)
			escapedURL := html.EscapeString(src.URL)
			sb.WriteString(fmt.Sprintf("%s %s %s\n   <a href=\"%s\">%s</a> [%s]\n",
				src.Marker,
				trustIcon,
				html.EscapeString(src.Title),
				escapedURL,
				html.EscapeString(truncateURL(src.URL, 50)),
				src.TrustLevel,
			))
		}
	}

	return sb.String()
}

func SplitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var messages []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			messages = append(messages, text)
			break
		}

		splitPoint := findSafeSplitPoint(text, maxLen)
		if splitPoint <= 0 || splitPoint > len(text) {
			splitPoint = maxLen
		}

		messages = append(messages, text[:splitPoint])
		text = text[splitPoint:]
	}

	return messages
}

func findSafeSplitPoint(text string, maxLen int) int {
	// ищем пробел или перевод строки, не ломая HTML-теги
	for i := maxLen - 1; i > maxLen/2; i-- {
		if i >= len(text) {
			continue
		}
		if isInsideHTMLTag(text, i) {
			continue
		}

		if text[i] == '\n' || text[i] == ' ' {
			return i + 1
		}
	}

	// внутри тега - ищем конец
	if maxLen < len(text) && isInsideHTMLTag(text, maxLen) {
		for i := maxLen; i < len(text); i++ {
			if text[i] == '>' {
				for j := i + 1; j < len(text) && j < i+50; j++ {
					if text[j] == '\n' || text[j] == ' ' {
						return j + 1
					}
				}
				return i + 1
			}
		}
	}

	for i := maxLen - 1; i > 0; i-- {
		if text[i] == ' ' || text[i] == '\n' {
			return i + 1
		}
	}

	return maxLen
}

func isInsideHTMLTag(text string, pos int) bool {
	if pos >= len(text) || pos < 0 {
		return false
	}
	for i := pos; i >= 0; i-- {
		if text[i] == '>' {
			return false
		}
		if text[i] == '<' {
			return true
		}
	}
	return false
}

func getTrustIcon(level domain.TrustLevel) string {
	switch level {
	case domain.TrustHigh:
		return "●"
	case domain.TrustMedium:
		return "◐"
	case domain.TrustLow:
		return "○"
	default:
		return "○"
	}
}

func truncateURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return url[:maxLen-3] + "..."
}
