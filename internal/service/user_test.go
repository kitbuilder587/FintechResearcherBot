package service

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/repository"
)

func TestUserService_GetOrCreate(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name       string
		telegramID int64
		username   string
		setup      func(*repository.MockUserRepository)
		wantNew    bool
		wantErr    bool
	}{
		{
			name:       "new user created",
			telegramID: 123,
			username:   "testuser",
			setup:      func(m *repository.MockUserRepository) {},
			wantNew:    true,
			wantErr:    false,
		},
		{
			name:       "existing user returned",
			telegramID: 123,
			username:   "testuser",
			setup: func(m *repository.MockUserRepository) {
				m.GetOrCreate(context.Background(), 123, "testuser")
			},
			wantNew: false,
			wantErr: false,
		},
		{
			name:       "username updated",
			telegramID: 123,
			username:   "newname",
			setup: func(m *repository.MockUserRepository) {
				m.GetOrCreate(context.Background(), 123, "oldname")
			},
			wantNew: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repository.NewMockUserRepository()
			tt.setup(repo)

			svc := NewUserService(repo, logger)
			user, err := svc.GetOrCreate(context.Background(), tt.telegramID, tt.username)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetOrCreate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if user == nil {
					t.Error("GetOrCreate() returned nil user")
					return
				}
				if user.TelegramID != tt.telegramID {
					t.Errorf("user.TelegramID = %v, want %v", user.TelegramID, tt.telegramID)
				}
				if user.Username != tt.username {
					t.Errorf("user.Username = %v, want %v", user.Username, tt.username)
				}
			}
		})
	}
}

func TestUserService_GetOrCreate_RepoError(t *testing.T) {
	logger := zap.NewNop()

	repo := &errorMockUserRepo{err: errors.New("database error")}

	svc := NewUserService(repo, logger)
	_, err := svc.GetOrCreate(context.Background(), 123, "test")

	if err == nil {
		t.Error("GetOrCreate() expected error, got nil")
	}
}

type errorMockUserRepo struct {
	err error
}

func (m *errorMockUserRepo) GetOrCreate(ctx context.Context, telegramID int64, username string) (*domain.User, error) {
	return nil, m.err
}

func (m *errorMockUserRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	return nil, m.err
}

func (m *errorMockUserRepo) GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	return nil, m.err
}

func (m *errorMockUserRepo) Update(ctx context.Context, user *domain.User) error {
	return m.err
}

func (m *errorMockUserRepo) Create(ctx context.Context, user *domain.User) error {
	return m.err
}
