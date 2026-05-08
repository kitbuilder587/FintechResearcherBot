package eval

import "testing"

func TestAssessAnswer(t *testing.T) {
	tests := []struct {
		name          string
		answer        string
		sourceCount   int
		wantValidity  float64
		wantCoverage  float64
		wantUncited   int
		wantCitations int
	}{
		{
			name:          "valid citations and cited numeric",
			answer:        "Revenue grew by 12% [S1]. The market expanded [S2].",
			sourceCount:   2,
			wantValidity:  1,
			wantCoverage:  1,
			wantUncited:   0,
			wantCitations: 2,
		},
		{
			name:          "invalid citation index",
			answer:        "Revenue grew by 12% [S3].",
			sourceCount:   2,
			wantValidity:  0,
			wantCoverage:  1,
			wantUncited:   0,
			wantCitations: 1,
		},
		{
			name:          "uncited numeric",
			answer:        "Revenue grew by 12%.",
			sourceCount:   1,
			wantValidity:  1,
			wantCoverage:  0,
			wantUncited:   1,
			wantCitations: 0,
		},
		{
			name:          "no citations and no numerics is neutral",
			answer:        "The report describes market trends.",
			sourceCount:   1,
			wantValidity:  1,
			wantCoverage:  1,
			wantUncited:   0,
			wantCitations: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssessAnswer(tt.answer, tt.sourceCount)
			if got.CitationValidity != tt.wantValidity {
				t.Fatalf("CitationValidity = %v, want %v", got.CitationValidity, tt.wantValidity)
			}
			if got.CitationCoverage != tt.wantCoverage {
				t.Fatalf("CitationCoverage = %v, want %v", got.CitationCoverage, tt.wantCoverage)
			}
			if got.UncitedNumerics != tt.wantUncited {
				t.Fatalf("UncitedNumerics = %v, want %v", got.UncitedNumerics, tt.wantUncited)
			}
			if got.CitationsTotal != tt.wantCitations {
				t.Fatalf("CitationsTotal = %v, want %v", got.CitationsTotal, tt.wantCitations)
			}
		})
	}
}
