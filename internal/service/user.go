package service

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/repository"
)

type UserService interface {
	GetOrCreate(ctx context.Context, telegramID int64, username string) (*domain.User, error)
}

type userService struct {
	repo   repository.UserRepository
	logger *zap.Logger
}

func NewUserService(repo repository.UserRepository, logger *zap.Logger) UserService {
	return &userService{
		repo:   repo,
		logger: logger,
	}
}

func (s *userService) GetOrCreate(ctx context.Context, telegramID int64, username string) (*domain.User, error) {
	user, err := s.repo.GetByTelegramID(ctx, telegramID)
	if err == nil {
		if user.Username != username {
			user.Username = username
			if updateErr := s.repo.Update(ctx, user); updateErr != nil {
				s.logger.Warn("failed to update username",
					zap.Error(updateErr),
					zap.Int64("telegram_id", telegramID),
				)
			}
		}
		return user, nil
	}

	if !errors.Is(err, domain.ErrUserNotFound) {
		return nil, err
	}

	newUser := &domain.User{
		TelegramID: telegramID,
		Username:   username,
		CreatedAt:  time.Now(),
	}

	if err := s.repo.Create(ctx, newUser); err != nil {
		return nil, err
	}

	s.logger.Info("new user created",
		zap.Int64("telegram_id", telegramID),
		zap.String("username", username),
	)

	return newUser, nil
}
