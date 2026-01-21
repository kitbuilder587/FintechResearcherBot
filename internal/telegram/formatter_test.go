package telegram

import (
	"strings"
	"testing"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
)

func TestFormatSourcesList(t *testing.T) {
	sources := []domain.Source{
		{
			ID:         1,
			URL:        "https://example.com",
			Name:       "Example",
			TrustLevel: domain.TrustHigh,
		},
		{
			ID:         2,
			URL:        "https://test.org/path",
			Name:       "Test Site",
			TrustLevel: domain.TrustMedium,
		},
	}

	result := FormatSourcesList(sources)

	if !strings.Contains(result, "Example") {
		t.Error("FormatSourcesList() should contain source name")
	}
	if !strings.Contains(result, "high") {
		t.Error("FormatSourcesList() should contain trust level")
	}
	if !strings.Contains(result, "Всего: 2") {
		t.Error("FormatSourcesList() should contain total count")
	}
}

func TestFormatQueryResponse(t *testing.T) {
	resp := &domain.QueryResponse{
		Text: "This is the answer with [S1] reference.",
		Sources: []domain.SourceRef{
			{
				Marker:     "[S1]",
				Title:      "Source Title",
				URL:        "https://example.com/article",
				TrustLevel: domain.TrustHigh,
			},
		},
	}

	result := FormatQueryResponse(resp)

	if !strings.Contains(result, "This is the answer") {
		t.Error("FormatQueryResponse() should contain answer text")
	}
	if !strings.Contains(result, "Источники:") {
		t.Error("FormatQueryResponse() should contain sources section")
	}
	if !strings.Contains(result, "[S1]") {
		t.Error("FormatQueryResponse() should contain source marker")
	}
}

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		maxLen int
		want   int // number of parts
	}{
		{"short message", "Hello", 100, 1},
		{"exact length", "Hello", 5, 1},
		{"split needed", "Hello World Test", 7, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitMessage(tt.text, tt.maxLen)
			if len(got) != tt.want {
				t.Errorf("SplitMessage() parts = %v, want %v", len(got), tt.want)
			}
		})
	}
}

func TestSplitMessage_HTMLTags(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{
			name: "link tag",
			text: `Text before <a href="https://example.com/very/long/url">link text</a> text after`,
		},
		{
			name: "bold tag",
			text: `Some text <b>bold text here</b> more text`,
		},
		{
			name: "multiple tags",
			text: `<b>Title</b>\n<a href="https://example.com">Link</a>\nMore text here`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := SplitMessage(tt.text, 30)

			for i, part := range parts {
				openCount := strings.Count(part, "<")
				closeCount := strings.Count(part, ">")

				if openCount != closeCount {
					t.Errorf("Part %d has unbalanced tags (open=%d, close=%d): %q",
						i, openCount, closeCount, part)
				}
			}
		})
	}
}

func TestIsInsideHTMLTag(t *testing.T) {
	tests := []struct {
		text string
		pos  int
		want bool
	}{
		{`<a href="url">text</a>`, 5, true},   // inside <a href="...">
		{`<a href="url">text</a>`, 15, false}, // in "text"
		{`text <b>bold</b>`, 0, false},        // before any tag
		{`text <b>bold</b>`, 6, true},         // inside <b>
		{`text <b>bold</b>`, 9, false},        // in "bold"
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := isInsideHTMLTag(tt.text, tt.pos)
			if got != tt.want {
				t.Errorf("isInsideHTMLTag(%q, %d) = %v, want %v", tt.text, tt.pos, got, tt.want)
			}
		})
	}
}

func TestTruncateURL(t *testing.T) {
	tests := []struct {
		url    string
		maxLen int
		want   string
	}{
		{"https://example.com", 50, "https://example.com"},
		{"https://example.com/very/long/path", 20, "https://example.c..."},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := truncateURL(tt.url, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
