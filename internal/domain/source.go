package domain

import (
	"net/url"
	"time"
)

const (
	MaxSourcesPerUser = 100
)

type Source struct {
	ID          int64
	UserID      int64
	URL         string
	Name        string
	TrustLevel  TrustLevel
	IsUserAdded bool
	CreatedAt   time.Time
}

// только http/https с валидным хостом
func (s *Source) Validate() error {
	if s.URL == "" {
		return ErrInvalidURL
	}

	u, err := url.Parse(s.URL)
	if err != nil {
		return ErrInvalidURL
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrInvalidURL
	}

	if u.Host == "" {
		return ErrInvalidURL
	}

	return nil
}

func (s *Source) Domain() string {
	if s.URL == "" {
		return ""
	}

	u, err := url.Parse(s.URL)
	if err != nil {
		return ""
	}

	host := u.Host
	if len(host) > 4 && host[:4] == "www." {
		host = host[4:]
	}

	return host
}
