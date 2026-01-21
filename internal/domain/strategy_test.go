package domain

import "testing"

func TestStrategyType_IsValid(t *testing.T) {
	tests := []struct {
		name         string
		strategyType StrategyType
		want         bool
	}{
		{
			name:         "quick is valid",
			strategyType: "quick",
			want:         true,
		},
		{
			name:         "standard is valid",
			strategyType: "standard",
			want:         true,
		},
		{
			name:         "deep is valid",
			strategyType: "deep",
			want:         true,
		},
		{
			name:         "empty is invalid",
			strategyType: "",
			want:         false,
		},
		{
			name:         "ultra is invalid",
			strategyType: "ultra",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.strategyType.IsValid(); got != tt.want {
				t.Errorf("StrategyType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStrategy_Validate(t *testing.T) {
	tests := []struct {
		name     string
		strategy Strategy
		wantErr  bool
	}{
		{
			name: "Quick strategy is valid",
			strategy: Strategy{
				Type:                  StrategyQuick,
				MaxQueries:            1,
				MaxResults:            5,
				MaxAnalysisIterations: 1,
				UseCritic:             false,
				TimeoutSeconds:        30,
			},
			wantErr: false,
		},
		{
			name: "zero queries is invalid",
			strategy: Strategy{
				Type:                  StrategyStandard,
				MaxQueries:            0,
				MaxResults:            5,
				MaxAnalysisIterations: 1,
				UseCritic:             true,
				TimeoutSeconds:        60,
			},
			wantErr: true,
		},
		{
			name: "negative results is invalid",
			strategy: Strategy{
				Type:                  StrategyStandard,
				MaxQueries:            1,
				MaxResults:            -1,
				MaxAnalysisIterations: 1,
				UseCritic:             true,
				TimeoutSeconds:        60,
			},
			wantErr: true,
		},
		{
			name: "too many queries is invalid",
			strategy: Strategy{
				Type:                  StrategyDeep,
				MaxQueries:            100,
				MaxResults:            5,
				MaxAnalysisIterations: 1,
				UseCritic:             true,
				TimeoutSeconds:        60,
			},
			wantErr: true,
		},
		{
			name: "too many results is invalid",
			strategy: Strategy{
				Type:                  StrategyDeep,
				MaxQueries:            5,
				MaxResults:            1000,
				MaxAnalysisIterations: 1,
				UseCritic:             true,
				TimeoutSeconds:        60,
			},
			wantErr: true,
		},
		{
			name: "invalid strategy type",
			strategy: Strategy{
				Type:                  "invalid",
				MaxQueries:            1,
				MaxResults:            5,
				MaxAnalysisIterations: 1,
				UseCritic:             false,
				TimeoutSeconds:        30,
			},
			wantErr: true,
		},
		{
			name: "zero analysis iterations is invalid",
			strategy: Strategy{
				Type:                  StrategyQuick,
				MaxQueries:            1,
				MaxResults:            5,
				MaxAnalysisIterations: 0,
				UseCritic:             false,
				TimeoutSeconds:        30,
			},
			wantErr: true,
		},
		{
			name: "zero timeout is invalid",
			strategy: Strategy{
				Type:                  StrategyQuick,
				MaxQueries:            1,
				MaxResults:            5,
				MaxAnalysisIterations: 1,
				UseCritic:             false,
				TimeoutSeconds:        0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.strategy.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Strategy.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQuickStrategy(t *testing.T) {
	s := QuickStrategy()

	if s.Type != StrategyQuick {
		t.Errorf("QuickStrategy().Type = %v, want %v", s.Type, StrategyQuick)
	}
	if s.MaxQueries != 1 {
		t.Errorf("QuickStrategy().MaxQueries = %v, want %v", s.MaxQueries, 1)
	}
	if s.MaxResults != 5 {
		t.Errorf("QuickStrategy().MaxResults = %v, want %v", s.MaxResults, 5)
	}
	if s.MaxAnalysisIterations != 1 {
		t.Errorf("QuickStrategy().MaxAnalysisIterations = %v, want %v", s.MaxAnalysisIterations, 1)
	}
	if s.UseCritic != false {
		t.Errorf("QuickStrategy().UseCritic = %v, want %v", s.UseCritic, false)
	}
	if s.TimeoutSeconds != 30 {
		t.Errorf("QuickStrategy().TimeoutSeconds = %v, want %v", s.TimeoutSeconds, 30)
	}

	if err := s.Validate(); err != nil {
		t.Errorf("QuickStrategy().Validate() = %v, want nil", err)
	}
}

func TestStandardStrategy(t *testing.T) {
	s := StandardStrategy()

	if s.Type != StrategyStandard {
		t.Errorf("StandardStrategy().Type = %v, want %v", s.Type, StrategyStandard)
	}
	if s.MaxQueries != 3 {
		t.Errorf("StandardStrategy().MaxQueries = %v, want %v", s.MaxQueries, 3)
	}
	if s.MaxResults != 15 {
		t.Errorf("StandardStrategy().MaxResults = %v, want %v", s.MaxResults, 15)
	}
	if s.MaxAnalysisIterations != 1 {
		t.Errorf("StandardStrategy().MaxAnalysisIterations = %v, want %v", s.MaxAnalysisIterations, 1)
	}
	if s.UseCritic != true {
		t.Errorf("StandardStrategy().UseCritic = %v, want %v", s.UseCritic, true)
	}
	if s.TimeoutSeconds != 60 {
		t.Errorf("StandardStrategy().TimeoutSeconds = %v, want %v", s.TimeoutSeconds, 60)
	}

	if err := s.Validate(); err != nil {
		t.Errorf("StandardStrategy().Validate() = %v, want nil", err)
	}
}

func TestDeepStrategy(t *testing.T) {
	s := DeepStrategy()

	if s.Type != StrategyDeep {
		t.Errorf("DeepStrategy().Type = %v, want %v", s.Type, StrategyDeep)
	}
	if s.MaxQueries != 5 {
		t.Errorf("DeepStrategy().MaxQueries = %v, want %v", s.MaxQueries, 5)
	}
	if s.MaxResults != 30 {
		t.Errorf("DeepStrategy().MaxResults = %v, want %v", s.MaxResults, 30)
	}
	if s.MaxAnalysisIterations != 3 {
		t.Errorf("DeepStrategy().MaxAnalysisIterations = %v, want %v", s.MaxAnalysisIterations, 3)
	}
	if s.UseCritic != true {
		t.Errorf("DeepStrategy().UseCritic = %v, want %v", s.UseCritic, true)
	}
	if s.TimeoutSeconds != 180 {
		t.Errorf("DeepStrategy().TimeoutSeconds = %v, want %v", s.TimeoutSeconds, 180)
	}

	if err := s.Validate(); err != nil {
		t.Errorf("DeepStrategy().Validate() = %v, want nil", err)
	}
}

func TestStrategyType_String(t *testing.T) {
	tests := []struct {
		name         string
		strategyType StrategyType
		want         string
	}{
		{
			name:         "quick",
			strategyType: StrategyQuick,
			want:         "quick",
		},
		{
			name:         "standard",
			strategyType: StrategyStandard,
			want:         "standard",
		},
		{
			name:         "deep",
			strategyType: StrategyDeep,
			want:         "deep",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.strategyType.String(); got != tt.want {
				t.Errorf("StrategyType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
