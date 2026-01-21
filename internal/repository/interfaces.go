package repository

import (
	"context"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
)

type UserRepository interface {
	GetOrCreate(ctx context.Context, telegramID int64, username string) (*domain.User, error)
	GetByID(ctx context.Context, id int64) (*domain.User, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Create(ctx context.Context, user *domain.User) error
}

type SourceRepository interface {
	Create(ctx context.Context, source *domain.Source) error
	Delete(ctx context.Context, userID, sourceID int64) error
	ListByUser(ctx context.Context, userID int64) ([]domain.Source, error)
	GetByID(ctx context.Context, sourceID int64) (*domain.Source, error)
	ExistsByURL(ctx context.Context, userID int64, url string) (bool, error)
	CountByUser(ctx context.Context, userID int64) (int, error)
	GetDomainsByUserID(ctx context.Context, userID int64) ([]string, error)
	UpdateTrustLevel(ctx context.Context, userID, sourceID int64, level domain.TrustLevel) error
}

// WorldModelRepository - хранилище для модели мира (факты, сущности, сессии).
// XXX: возможно стоит разбить на 3 отдельных интерфейса, но пока не критично
type WorldModelRepository interface {
	// факты
	CreateFact(ctx context.Context, fact *domain.Fact) error
	GetFactsByUser(ctx context.Context, userID int64, limit int) ([]domain.Fact, error)
	GetFactsBySession(ctx context.Context, sessionID string) ([]domain.Fact, error)
	SearchFacts(ctx context.Context, userID int64, query string) ([]domain.Fact, error)
	FindFactByContent(ctx context.Context, userID int64, content string) (*domain.Fact, error)

	// сущности
	CreateEntity(ctx context.Context, entity *domain.Entity) error
	GetEntityByName(ctx context.Context, userID int64, name string) (*domain.Entity, error)
	GetEntitiesByUser(ctx context.Context, userID int64) ([]domain.Entity, error)
	UpdateEntity(ctx context.Context, entity *domain.Entity) error

	// сессии исследований
	CreateSession(ctx context.Context, session *domain.ResearchSession) error
	GetRecentSessions(ctx context.Context, userID int64, limit int) ([]domain.ResearchSession, error)
	AddFactToSession(ctx context.Context, sessionID, factID string) error
	AddEntityToSession(ctx context.Context, sessionID, entityID string) error
}
