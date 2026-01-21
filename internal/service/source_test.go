package service

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/repository"
)

func TestSourceService_Add(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name    string
		userID  int64
		url     string
		setup   func(*repository.MockSourceRepository)
		wantErr error
	}{
		{
			name:   "valid URL",
			userID: 1,
			url:    "https://example.com",
			setup:  func(m *repository.MockSourceRepository) {},
			wantErr: nil,
		},
		{
			name:    "invalid URL",
			userID:  1,
			url:     "not-a-url",
			setup:   func(m *repository.MockSourceRepository) {},
			wantErr: domain.ErrInvalidURL,
		},
		{
			name:   "duplicate URL",
			userID: 1,
			url:    "https://example.com",
			setup: func(m *repository.MockSourceRepository) {
				m.Create(context.Background(), &domain.Source{
					UserID: 1,
					URL:    "https://example.com",
					Name:   "Example",
				})
			},
			wantErr: domain.ErrDuplicateSource,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repository.NewMockSourceRepository()
			tt.setup(repo)

			svc := NewSourceService(repo, logger)
			err := svc.Add(context.Background(), tt.userID, tt.url)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("Add() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Add() unexpected error = %v", err)
			}
		})
	}
}

func TestSourceService_Remove(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name     string
		userID   int64
		sourceID int64
		setup    func(*repository.MockSourceRepository) int64
		wantErr  error
	}{
		{
			name:   "existing source",
			userID: 1,
			setup: func(m *repository.MockSourceRepository) int64 {
				source := &domain.Source{UserID: 1, URL: "https://example.com", Name: "Example"}
				m.Create(context.Background(), source)
				return source.ID
			},
			wantErr: nil,
		},
		{
			name:     "non-existing source",
			userID:   1,
			sourceID: 9999,
			setup:    func(m *repository.MockSourceRepository) int64 { return 9999 },
			wantErr:  domain.ErrSourceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repository.NewMockSourceRepository()
			sourceID := tt.setup(repo)
			if tt.sourceID != 0 {
				sourceID = tt.sourceID
			}

			svc := NewSourceService(repo, logger)
			err := svc.Remove(context.Background(), tt.userID, sourceID)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("Remove() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Remove() unexpected error = %v", err)
			}
		})
	}
}

func TestSourceService_List(t *testing.T) {
	logger := zap.NewNop()
	repo := repository.NewMockSourceRepository()

	repo.Create(context.Background(), &domain.Source{UserID: 1, URL: "https://a.com", Name: "A"})
	repo.Create(context.Background(), &domain.Source{UserID: 1, URL: "https://b.com", Name: "B"})
	repo.Create(context.Background(), &domain.Source{UserID: 2, URL: "https://c.com", Name: "C"})

	svc := NewSourceService(repo, logger)

	sources, err := svc.List(context.Background(), 1)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(sources) != 2 {
		t.Errorf("List() got %d sources, want 2", len(sources))
	}
}

func TestSourceService_ImportSeed(t *testing.T) {
	logger := zap.NewNop()
	repo := repository.NewMockSourceRepository()

	svc := NewSourceService(repo, logger)

	count, err := svc.ImportSeed(context.Background(), 1)
	if err != nil {
		t.Fatalf("ImportSeed() error = %v", err)
	}

	if count == 0 {
		t.Error("ImportSeed() imported 0 sources, expected more")
	}

	count2, err := svc.ImportSeed(context.Background(), 1)
	if err != nil {
		t.Fatalf("ImportSeed() error = %v", err)
	}

	if count2 != 0 {
		t.Errorf("ImportSeed() second import = %d, want 0 (all should exist)", count2)
	}
}
