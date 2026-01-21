package telegram

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/metrics"
	"github.com/kitbuilder587/fintech-bot/internal/ratelimit"
	"github.com/kitbuilder587/fintech-bot/internal/service"
)

type BotConfig struct {
	Token             string
	Debug             bool
	RequestsPerMinute int
}

type Bot struct {
	api           *tgbotapi.BotAPI
	userService   service.UserService
	sourceService service.SourceService
	queryService  service.QueryService
	logger        *zap.Logger
	metrics       *metrics.Metrics
	handler       *Handler
	rateLimiter   *ratelimit.Limiter
	wg            sync.WaitGroup
}

func New(cfg BotConfig, userSvc service.UserService, sourceSvc service.SourceService, querySvc service.QueryService, logger *zap.Logger, m *metrics.Metrics) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("create bot api: %w", err)
	}

	api.Debug = cfg.Debug

	rateLimiter := ratelimit.New(ratelimit.Config{
		RequestsPerMinute: cfg.RequestsPerMinute,
	})

	bot := &Bot{
		api:           api,
		userService:   userSvc,
		sourceService: sourceSvc,
		queryService:  querySvc,
		logger:        logger,
		metrics:       m,
		rateLimiter:   rateLimiter,
	}

	bot.handler = NewHandler(bot)

	logger.Info("telegram bot authorized",
		zap.String("username", api.Self.UserName),
	)

	return bot, nil
}

func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	b.logger.Info("bot started, waiting for updates")

	for {
		select {
		case <-ctx.Done():
			b.logger.Info("bot stopping, waiting for handlers to finish")
			b.api.StopReceivingUpdates()
			b.wg.Wait()
			b.logger.Info("all handlers finished")
			return ctx.Err()
		case update := <-updates:
			if update.Message == nil {
				continue
			}
			b.wg.Add(1)
			go func(upd tgbotapi.Update) {
				defer b.wg.Done()
				b.handleUpdate(ctx, upd)
			}(update)
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	startTime := time.Now()

	defer func() {
		if r := recover(); r != nil {
			chatID := int64(0)
			if update.Message != nil && update.Message.Chat != nil {
				chatID = update.Message.Chat.ID
			}
			b.logger.Error("panic in update handler",
				zap.Any("panic", r),
				zap.Int64("chat_id", chatID),
			)
			if b.metrics != nil {
				b.metrics.RecordRequest("message", "panic", time.Since(startTime))
			}
		}
	}()

	b.handler.HandleMessage(ctx, update.Message)

	if b.metrics != nil {
		reqType := "command"
		if update.Message != nil && !update.Message.IsCommand() {
			reqType = "query"
		}
		b.metrics.RecordRequest(reqType, "processed", time.Since(startTime))
	}
}

func (b *Bot) Send(chatID int64, text string) error {
	if b.api == nil {
		return nil
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = true
	_, err := b.api.Send(msg)
	return err
}

func (b *Bot) SendTyping(chatID int64) {
	if b.api == nil {
		return
	}
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	b.api.Send(action)
}

func (b *Bot) RecordRateLimitHit(userID int64) {
	if b.metrics != nil {
		b.metrics.RecordRateLimitHit(strconv.FormatInt(userID, 10))
	}
}
