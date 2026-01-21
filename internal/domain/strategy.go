package domain

import "errors"

var (
	ErrInvalidStrategyType   = errors.New("invalid strategy type")
	ErrInvalidMaxQueries     = errors.New("max queries must be between 1 and 10")
	ErrInvalidMaxResults     = errors.New("max results must be between 1 and 100")
	ErrInvalidAnalysisIter   = errors.New("max analysis iterations must be at least 1")
	ErrInvalidTimeoutSeconds = errors.New("timeout seconds must be at least 1")
)

type StrategyType string

const (
	StrategyQuick    StrategyType = "quick"
	StrategyStandard StrategyType = "standard"
	StrategyDeep     StrategyType = "deep"
)

func (s StrategyType) IsValid() bool {
	switch s {
	case StrategyQuick, StrategyStandard, StrategyDeep:
		return true
	}
	return false
}

func (s StrategyType) String() string { return string(s) }

// Strategy - конфиг стратегии исследования
type Strategy struct {
	Type                  StrategyType
	MaxQueries            int
	MaxResults            int
	MaxAnalysisIterations int
	UseCritic             bool
	TimeoutSeconds        int
}

// Validate проверяет что все поля в допустимых диапазонах.
// FIXME: магические числа, вынести в константы?
func (s Strategy) Validate() error {
	if !s.Type.IsValid() {
		return ErrInvalidStrategyType
	}
	if s.MaxQueries < 1 || s.MaxQueries > 10 {
		return ErrInvalidMaxQueries
	}
	if s.MaxResults < 1 || s.MaxResults > 100 {
		return ErrInvalidMaxResults
	}
	if s.MaxAnalysisIterations < 1 {
		return ErrInvalidAnalysisIter
	}
	if s.TimeoutSeconds < 1 {
		return ErrInvalidTimeoutSeconds
	}
	return nil
}

// Предустановленные стратегии

func QuickStrategy() Strategy {
	return Strategy{
		Type:                  StrategyQuick,
		MaxQueries:            1,
		MaxResults:            5,
		MaxAnalysisIterations: 1,
		UseCritic:             false,
		TimeoutSeconds:        30,
	}
}

func StandardStrategy() Strategy {
	return Strategy{
		Type:                  StrategyStandard,
		MaxQueries:            3,
		MaxResults:            15,
		MaxAnalysisIterations: 1,
		UseCritic:             true,
		TimeoutSeconds:        60,
	}
}

func DeepStrategy() Strategy {
	return Strategy{
		Type:                  StrategyDeep,
		MaxQueries:            5,
		MaxResults:            30,
		MaxAnalysisIterations: 3,
		UseCritic:             true,
		TimeoutSeconds:        180,
	}
}
