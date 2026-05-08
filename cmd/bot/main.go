package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/agent"
	"github.com/kitbuilder587/fintech-bot/internal/cache/memory"
	"github.com/kitbuilder587/fintech-bot/internal/config"
	"github.com/kitbuilder587/fintech-bot/internal/domain"
	"github.com/kitbuilder587/fintech-bot/internal/llm"
	"github.com/kitbuilder587/fintech-bot/internal/llm/gigachat"
	llmmock "github.com/kitbuilder587/fintech-bot/internal/llm/mock"
	"github.com/kitbuilder587/fintech-bot/internal/llm/openrouter"
	"github.com/kitbuilder587/fintech-bot/internal/metrics"
	"github.com/kitbuilder587/fintech-bot/internal/repository/postgres"
	"github.com/kitbuilder587/fintech-bot/internal/search"
	"github.com/kitbuilder587/fintech-bot/internal/search/searxng"
	"github.com/kitbuilder587/fintech-bot/internal/search/tavily"
	"github.com/kitbuilder587/fintech-bot/internal/service"
	"github.com/kitbuilder587/fintech-bot/internal/telegram"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, err := config.NewLogger(cfg.Log)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	defer logger.Sync()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	db, err := postgres.New(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer db.Close()
	logger.Info("connected to database")

	m := metrics.New()

	llmClient := newLLMClient(cfg.LLM, logger)
	llmClient = llm.WithMetrics(llmClient, m, cfg.LLM.Provider, llmModelName(cfg.LLM))

	searchClient := newSearchClient(cfg.Search, logger)

	cacheStore := memory.New()

	userRepo := postgres.NewUserRepo(db)
	sourceRepo := postgres.NewSourceRepo(db)
	worldModelRepo := postgres.NewWorldModelRepo(db)

	userSvc := service.NewUserService(userRepo, logger)
	sourceSvc := service.NewSourceService(sourceRepo, logger)

	worldModelSvc := service.NewWorldModelService(worldModelRepo, llmClient, logger)

	agents := agent.NewAllAgents(llmClient, logger)
	coordinator := agent.NewCoordinator(agents, llmClient, logger)
	coordAdapter := service.NewCoordinatorAdapter(coordinator)

	criticSvc := service.NewCriticService(llmClient, logger, domain.CriticConfig{
		MaxRetries: 2,
		StrictMode: false,
	})

	querySvc := service.NewQueryService(service.QueryServiceDeps{
		Sources: sourceRepo,
		LLM:     llmClient,
		Search:  searchClient,
		Cache:   cacheStore,
		Logger:  logger,
		Metrics: m,
		Config: service.QueryConfig{
			CacheTTL:      cfg.Cache.TTL,
			SearchTimeout: cfg.Timeouts.Source,
		},
		Critic:      criticSvc,
		WorldModel:  worldModelSvc,
		Coordinator: coordAdapter,
	})

	bot, err := telegram.New(telegram.BotConfig{
		Token:             cfg.Telegram.Token,
		RequestsPerMinute: cfg.RateLimit.RequestsPerMinute,
	}, userSvc, sourceSvc, querySvc, logger, m)
	if err != nil {
		return fmt.Errorf("create bot: %w", err)
	}

	// metrics + health endpoints
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	mux.Handle("/eval/query", service.NewEvalHandler(querySvc, sourceSvc, userSvc, service.EvalHandlerConfig{}))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			http.Error(w, "db unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	go func() {
		logger.Info("starting http server", zap.String("addr", ":8080"))
		if err := http.ListenAndServe(":8080", mux); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", zap.Error(err))
		}
	}()

	logger.Info("starting bot")
	return bot.Run(ctx)
}

func newLLMClient(cfg config.LLMConfig, logger *zap.Logger) llm.Client {
	switch cfg.Provider {
	case "gigachat":
		return gigachat.New(gigachat.Config{
			AuthKey:      cfg.GigaChat.AuthKey,
			ClientID:     cfg.GigaChat.ClientID,
			ClientSecret: cfg.GigaChat.ClientSecret,
			Scope:        cfg.GigaChat.Scope,
			AuthURL:      cfg.GigaChat.AuthURL,
			BaseURL:      cfg.GigaChat.BaseURL,
		}, logger)
	case "openrouter":
		return openrouter.New(openrouter.Config{
			APIKey:  cfg.OpenRouter.APIKey,
			Model:   cfg.OpenRouter.Model,
			BaseURL: cfg.OpenRouter.BaseURL,
		}, logger)
	default:
		logger.Warn("using mock LLM client", zap.String("provider", cfg.Provider))
		return llmmock.New()
	}
}

func llmModelName(cfg config.LLMConfig) string {
	switch cfg.Provider {
	case "gigachat":
		return "GigaChat"
	case "openrouter":
		return cfg.OpenRouter.Model
	default:
		return "mock"
	}
}

func newSearchClient(cfg config.SearchConfig, logger *zap.Logger) search.SearchClient {
	switch cfg.Provider {
	case "tavily":
		return tavily.New(tavily.Config{
			APIKey:  cfg.Tavily.APIKey,
			BaseURL: cfg.Tavily.BaseURL,
			Timeout: cfg.Tavily.Timeout,
		}, logger)
	default:
		return searxng.New(searxng.Config{
			BaseURL: cfg.SearXNG.BaseURL,
			Timeout: cfg.SearXNG.Timeout,
		}, logger)
	}
}
