package telegram

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/kitbuilder587/fintech-bot/internal/domain"
)

type Handler struct {
	bot *Bot
}

func NewHandler(bot *Bot) *Handler {
	return &Handler{bot: bot}
}

var DefaultStrategy = domain.StandardStrategy

func (h *Handler) HandleMessage(ctx context.Context, msg *tgbotapi.Message) {
	h.bot.logger.Info("received message",
		zap.Int64("user_id", msg.From.ID),
		zap.String("username", msg.From.UserName),
		zap.Bool("is_command", msg.IsCommand()),
	)

	if msg.IsCommand() {
		cmd := msg.Command()
		if cmd == "quick" || cmd == "deep" || cmd == "research" {
			h.handleQuery(ctx, msg)
			return
		}
		h.handleCommand(ctx, msg)
	} else {
		h.handleQuery(ctx, msg)
	}
}

func (h *Handler) handleCommand(ctx context.Context, msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start":
		h.handleStart(ctx, msg)
	case "help":
		h.handleHelp(ctx, msg)
	case "sources":
		h.handleSources(ctx, msg)
	case "add":
		h.handleAdd(ctx, msg)
	case "remove":
		h.handleRemove(ctx, msg)
	case "trust":
		h.handleTrust(ctx, msg)
	default:
		h.bot.Send(msg.Chat.ID, "Неизвестная команда. Используйте /help для справки.")
	}
}

func (h *Handler) handleStart(ctx context.Context, msg *tgbotapi.Message) {
	user, err := h.bot.userService.GetOrCreate(ctx, msg.From.ID, msg.From.UserName)
	if err != nil {
		h.bot.logger.Error("failed to create user", zap.Error(err))
		h.bot.Send(msg.Chat.ID, "Произошла ошибка. Попробуйте позже.")
		return
	}

	count, err := h.bot.sourceService.ImportSeed(ctx, user.ID)
	if err != nil {
		h.bot.logger.Warn("failed to import seed sources", zap.Error(err))
	}

	var response string
	if count > 0 {
		response = fmt.Sprintf("Добро пожаловать! Добавлено %d доверенных источников.\n\nИспользуйте /help для просмотра доступных команд.", count)
	} else {
		response = "Добро пожаловать! Источники уже настроены.\n\nИспользуйте /help для просмотра доступных команд."
	}

	h.bot.Send(msg.Chat.ID, response)
}

func (h *Handler) handleHelp(ctx context.Context, msg *tgbotapi.Message) {
	helpText := `<b>Доступные команды:</b>

/start - Регистрация и импорт источников
/help - Показать эту справку
/sources - Список ваших источников
/add URL - Добавить источник
/remove N - Удалить источник по номеру
/trust N уровень - Изменить уровень доверия

<b>Режимы поиска:</b>
/quick вопрос - Быстрый поиск (1 запрос, без критика)
/research вопрос - Стандартный поиск (3 запроса, с критиком)
/deep вопрос - Глубокий анализ (5 запросов, с критиком)

<b>Уровни доверия:</b>
• high - высокий (приоритет в ответах)
• medium - средний
• low - низкий

<b>Как использовать:</b>
Просто отправьте ваш вопрос о финтехе, и я найду информацию из ваших доверенных источников.

<b>Примеры:</b>
• Обычный вопрос: "Какие тренды в финтехе в 2025 году?"
• Быстрый поиск: /quick что такое API?
• Глубокий анализ: /deep анализ рынка криптовалют`

	h.bot.Send(msg.Chat.ID, helpText)
}

func (h *Handler) handleSources(ctx context.Context, msg *tgbotapi.Message) {
	user, err := h.bot.userService.GetOrCreate(ctx, msg.From.ID, msg.From.UserName)
	if err != nil {
		h.bot.Send(msg.Chat.ID, "Произошла ошибка. Попробуйте позже.")
		return
	}

	sources, err := h.bot.sourceService.List(ctx, user.ID)
	if err != nil {
		h.bot.logger.Error("failed to list sources", zap.Error(err))
		h.bot.Send(msg.Chat.ID, "Произошла ошибка. Попробуйте позже.")
		return
	}

	if len(sources) == 0 {
		h.bot.Send(msg.Chat.ID, "У вас нет источников. Используйте /add URL для добавления.")
		return
	}

	response := FormatSourcesList(sources)
	h.bot.Send(msg.Chat.ID, response)
}

