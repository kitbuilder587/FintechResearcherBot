package service

import (
	"context"
	_ "embed"
	"encoding/json"

	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/repository"
)

//go:embed seed_sources.json
var seedSourcesJSON []byte

type SeedSource struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type SourceService interface {
	Add(ctx context.Context, userID int64, url string) error
	Remove(ctx context.Context, userID, sourceID int64) error
	List(ctx context.Context, userID int64) ([]domain.Source, error)
	ImportSeed(ctx context.Context, userID int64) (int, error)
	SetTrustLevel(ctx context.Context, userID, sourceID int64, level domain.TrustLevel) error
}

type sourceService struct {
	repo   repository.SourceRepository
	logger *zap.Logger
}

func NewSourceService(repo repository.SourceRepository, logger *zap.Logger) SourceService {
	return &sourceService{
		repo:   repo,
		logger: logger,
	}
}

func (s *sourceService) Add(ctx context.Context, userID int64, url string) error {
	source := &domain.Source{
		UserID:      userID,
		URL:         url,
		TrustLevel:  domain.TrustMedium,
		IsUserAdded: true,
	}

	if err := source.Validate(); err != nil {
		return err
	}

	count, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return err
	}
	if count >= domain.MaxSourcesPerUser {
		return domain.ErrSourceLimitReached
	}

	source.Name = source.Domain()

	if err := s.repo.Create(ctx, source); err != nil {
		return err
	}

	s.logger.Info("source added",
		zap.Int64("user_id", userID),
		zap.String("url", url),
	)

	return nil
}

func (s *sourceService) Remove(ctx context.Context, userID, sourceID int64) error {
	if err := s.repo.Delete(ctx, userID, sourceID); err != nil {
		return err
	}

	s.logger.Info("source removed",
		zap.Int64("user_id", userID),
		zap.Int64("source_id", sourceID),
	)

	return nil
}

func (s *sourceService) List(ctx context.Context, userID int64) ([]domain.Source, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *sourceService) ImportSeed(ctx context.Context, userID int64) (int, error) {
	var seedSources []SeedSource
	if err := json.Unmarshal(seedSourcesJSON, &seedSources); err != nil {
		return 0, err
	}

	imported := 0
	for _, seed := range seedSources {
		exists, err := s.repo.ExistsByURL(ctx, userID, seed.URL)
		if err != nil {
			s.logger.Warn("failed to check source existence",
				zap.Error(err),
				zap.String("url", seed.URL),
			)
			continue
		}
		if exists {
			continue
		}

		source := &domain.Source{
			UserID:      userID,
			URL:         seed.URL,
			Name:        seed.Name,
			TrustLevel:  domain.TrustHigh,
			IsUserAdded: false,
		}

		if err := s.repo.Create(ctx, source); err != nil {
			s.logger.Warn("failed to import seed source",
				zap.Error(err),
				zap.String("url", seed.URL),
			)
			continue
		}
		imported++
	}

	s.logger.Info("seed sources imported",
		zap.Int64("user_id", userID),
		zap.Int("count", imported),
	)

	return imported, nil
}

func (s *sourceService) SetTrustLevel(ctx context.Context, userID, sourceID int64, level domain.TrustLevel) error {
	if err := s.repo.UpdateTrustLevel(ctx, userID, sourceID, level); err != nil {
		return err
	}

	s.logger.Info("trust level updated",
		zap.Int64("user_id", userID),
		zap.Int64("source_id", sourceID),
		zap.String("level", level.String()),
	)

	return nil
}

func loadSeedSources() ([]SeedSource, error) {
	var sources []SeedSource
	if err := json.Unmarshal(seedSourcesJSON, &sources); err != nil {
		return nil, err
	}
	return sources, nil
}
