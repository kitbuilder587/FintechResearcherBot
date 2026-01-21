package domain

import (
	"strings"
)

const MaxQueryLength = 1000

type QueryRequest struct {
	UserID       int64
	Text         string
	OnlyReliable bool
	Strategy     Strategy
}

func (q *QueryRequest) Validate() error {
	if strings.TrimSpace(q.Text) == "" {
		return ErrEmptyQuery
	}

	if len(q.Text) > MaxQueryLength {
		return ErrQueryTooLong
	}

	if err := q.Strategy.Validate(); err != nil {
		return err
	}

	return nil
}

func (q *QueryRequest) Sanitize() {
	q.Text = strings.TrimSpace(q.Text)
	if len(q.Text) > MaxQueryLength {
		q.Text = q.Text[:MaxQueryLength]
	}
}

type QueryResponse struct {
	Text    string
	Sources []SourceRef
}

type SourceRef struct {
	Marker     string
	Title      string
	URL        string
	TrustLevel TrustLevel
}
