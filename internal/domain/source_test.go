package domain

import (
	"errors"
	"testing"
)

func TestSource_Validate(t *testing.T) {
	tests := []struct {
		name    string
		source  Source
		wantErr error
	}{
		{
			name: "valid http URL",
			source: Source{
				URL: "http://example.com",
			},
			wantErr: nil,
		},
		{
			name: "valid https URL",
			source: Source{
				URL: "https://example.com/path?query=value",
			},
			wantErr: nil,
		},
		{
			name: "empty URL",
			source: Source{
				URL: "",
			},
			wantErr: ErrInvalidURL,
		},
		{
			name: "invalid scheme ftp",
			source: Source{
				URL: "ftp://example.com",
			},
			wantErr: ErrInvalidURL,
		},
		{
			name: "no scheme",
			source: Source{
				URL: "example.com",
			},
			wantErr: ErrInvalidURL,
		},
		{
			name: "no host",
			source: Source{
				URL: "http://",
			},
			wantErr: ErrInvalidURL,
		},
		{
			name: "malformed URL",
			source: Source{
				URL: "://invalid",
			},
			wantErr: ErrInvalidURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.source.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Source.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSource_Domain(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantDomain string
	}{
		{
			name:       "simple domain",
			url:        "https://example.com",
			wantDomain: "example.com",
		},
		{
			name:       "domain with www prefix stripped",
			url:        "https://www.example.com",
			wantDomain: "example.com",
		},
		{
			name:       "domain with port",
			url:        "https://example.com:8080",
			wantDomain: "example.com:8080",
		},
		{
			name:       "domain with path and query",
			url:        "https://example.com/path?query=value",
			wantDomain: "example.com",
		},
		{
			name:       "empty URL",
			url:        "",
			wantDomain: "",
		},
		{
			name:       "no host",
			url:        "http://",
			wantDomain: "",
		},
		{
			name:       "malformed URL",
			url:        "://invalid",
			wantDomain: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Source{URL: tt.url}
			got := s.Domain()

			if got != tt.wantDomain {
				t.Errorf("Source.Domain() = %v, want %v", got, tt.wantDomain)
			}
		})
	}
}
