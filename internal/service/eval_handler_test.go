package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
)

type stubQueryService struct {
	req *domain.QueryRequest
}

func (s *stubQueryService) Process(ctx context.Context, req *domain.QueryRequest) (*domain.QueryResponse, error) {
	s.req = req
	return &domain.QueryResponse{
		Text: "answer [S1]",
		Usage: domain.UsageMetrics{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
			LLMRequests:  2,
			CostUSD:      0.001,
		},
		Sources: []domain.SourceRef{
			{Title: "Source", URL: "https://example.com/a", Content: "source content"},
		},
	}, nil
}

type stubSourceService struct {
	importedUserID int64
}

func (s *stubSourceService) Add(ctx context.Context, userID int64, url string) error { return nil }
func (s *stubSourceService) Remove(ctx context.Context, userID, sourceID int64) error {
	return nil
}
func (s *stubSourceService) List(ctx context.Context, userID int64) ([]domain.Source, error) {
	return nil, nil
}
func (s *stubSourceService) ImportSeed(ctx context.Context, userID int64) (int, error) {
	s.importedUserID = userID
	return 1, nil
}
func (s *stubSourceService) SetTrustLevel(ctx context.Context, userID, sourceID int64, level domain.TrustLevel) error {
	return nil
}

type stubUserService struct {
	userID   int64
	username string
}

func (s *stubUserService) GetOrCreate(ctx context.Context, telegramID int64, username string) (*domain.User, error) {
	s.userID = telegramID
	s.username = username
	return &domain.User{ID: telegramID, TelegramID: telegramID, Username: username}, nil
}

func TestEvalHandlerQuery(t *testing.T) {
	querySvc := &stubQueryService{}
	sourceSvc := &stubSourceService{}
	userSvc := &stubUserService{}
	handler := NewEvalHandler(querySvc, sourceSvc, userSvc, EvalHandlerConfig{UserID: 42})

	req := httptest.NewRequest(http.MethodPost, "/eval/query", strings.NewReader(`{"question":"What changed?","strategy":"quick"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if sourceSvc.importedUserID != 42 {
		t.Fatalf("import user = %d, want 42", sourceSvc.importedUserID)
	}
	if userSvc.userID != 42 || userSvc.username != "eval" {
		t.Fatalf("user = %d/%q, want 42/eval", userSvc.userID, userSvc.username)
	}
	if querySvc.req == nil || querySvc.req.Text != "What changed?" {
		t.Fatalf("query request was not forwarded")
	}
	if querySvc.req.Strategy.Type != domain.StrategyQuick {
		t.Fatalf("strategy = %s, want quick", querySvc.req.Strategy.Type)
	}

	var resp EvalQueryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Answer != "answer [S1]" {
		t.Fatalf("answer = %q", resp.Answer)
	}
	if len(resp.Sources) != 1 || resp.Sources[0].Content != "source content" {
		t.Fatalf("sources = %+v", resp.Sources)
	}
	if resp.Usage.InputTokens != 10 || resp.Usage.OutputTokens != 5 || resp.Usage.LLMRequests != 2 || resp.Usage.CostUSD != 0.001 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}
