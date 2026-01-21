package telegram

import (
	"context"
	"testing"
	"time"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/ratelimit"
)

type MockUserService struct {
	GetOrCreateFunc func(ctx context.Context, telegramID int64, username string) (*domain.User, error)
}

func (m *MockUserService) GetOrCreate(ctx context.Context, telegramID int64, username string) (*domain.User, error) {
	if m.GetOrCreateFunc != nil {
		return m.GetOrCreateFunc(ctx, telegramID, username)
	}
	return &domain.User{
		ID:         telegramID,
		TelegramID: telegramID,
		Username:   username,
		CreatedAt:  time.Now(),
	}, nil
}

type MockSourceService struct {
	AddFunc           func(ctx context.Context, userID int64, url string) error
	RemoveFunc        func(ctx context.Context, userID, sourceID int64) error
	ListFunc          func(ctx context.Context, userID int64) ([]domain.Source, error)
	ImportSeedFunc    func(ctx context.Context, userID int64) (int, error)
	SetTrustLevelFunc func(ctx context.Context, userID, sourceID int64, level domain.TrustLevel) error
}

func (m *MockSourceService) Add(ctx context.Context, userID int64, url string) error {
	if m.AddFunc != nil {
		return m.AddFunc(ctx, userID, url)
	}
	return nil
}

func (m *MockSourceService) Remove(ctx context.Context, userID, sourceID int64) error {
	if m.RemoveFunc != nil {
		return m.RemoveFunc(ctx, userID, sourceID)
	}
	return nil
}

func (m *MockSourceService) List(ctx context.Context, userID int64) ([]domain.Source, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, userID)
	}
	return []domain.Source{}, nil
}

func (m *MockSourceService) ImportSeed(ctx context.Context, userID int64) (int, error) {
	if m.ImportSeedFunc != nil {
		return m.ImportSeedFunc(ctx, userID)
	}
	return 0, nil
}

func (m *MockSourceService) SetTrustLevel(ctx context.Context, userID, sourceID int64, level domain.TrustLevel) error {
	if m.SetTrustLevelFunc != nil {
		return m.SetTrustLevelFunc(ctx, userID, sourceID, level)
	}
	return nil
}

type MockQueryService struct {
	ProcessFunc func(ctx context.Context, req *domain.QueryRequest) (*domain.QueryResponse, error)
}

func (m *MockQueryService) Process(ctx context.Context, req *domain.QueryRequest) (*domain.QueryResponse, error) {
	if m.ProcessFunc != nil {
		return m.ProcessFunc(ctx, req)
	}
	return &domain.QueryResponse{
		Text:    "Mock response",
		Sources: []domain.SourceRef{},
	}, nil
}

func TestRateLimiter(t *testing.T) {
	limiter := ratelimit.New(ratelimit.Config{
		RequestsPerMinute: 2,
	})

	userID := int64(12345)

	if !limiter.Allow(userID) {
		t.Error("First request should be allowed")
	}

	if !limiter.Allow(userID) {
		t.Error("Second request should be allowed")
	}

	if limiter.Allow(userID) {
		t.Error("Third request should be blocked due to rate limit")
	}

	remaining := limiter.RemainingRequests(userID)
	if remaining != 0 {
		t.Errorf("RemainingRequests() = %d, want 0", remaining)
	}
}

func TestRateLimiter_DifferentUsers(t *testing.T) {
	limiter := ratelimit.New(ratelimit.Config{
		RequestsPerMinute: 1,
	})

	user1 := int64(111)
	user2 := int64(222)

	if !limiter.Allow(user1) {
		t.Error("User1 first request should be allowed")
	}

	if limiter.Allow(user1) {
		t.Error("User1 second request should be blocked")
	}

	if !limiter.Allow(user2) {
		t.Error("User2 first request should be allowed")
	}
}

func TestRateLimiter_ResetTime(t *testing.T) {
	limiter := ratelimit.New(ratelimit.Config{
		RequestsPerMinute: 1,
	})

	userID := int64(12345)

	limiter.Allow(userID)

	resetTime := limiter.ResetTime(userID)
	if resetTime.Before(time.Now()) {
		t.Error("ResetTime should be in the future")
	}

	if resetTime.After(time.Now().Add(time.Minute + time.Second)) {
		t.Error("ResetTime should be within 1 minute")
	}
}

func TestBotConfig_DefaultValues(t *testing.T) {
	cfg := BotConfig{
		Token:             "test-token",
		Debug:             false,
		RequestsPerMinute: 0, // Should use default
	}

	limiter := ratelimit.New(ratelimit.Config{
		RequestsPerMinute: cfg.RequestsPerMinute,
	})

	if !limiter.Allow(1) {
		t.Error("Should allow at least 1 request with default config")
	}
}
