package domain

import "testing"

func TestTrustLevel_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		level TrustLevel
		want  bool
	}{
		{
			name:  "high is valid",
			level: TrustHigh,
			want:  true,
		},
		{
			name:  "medium is valid",
			level: TrustMedium,
			want:  true,
		},
		{
			name:  "low is valid",
			level: TrustLow,
			want:  true,
		},
		{
			name:  "empty is invalid",
			level: "",
			want:  false,
		},
		{
			name:  "random string is invalid",
			level: "invalid",
			want:  false,
		},
		{
			name:  "uppercase HIGH is invalid",
			level: "HIGH",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.IsValid(); got != tt.want {
				t.Errorf("TrustLevel.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrustLevel_String(t *testing.T) {
	tests := []struct {
		name  string
		level TrustLevel
		want  string
	}{
		{
			name:  "high",
			level: TrustHigh,
			want:  "high",
		},
		{
			name:  "medium",
			level: TrustMedium,
			want:  "medium",
		},
		{
			name:  "low",
			level: TrustLow,
			want:  "low",
		},
		{
			name:  "empty",
			level: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("TrustLevel.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
