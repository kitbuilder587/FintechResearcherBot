package telegram

import (
	"context"
	"errors"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/ratelimit"
)

func TestMapErrorToMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"invalid url", domain.ErrInvalidURL, "Некорректный URL."},
		{"duplicate", domain.ErrDuplicateSource, "Источник уже добавлен."},
		{"not found", domain.ErrSourceNotFound, "Источник не найден."},
		{"limit", domain.ErrSourceLimitReached, "Достигнут лимит источников (100)."},
		{"no sources", domain.ErrNoSources, "Нет источников для запроса. Добавьте источники с помощью /add."},
		{"no results", domain.ErrNoResults, "Не найдено результатов по вашему запросу."},
		{"empty", domain.ErrEmptyQuery, "Пустой запрос. Введите ваш вопрос."},
		{"too long", domain.ErrQueryTooLong, "Запрос слишком длинный. Максимум 1000 символов."},
		{"llm fail", domain.ErrLLMFailed, "Не удалось сформировать ответ. Попробуйте позже."},
		{"unknown", errors.New("some random error"), "Произошла ошибка. Попробуйте позже."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapErrorToMessage(tt.err)
			if got != tt.want {
				t.Errorf("mapErrorToMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMapErrorToMessage_WrappedErrors(t *testing.T) {
	wrappedErr := errors.Join(errors.New("context"), domain.ErrInvalidURL)
	got := mapErrorToMessage(wrappedErr)
	want := "Некорректный URL."
	if got != want {
		t.Errorf("mapErrorToMessage(wrapped) = %v, want %v", got, want)
	}
}

func TestMapErrorToMessage_AllDomainErrors(t *testing.T) {
	defaultMsg := "Произошла ошибка. Попробуйте позже."

	domainErrors := []error{
		domain.ErrInvalidURL,
		domain.ErrDuplicateSource,
		domain.ErrSourceNotFound,
		domain.ErrSourceLimitReached,
		domain.ErrNoSources,
		domain.ErrNoResults,
		domain.ErrEmptyQuery,
		domain.ErrQueryTooLong,
		domain.ErrLLMFailed,
	}

	for _, err := range domainErrors {
		got := mapErrorToMessage(err)
		if got == defaultMsg {
			t.Errorf("Domain error %v should have custom message, got default", err)
		}
	}
}

type TrackingQueryService struct {
	LastRequest  *domain.QueryRequest
	LastStrategy domain.Strategy
	CallCount    int
	Response     *domain.QueryResponse
	Error        error
}

func (m *TrackingQueryService) Process(ctx context.Context, req *domain.QueryRequest) (*domain.QueryResponse, error) {
	m.CallCount++
	m.LastRequest = req
	m.LastStrategy = req.Strategy

	if m.Error != nil {
		return nil, m.Error
	}
	if m.Response != nil {
		return m.Response, nil
	}
	return &domain.QueryResponse{
		Text:    "Mock response",
		Sources: []domain.SourceRef{},
	}, nil
}

func createTestBot(querySvc *TrackingQueryService) *Bot {
	logger := zap.NewNop()
	userSvc := &MockUserService{}
	sourceSvc := &MockSourceService{}

	return &Bot{
		api:           nil, // We won't use API in tests
		userService:   userSvc,
		sourceService: sourceSvc,
		queryService:  querySvc,
		logger:        logger,
		rateLimiter:   ratelimit.New(ratelimit.Config{RequestsPerMinute: 100}),
	}
}

func createTestMessage(userID int64, text string) *tgbotapi.Message {
	return &tgbotapi.Message{
		From: &tgbotapi.User{
			ID:       userID,
			UserName: "testuser",
		},
		Chat: &tgbotapi.Chat{
			ID: userID,
		},
		Text: text,
	}
}

func TestHandler_QuickCommand(t *testing.T) {
	querySvc := &TrackingQueryService{
		Response: &domain.QueryResponse{
			Text:    "Quick response",
			Sources: []domain.SourceRef{},
		},
	}
	bot := createTestBot(querySvc)
	handler := NewHandler(bot)

	bot.logger = zap.NewNop()
	msg := createTestMessage(123, "/quick что такое API?")
	handler.HandleMessage(context.Background(), msg)

	if querySvc.CallCount != 1 {
		t.Errorf("CallCount = %d, want 1", querySvc.CallCount)
	}
	if querySvc.LastStrategy.Type != domain.StrategyQuick {
		t.Errorf("Strategy = %v, want Quick", querySvc.LastStrategy.Type)
	}
	if querySvc.LastRequest.Text != "что такое API?" {
		t.Errorf("Text = %q, want 'что такое API?'", querySvc.LastRequest.Text)
	}
	if querySvc.LastStrategy.MaxQueries != 1 {
		t.Errorf("MaxQueries = %d, want 1", querySvc.LastStrategy.MaxQueries)
	}
	if querySvc.LastStrategy.MaxResults != 5 {
		t.Errorf("MaxResults = %d, want 5", querySvc.LastStrategy.MaxResults)
	}
	if querySvc.LastStrategy.UseCritic != false {
		t.Error("UseCritic should be false")
	}
}

func TestHandler_DeepCommand(t *testing.T) {
	querySvc := &TrackingQueryService{
		Response: &domain.QueryResponse{
			Text:    "Deep response",
			Sources: []domain.SourceRef{},
		},
	}
	bot := createTestBot(querySvc)
	handler := NewHandler(bot)

	msg := createTestMessage(123, "/deep анализ рынка")
	handler.HandleMessage(context.Background(), msg)

	if querySvc.CallCount != 1 {
		t.Errorf("CallCount = %d, want 1", querySvc.CallCount)
	}
	if querySvc.LastStrategy.Type != domain.StrategyDeep {
		t.Errorf("Strategy = %v, want Deep", querySvc.LastStrategy.Type)
	}
	if querySvc.LastRequest.Text != "анализ рынка" {
		t.Errorf("Text = %q, want 'анализ рынка'", querySvc.LastRequest.Text)
	}
	if querySvc.LastStrategy.MaxQueries != 5 {
		t.Errorf("MaxQueries = %d, want 5", querySvc.LastStrategy.MaxQueries)
	}
	if querySvc.LastStrategy.MaxResults != 30 {
		t.Errorf("MaxResults = %d, want 30", querySvc.LastStrategy.MaxResults)
	}
	if querySvc.LastStrategy.UseCritic != true {
		t.Error("UseCritic should be true")
	}
}

func TestHandler_ResearchCommand(t *testing.T) {
	querySvc := &TrackingQueryService{
		Response: &domain.QueryResponse{
			Text:    "Research response",
			Sources: []domain.SourceRef{},
		},
	}
	bot := createTestBot(querySvc)
	handler := NewHandler(bot)

	msg := createTestMessage(123, "/research финтех тренды")
	handler.HandleMessage(context.Background(), msg)

	if querySvc.CallCount != 1 {
		t.Errorf("CallCount = %d, want 1", querySvc.CallCount)
	}
	if querySvc.LastStrategy.Type != domain.StrategyStandard {
		t.Errorf("Strategy = %v, want Standard", querySvc.LastStrategy.Type)
	}
	if querySvc.LastRequest.Text != "финтех тренды" {
		t.Errorf("Text = %q, want 'финтех тренды'", querySvc.LastRequest.Text)
	}
}

func TestHandler_PlainText(t *testing.T) {
	querySvc := &TrackingQueryService{
		Response: &domain.QueryResponse{
			Text:    "Plain text response",
			Sources: []domain.SourceRef{},
		},
	}
	bot := createTestBot(querySvc)
	handler := NewHandler(bot)

	msg := createTestMessage(123, "обычный вопрос без команды")
	handler.HandleMessage(context.Background(), msg)

	if querySvc.CallCount != 1 {
		t.Errorf("CallCount = %d, want 1", querySvc.CallCount)
	}
	if querySvc.LastStrategy.Type != domain.StrategyStandard {
		t.Errorf("Strategy = %v, want Standard", querySvc.LastStrategy.Type)
	}
	if querySvc.LastRequest.Text != "обычный вопрос без команды" {
		t.Errorf("Text = %q, want 'обычный вопрос без команды'", querySvc.LastRequest.Text)
	}
}

func TestHandler_EmptyQuery(t *testing.T) {
	querySvc := &TrackingQueryService{
		Error: domain.ErrEmptyQuery,
	}
	bot := createTestBot(querySvc)
	handler := NewHandler(bot)

	msg := createTestMessage(123, "/quick ")
	handler.HandleMessage(context.Background(), msg)

	if querySvc.CallCount != 1 {
		t.Errorf("CallCount = %d, want 1", querySvc.CallCount)
	}
	if querySvc.LastStrategy.Type != domain.StrategyQuick {
		t.Errorf("Strategy = %v, want Quick", querySvc.LastStrategy.Type)
	}
}