func (h *Handler) handleAdd(ctx context.Context, msg *tgbotapi.Message) {
	user, err := h.bot.userService.GetOrCreate(ctx, msg.From.ID, msg.From.UserName)
	if err != nil {
		h.bot.Send(msg.Chat.ID, "Произошла ошибка. Попробуйте позже.")
		return
	}

	url := strings.TrimSpace(msg.CommandArguments())
	if url == "" {
		h.bot.Send(msg.Chat.ID, "Укажите URL: /add https://example.com")
		return
	}

	err = h.bot.sourceService.Add(ctx, user.ID, url)
	if err != nil {
		h.bot.Send(msg.Chat.ID, mapErrorToMessage(err))
		return
	}

	h.bot.Send(msg.Chat.ID, "Источник добавлен.")
}

func (h *Handler) handleRemove(ctx context.Context, msg *tgbotapi.Message) {
	user, err := h.bot.userService.GetOrCreate(ctx, msg.From.ID, msg.From.UserName)
	if err != nil {
		h.bot.Send(msg.Chat.ID, "Произошла ошибка. Попробуйте позже.")
		return
	}

	numStr := strings.TrimSpace(msg.CommandArguments())
	if numStr == "" {
		h.bot.Send(msg.Chat.ID, "Укажите номер источника: /remove 1")
		return
	}

	num, err := strconv.Atoi(numStr)
	if err != nil || num < 1 {
		h.bot.Send(msg.Chat.ID, "Укажите корректный номер источника.")
		return
	}

	sources, err := h.bot.sourceService.List(ctx, user.ID)
	if err != nil {
		h.bot.Send(msg.Chat.ID, "Произошла ошибка. Попробуйте позже.")
		return
	}

	if num > len(sources) {
		h.bot.Send(msg.Chat.ID, fmt.Sprintf("Источник %d не найден.", num))
		return
	}

	sourceID := sources[num-1].ID
	err = h.bot.sourceService.Remove(ctx, user.ID, sourceID)
	if err != nil {
		h.bot.Send(msg.Chat.ID, mapErrorToMessage(err))
		return
	}

	h.bot.Send(msg.Chat.ID, "Источник удален.")
}

func (h *Handler) handleTrust(ctx context.Context, msg *tgbotapi.Message) {
	user, err := h.bot.userService.GetOrCreate(ctx, msg.From.ID, msg.From.UserName)
	if err != nil {
		h.bot.Send(msg.Chat.ID, "Произошла ошибка. Попробуйте позже.")
		return
	}

	args := strings.Fields(msg.CommandArguments())
	if len(args) != 2 {
		h.bot.Send(msg.Chat.ID, "Использование: /trust N уровень\nУровни: high, medium, low\nПример: /trust 1 high")
		return
	}

	num, err := strconv.Atoi(args[0])
	if err != nil || num < 1 {
		h.bot.Send(msg.Chat.ID, "Укажите корректный номер источника.")
		return
	}

	var level domain.TrustLevel
	switch strings.ToLower(args[1]) {
	case "high", "высокий":
		level = domain.TrustHigh
	case "medium", "средний":
		level = domain.TrustMedium
	case "low", "низкий":
		level = domain.TrustLow
	default:
		h.bot.Send(msg.Chat.ID, "Некорректный уровень доверия. Используйте: high, medium, low")
		return
	}

	sources, err := h.bot.sourceService.List(ctx, user.ID)
	if err != nil {
		h.bot.Send(msg.Chat.ID, "Произошла ошибка. Попробуйте позже.")
		return
	}

	if num > len(sources) {
		h.bot.Send(msg.Chat.ID, fmt.Sprintf("Источник %d не найден.", num))
		return
	}

	err = h.bot.sourceService.SetTrustLevel(ctx, user.ID, sources[num-1].ID, level)
	if err != nil {
		h.bot.Send(msg.Chat.ID, mapErrorToMessage(err))
		return
	}

	h.bot.Send(msg.Chat.ID, fmt.Sprintf("Уровень доверия источника #%d изменен на %s.", num, level.String()))
}

