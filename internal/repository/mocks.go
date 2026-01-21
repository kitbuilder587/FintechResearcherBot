package repository

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
)

type MockUserRepository struct {
	mu     sync.RWMutex
	users  map[int64]*domain.User // key: TelegramID
	nextID int64
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:  make(map[int64]*domain.User),
		nextID: 1,
	}
}

func (m *MockUserRepository) GetOrCreate(ctx context.Context, telegramID int64, username string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if user, exists := m.users[telegramID]; exists {
		if user.Username != username {
			user.Username = username
		}
		return user, nil
	}

	user := &domain.User{
		ID:         m.nextID,
		TelegramID: telegramID,
		Username:   username,
		CreatedAt:  time.Now(),
	}
	m.nextID++
	m.users[telegramID] = user
	return user, nil
}

func (m *MockUserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, user := range m.users {
		if user.ID == id {
			return user, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (m *MockUserRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if user, exists := m.users[telegramID]; exists {
		return user, nil
	}
	return nil, domain.ErrUserNotFound
}

func (m *MockUserRepository) Update(ctx context.Context, user *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.users[user.TelegramID]; !exists {
		return domain.ErrUserNotFound
	}
	m.users[user.TelegramID] = user
	return nil
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.users[user.TelegramID]; exists {
		return domain.ErrDuplicateSource // reuse error for simplicity
	}

	user.ID = m.nextID
	m.nextID++
	user.CreatedAt = time.Now()
	m.users[user.TelegramID] = user
	return nil
}

type MockSourceRepository struct {
	mu      sync.RWMutex
	sources map[int64]*domain.Source // key: Source ID
	nextID  int64
}

func NewMockSourceRepository() *MockSourceRepository {
	return &MockSourceRepository{
		sources: make(map[int64]*domain.Source),
		nextID:  1,
	}
}

func (m *MockSourceRepository) Create(ctx context.Context, source *domain.Source) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range m.sources {
		if s.UserID == source.UserID && s.URL == source.URL {
			return domain.ErrDuplicateSource
		}
	}

	source.ID = m.nextID
	m.nextID++
	source.CreatedAt = time.Now()
	m.sources[source.ID] = source
	return nil
}

func (m *MockSourceRepository) Delete(ctx context.Context, userID, sourceID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	source, exists := m.sources[sourceID]
	if !exists || source.UserID != userID {
		return domain.ErrSourceNotFound
	}

	delete(m.sources, sourceID)
	return nil
}

func (m *MockSourceRepository) ListByUser(ctx context.Context, userID int64) ([]domain.Source, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []domain.Source
	for _, s := range m.sources {
		if s.UserID == userID {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (m *MockSourceRepository) GetByID(ctx context.Context, sourceID int64) (*domain.Source, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if source, exists := m.sources[sourceID]; exists {
		return source, nil
	}
	return nil, domain.ErrSourceNotFound
}

func (m *MockSourceRepository) ExistsByURL(ctx context.Context, userID int64, url string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.sources {
		if s.UserID == userID && s.URL == url {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockSourceRepository) CountByUser(ctx context.Context, userID int64) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, s := range m.sources {
		if s.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (m *MockSourceRepository) GetDomainsByUserID(ctx context.Context, userID int64) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	domains := make(map[string]bool)
	for _, s := range m.sources {
		if s.UserID == userID {
			d := s.Domain()
			if d != "" {
				domains[d] = true
			}
		}
	}

	result := make([]string, 0, len(domains))
	for d := range domains {
		result = append(result, d)
	}
	return result, nil
}

func (m *MockSourceRepository) UpdateTrustLevel(ctx context.Context, userID, sourceID int64, level domain.TrustLevel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	source, exists := m.sources[sourceID]
	if !exists || source.UserID != userID {
		return domain.ErrSourceNotFound
	}

	source.TrustLevel = level
	return nil
}

type MockWorldModelRepository struct {
	mu              sync.RWMutex
	facts           map[string]*domain.Fact            // key: Fact ID
	entities        map[string]*domain.Entity          // key: Entity ID
	sessions        map[string]*domain.ResearchSession // key: Session ID
	sessionFacts    map[string][]string                // session_id -> []fact_id
	sessionEntities map[string][]string
}

func NewMockWorldModelRepository() *MockWorldModelRepository {
	return &MockWorldModelRepository{
		facts:           make(map[string]*domain.Fact),
		entities:        make(map[string]*domain.Entity),
		sessions:        make(map[string]*domain.ResearchSession),
		sessionFacts:    make(map[string][]string),
		sessionEntities: make(map[string][]string),
	}
}

func (m *MockWorldModelRepository) CreateFact(ctx context.Context, fact *domain.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.facts[fact.ID]; exists {
		return domain.ErrDuplicateSource
	}

	if fact.ExtractedAt.IsZero() {
		fact.ExtractedAt = time.Now()
	}
	m.facts[fact.ID] = fact
	return nil
}

func (m *MockWorldModelRepository) GetFactsByUser(ctx context.Context, userID int64, limit int) ([]domain.Fact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []domain.Fact
	for _, f := range m.facts {
		if f.UserID == userID {
			result = append(result, *f)
		}
	}

	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].ExtractedAt.Before(result[j].ExtractedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

func (m *MockWorldModelRepository) GetFactsBySession(ctx context.Context, sessionID string) ([]domain.Fact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	factIDs, exists := m.sessionFacts[sessionID]
	if !exists {
		return nil, nil
	}

	var result []domain.Fact
	for _, fid := range factIDs {
		if f, ok := m.facts[fid]; ok {
			result = append(result, *f)
		}
	}
	return result, nil
}

func (m *MockWorldModelRepository) SearchFacts(ctx context.Context, userID int64, query string) ([]domain.Fact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []domain.Fact
	queryLower := strings.ToLower(query)
	for _, f := range m.facts {
		if f.UserID == userID && strings.Contains(strings.ToLower(f.Content), queryLower) {
			result = append(result, *f)
		}
	}
	return result, nil
}

func (m *MockWorldModelRepository) FindFactByContent(ctx context.Context, userID int64, content string) (*domain.Fact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, f := range m.facts {
		if f.UserID == userID && f.Content == content {
			return f, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *MockWorldModelRepository) CreateEntity(ctx context.Context, entity *domain.Entity) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, e := range m.entities {
		if e.UserID == entity.UserID && e.Name == entity.Name {
			return domain.ErrDuplicateSource
		}
	}

	if entity.FirstSeenAt.IsZero() {
		entity.FirstSeenAt = time.Now()
	}
	if entity.LastSeenAt.IsZero() {
		entity.LastSeenAt = time.Now()
	}
	if entity.Attributes == nil {
		entity.Attributes = make(map[string]string)
	}
	m.entities[entity.ID] = entity
	return nil
}

func (m *MockWorldModelRepository) GetEntityByName(ctx context.Context, userID int64, name string) (*domain.Entity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, e := range m.entities {
		if e.UserID == userID && e.Name == name {
			return e, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *MockWorldModelRepository) GetEntitiesByUser(ctx context.Context, userID int64) ([]domain.Entity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []domain.Entity
	for _, e := range m.entities {
		if e.UserID == userID {
			result = append(result, *e)
		}
	}
	return result, nil
}

func (m *MockWorldModelRepository) UpdateEntity(ctx context.Context, entity *domain.Entity) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.entities[entity.ID]; !exists {
		return domain.ErrNotFound
	}

	m.entities[entity.ID] = entity
	return nil
}

func (m *MockWorldModelRepository) CreateSession(ctx context.Context, session *domain.ResearchSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[session.ID]; exists {
		return domain.ErrDuplicateSource
	}

	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *MockWorldModelRepository) GetRecentSessions(ctx context.Context, userID int64, limit int) ([]domain.ResearchSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []domain.ResearchSession
	for _, s := range m.sessions {
		if s.UserID == userID {
			result = append(result, *s)
		}
	}

	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].CreatedAt.Before(result[j].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

func (m *MockWorldModelRepository) AddFactToSession(ctx context.Context, sessionID, factID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, fid := range m.sessionFacts[sessionID] {
		if fid == factID {
			return nil
		}
	}

	m.sessionFacts[sessionID] = append(m.sessionFacts[sessionID], factID)
	return nil
}

func (m *MockWorldModelRepository) AddEntityToSession(ctx context.Context, sessionID, entityID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, eid := range m.sessionEntities[sessionID] {
		if eid == entityID {
			return nil
		}
	}

	m.sessionEntities[sessionID] = append(m.sessionEntities[sessionID], entityID)
	return nil
}
