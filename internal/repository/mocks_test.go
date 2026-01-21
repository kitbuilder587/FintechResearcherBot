package repository

import (
	"context"
	"testing"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
)

func TestMockUserRepository_GetOrCreate(t *testing.T) {
	tests := []struct {
		name       string
		telegramID int64
		username   string
		setupRepo  func(*MockUserRepository)
		wantNew    bool
	}{
		{
			name:       "create new user",
			telegramID: 123,
			username:   "testuser",
			setupRepo:  func(m *MockUserRepository) {},
			wantNew:    true,
		},
		{
			name:       "get existing user",
			telegramID: 123,
			username:   "testuser",
			setupRepo: func(m *MockUserRepository) {
				m.GetOrCreate(context.Background(), 123, "testuser")
			},
			wantNew: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockUserRepository()
			tt.setupRepo(repo)

			user, err := repo.GetOrCreate(context.Background(), tt.telegramID, tt.username)
			if err != nil {
				t.Errorf("GetOrCreate() error = %v", err)
				return
			}
			if user == nil {
				t.Error("GetOrCreate() returned nil user")
			}
			if user.TelegramID != tt.telegramID {
				t.Errorf("user.TelegramID = %v, want %v", user.TelegramID, tt.telegramID)
			}
		})
	}
}

func TestMockUserRepository_GetByID(t *testing.T) {
	repo := NewMockUserRepository()
	user, _ := repo.GetOrCreate(context.Background(), 123, "testuser")

	found, err := repo.GetByID(context.Background(), user.ID)
	if err != nil {
		t.Errorf("GetByID() error = %v", err)
	}
	if found.ID != user.ID {
		t.Errorf("GetByID() got ID = %v, want %v", found.ID, user.ID)
	}

	_, err = repo.GetByID(context.Background(), 9999)
	if err != domain.ErrUserNotFound {
		t.Errorf("GetByID() error = %v, want ErrUserNotFound", err)
	}
}

func TestMockSourceRepository_Create(t *testing.T) {
	tests := []struct {
		name      string
		source    *domain.Source
		setupRepo func(*MockSourceRepository)
		wantErr   error
	}{
		{
			name: "create new source",
			source: &domain.Source{
				UserID: 1,
				URL:    "https://example.com",
				Name:   "Example",
			},
			setupRepo: func(m *MockSourceRepository) {},
			wantErr:   nil,
		},
		{
			name: "duplicate URL",
			source: &domain.Source{
				UserID: 1,
				URL:    "https://example.com",
				Name:   "Example",
			},
			setupRepo: func(m *MockSourceRepository) {
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
			repo := NewMockSourceRepository()
			tt.setupRepo(repo)

			err := repo.Create(context.Background(), tt.source)
			if err != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && tt.source.ID == 0 {
				t.Error("Create() did not set source ID")
			}
		})
	}
}

func TestMockSourceRepository_Delete(t *testing.T) {
	repo := NewMockSourceRepository()
	source := &domain.Source{UserID: 1, URL: "https://example.com", Name: "Example"}
	repo.Create(context.Background(), source)

	err := repo.Delete(context.Background(), 1, source.ID)
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	err = repo.Delete(context.Background(), 1, 9999)
	if err != domain.ErrSourceNotFound {
		t.Errorf("Delete() error = %v, want ErrSourceNotFound", err)
	}
}

func TestMockSourceRepository_ListByUser(t *testing.T) {
	repo := NewMockSourceRepository()
	repo.Create(context.Background(), &domain.Source{UserID: 1, URL: "https://a.com", Name: "A"})
	repo.Create(context.Background(), &domain.Source{UserID: 1, URL: "https://b.com", Name: "B"})
	repo.Create(context.Background(), &domain.Source{UserID: 2, URL: "https://c.com", Name: "C"})

	sources, err := repo.ListByUser(context.Background(), 1)
	if err != nil {
		t.Errorf("ListByUser() error = %v", err)
	}
	if len(sources) != 2 {
		t.Errorf("ListByUser() got %d sources, want 2", len(sources))
	}
}

func TestMockSourceRepository_CountByUser(t *testing.T) {
	repo := NewMockSourceRepository()
	repo.Create(context.Background(), &domain.Source{UserID: 1, URL: "https://a.com", Name: "A"})
	repo.Create(context.Background(), &domain.Source{UserID: 1, URL: "https://b.com", Name: "B"})

	count, err := repo.CountByUser(context.Background(), 1)
	if err != nil {
		t.Errorf("CountByUser() error = %v", err)
	}
	if count != 2 {
		t.Errorf("CountByUser() = %d, want 2", count)
	}
}

func TestMockSourceRepository_GetDomainsByUserID(t *testing.T) {
	tests := []struct {
		name        string
		sources     []*domain.Source
		userID      int64
		wantDomains []string
	}{
		{
			name:        "empty sources",
			sources:     nil,
			userID:      1,
			wantDomains: []string{},
		},
		{
			name: "single domain",
			sources: []*domain.Source{
				{UserID: 1, URL: "https://example.com/page1", Name: "A"},
			},
			userID:      1,
			wantDomains: []string{"example.com"},
		},
		{
			name: "multiple unique domains",
			sources: []*domain.Source{
				{UserID: 1, URL: "https://example.com/page1", Name: "A"},
				{UserID: 1, URL: "https://test.org/article", Name: "B"},
				{UserID: 1, URL: "https://news.ru/post", Name: "C"},
			},
			userID:      1,
			wantDomains: []string{"example.com", "test.org", "news.ru"},
		},
		{
			name: "duplicate domains filtered",
			sources: []*domain.Source{
				{UserID: 1, URL: "https://example.com/page1", Name: "A"},
				{UserID: 1, URL: "https://example.com/page2", Name: "B"},
				{UserID: 1, URL: "https://test.org/article", Name: "C"},
			},
			userID:      1,
			wantDomains: []string{"example.com", "test.org"},
		},
		{
			name: "only returns domains for specified user",
			sources: []*domain.Source{
				{UserID: 1, URL: "https://user1.com/page", Name: "A"},
				{UserID: 2, URL: "https://user2.com/page", Name: "B"},
			},
			userID:      1,
			wantDomains: []string{"user1.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockSourceRepository()
			for _, s := range tt.sources {
				repo.Create(context.Background(), s)
			}

			domains, err := repo.GetDomainsByUserID(context.Background(), tt.userID)
			if err != nil {
				t.Errorf("GetDomainsByUserID() error = %v", err)
				return
			}

			if len(domains) != len(tt.wantDomains) {
				t.Errorf("GetDomainsByUserID() got %d domains, want %d", len(domains), len(tt.wantDomains))
				return
			}

			domainSet := make(map[string]bool)
			for _, d := range domains {
				domainSet[d] = true
			}
			for _, want := range tt.wantDomains {
				if !domainSet[want] {
					t.Errorf("GetDomainsByUserID() missing domain %q", want)
				}
			}
		})
	}
}
