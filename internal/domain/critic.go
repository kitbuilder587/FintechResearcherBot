package domain

type CriticResult struct {
	Approved    bool
	Issues      []string
	Suggestions []string
	Confidence  float64 // 0.0-1.0
}

type CriticConfig struct {
	MaxRetries int  // 0-10
	StrictMode bool // любая проблема = отклонить
}

func (r *CriticResult) HasCriticalIssues() bool {
	return len(r.Issues) > 0
}

func (r *CriticResult) NeedsRevision() bool {
	return !r.Approved || len(r.Issues) > 0 || len(r.Suggestions) > 0
}

// в strict режиме suggestions тоже требуют правки
func (r *CriticResult) NeedsRevisionStrict(strictMode bool) bool {
	return !r.Approved || len(r.Issues) > 0 || (strictMode && len(r.Suggestions) > 0)
}

func (c *CriticConfig) Validate() error {
	if c.MaxRetries < 0 {
		return ErrInvalidMaxRetries
	}
	if c.MaxRetries > 10 {
		return ErrMaxRetriesExceeded
	}
	return nil
}