func (h *Handler) handleQuery(ctx context.Context, msg *tgbotapi.Message) {
	question, strategy := ParseQueryCommand(msg.Text, DefaultStrategy())

	h.processQueryWithStrategy(ctx, msg, question, strategy)
}

func (h *Handler) processQueryWithStrategy(ctx context.Context, msg *tgbotapi.Message, question string, strategy domain.Strategy) {
	if !h.bot.rateLimiter.Allow(msg.From.ID) {
		resetTime := h.bot.rateLimiter.ResetTime(msg.From.ID)
		h.bot.logger.Warn("rate limit exceeded",
			zap.Int64("user_id", msg.From.ID),
			zap.Time("reset_at", resetTime),
		)
		h.bot.RecordRateLimitHit(msg.From.ID)
		h.bot.Send(msg.Chat.ID, "Слишком много запросов. Пожалуйста, подождите минуту.")
		return
	}

	user, err := h.bot.userService.GetOrCreate(ctx, msg.From.ID, msg.From.UserName)
	if err != nil {
		h.bot.Send(msg.Chat.ID, "Произошла ошибка. Попробуйте позже.")
		return
	}

	h.bot.SendTyping(msg.Chat.ID)

	req := &domain.QueryRequest{
		UserID:   user.ID,
		Text:     question,
		Strategy: strategy,
	}

	h.bot.logger.Info("processing query with strategy",
		zap.Int64("user_id", user.ID),
		zap.String("strategy_type", string(strategy.Type)),
		zap.Int("max_queries", strategy.MaxQueries),
		zap.Int("max_results", strategy.MaxResults),
		zap.Bool("use_critic", strategy.UseCritic),
	)

	response, err := h.bot.queryService.Process(ctx, req)
	if err != nil {
		h.bot.logger.Error("query processing failed",
			zap.Error(err),
			zap.Int64("user_id", user.ID),
		)
		h.bot.Send(msg.Chat.ID, mapErrorToMessage(err))
		return
	}

	formattedResponse := FormatQueryResponse(response)
	strategyIndicator := h.formatStrategyIndicator(strategy)
	if strategyIndicator != "" {
		formattedResponse = strategyIndicator + "\n\n" + formattedResponse
	}

	messages := SplitMessage(formattedResponse, 4096) // лимит телеграма
	for _, m := range messages {
		if err := h.bot.Send(msg.Chat.ID, m); err != nil {
			h.bot.logger.Error("failed to send message", zap.Error(err))
		}
	}
}

func (h *Handler) formatStrategyIndicator(strategy domain.Strategy) string {
	switch strategy.Type {
	case domain.StrategyQuick:
		return "<i>Быстрый поиск</i>"
	case domain.StrategyDeep:
		return "<i>Глубокий анализ</i>"
	case domain.StrategyStandard:
		return ""
	default:
		return ""
	}
}

func mapErrorToMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrInvalidURL):
		return "Некорректный URL."
	case errors.Is(err, domain.ErrDuplicateSource):
		return "Источник уже добавлен."
	case errors.Is(err, domain.ErrSourceNotFound):
		return "Источник не найден."
	case errors.Is(err, domain.ErrSourceLimitReached):
		return "Достигнут лимит источников (100)."
	case errors.Is(err, domain.ErrNoSources):
		return "Нет источников для запроса. Добавьте источники с помощью /add."
	case errors.Is(err, domain.ErrNoResults):
		return "Не найдено результатов по вашему запросу."
	case errors.Is(err, domain.ErrEmptyQuery):
		return "Пустой запрос. Введите ваш вопрос."
	case errors.Is(err, domain.ErrQueryTooLong):
		return "Запрос слишком длинный. Максимум 1000 символов."
	case errors.Is(err, domain.ErrLLMFailed):
		return "Не удалось сформировать ответ. Попробуйте позже."
	default:
		return "Произошла ошибка. Попробуйте позже."
	}
}
