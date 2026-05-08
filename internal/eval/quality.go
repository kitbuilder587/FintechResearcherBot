package eval

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	citationRe = regexp.MustCompile(`\[S(\d+)\]`)
	numericRe  = regexp.MustCompile(`(?i)(\d{1,2}[./-]\d{1,2}[./-]\d{2,4}|\d{4}|\d+(?:[.,]\d+)?\s?(?:%|bps?|bp|млн|млрд|тыс|руб|₽|\$|€|billion|million))`)
)

type AnswerQuality struct {
	CitationValidity float64
	CitationCoverage float64
	UncitedNumerics  int
	CitationsTotal   int
	CitationsValid   int
}

func AssessAnswer(answer string, sourceCount int) AnswerQuality {
	citationsTotal, citationsValid := countCitations(answer, sourceCount)
	uncitedNumerics, numericClaims := countUncitedNumerics(answer)

	validity := 1.0
	if citationsTotal > 0 {
		validity = float64(citationsValid) / float64(citationsTotal)
	}

	coverage := 1.0
	if numericClaims > 0 {
		coverage = float64(numericClaims-uncitedNumerics) / float64(numericClaims)
	}

	return AnswerQuality{
		CitationValidity: validity,
		CitationCoverage: coverage,
		UncitedNumerics:  uncitedNumerics,
		CitationsTotal:   citationsTotal,
		CitationsValid:   citationsValid,
	}
}

func countCitations(answer string, sourceCount int) (int, int) {
	matches := citationRe.FindAllStringSubmatch(answer, -1)
	valid := 0
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		idx, err := strconv.Atoi(match[1])
		if err == nil && idx >= 1 && idx <= sourceCount {
			valid++
		}
	}
	return len(matches), valid
}

func countUncitedNumerics(answer string) (int, int) {
	matches := numericRe.FindAllStringIndex(answer, -1)
	uncited := 0
	for _, match := range matches {
		end := match[1]
		windowEnd := end + 80
		if windowEnd > len(answer) {
			windowEnd = len(answer)
		}
		if !strings.Contains(citationRe.FindString(answer[end:windowEnd]), "[S") {
			uncited++
		}
	}
	return uncited, len(matches)
}
