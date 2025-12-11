package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/conveer/telegram-bot/internal/models"
	"github.com/conveer/telegram-bot/internal/service"
	"github.com/conveer/telegram-bot/internal/utils"
	"github.com/go-telegram/bot"
	botmodels "github.com/go-telegram/bot/models"
)

type CallbackHandlers struct {
	authService    service.AuthService
	commandService service.CommandService
	exportService  service.ExportService
	statsService   service.StatsService
	botService     service.BotService
}

func NewCallbackHandlers(
	authService service.AuthService,
	commandService service.CommandService,
	exportService service.ExportService,
	statsService service.StatsService,
	botService service.BotService,
) *CallbackHandlers {
	return &CallbackHandlers{
		authService:    authService,
		commandService: commandService,
		exportService:  exportService,
		statsService:   statsService,
		botService:     botService,
	}
}

func (h *CallbackHandlers) HandleCallback(ctx context.Context, b *bot.Bot, update *botmodels.Update) {
	query := update.CallbackQuery
	if query == nil {
		return
	}

	// Answer callback query to remove loading animation
	b.AnswerCallbackQuery(ctx, &botmodels.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
	})

	// Parse callback data
	parts := strings.Split(query.Data, ":")
	if len(parts) == 0 {
		return
	}

	action := parts[0]

	switch action {
	case "accounts":
		h.handleAccountsCallback(ctx, b, query, parts[1:])
	case "export":
		h.handleExportCallback(ctx, b, query, parts[1:])
	case "stats":
		h.handleStatsCallback(ctx, b, query, parts[1:])
	case "warming":
		h.handleWarmingCallback(ctx, b, query, parts[1:])
	case "proxy":
		h.handleProxyCallback(ctx, b, query, parts[1:])
	case "sms":
		h.handleSMSCallback(ctx, b, query, parts[1:])
	case "menu":
		h.handleMenuCallback(ctx, b, query, parts[1:])
	}
}

func (h *CallbackHandlers) handleAccountsCallback(ctx context.Context, b *bot.Bot, query *botmodels.CallbackQuery, params []string) {
	if len(params) < 1 {
		return
	}

	platform := params[0]
	page := 1
	if len(params) > 2 && params[1] == "page" {
		if p, err := strconv.Atoi(params[2]); err == nil {
			page = p
		}
	}

	// Get account stats
	stats, err := h.statsService.GetAccountStats(ctx, platform)
	if err != nil {
		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:    query.Message.Chat.ID,
			MessageID: query.Message.MessageID,
			Text:      "❌ Ошибка получения данных аккаунтов",
		})
		return
	}

	// Format accounts table
	text := fmt.Sprintf(`📊 *Аккаунты %s*

Всего: %d
✅ Готовы: %d
🔥 Прогрев: %d
❌ Баны: %d

Страница %d`, strings.ToUpper(platform), stats.Total,
		stats.ByStatus["ready"],
		stats.ByStatus["warming"],
		stats.ByStatus["banned"],
		page)

	// Add pagination keyboard
	keyboard := utils.PaginationKeyboard(page, 10, fmt.Sprintf("accounts:%s", platform))

	b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
		ChatID:      query.Message.Chat.ID,
		MessageID:   query.Message.MessageID,
		Text:        text,
		ParseMode:   botmodels.ParseModeMarkdown,
		ReplyMarkup: keyboard,
	})
}

func (h *CallbackHandlers) handleExportCallback(ctx context.Context, b *bot.Bot, query *botmodels.CallbackQuery, params []string) {
	if len(params) < 1 {
		return
	}

	if params[0] == "platform" && len(params) > 1 {
		// Platform selected, show format options
		platform := params[1]
		keyboard := utils.ExportFormatKeyboard(platform)

		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			Text:        fmt.Sprintf("📤 Экспорт %s\n\nВыберите формат:", strings.ToUpper(platform)),
			ReplyMarkup: keyboard,
		})
		return
	}

	if len(params) < 2 {
		return
	}

	platform := params[0]
	format := models.ExportFormat(params[1])

	// Update message to show progress
	b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
		ChatID:    query.Message.Chat.ID,
		MessageID: query.Message.MessageID,
		Text:      "⏳ Экспортирую аккаунты...",
	})

	// Export all accounts
	data, filename, err := h.exportService.ExportAccounts(ctx, platform, []string{"all"}, format)
	if err != nil {
		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:    query.Message.Chat.ID,
			MessageID: query.Message.MessageID,
			Text:      fmt.Sprintf("❌ Ошибка экспорта: %v", err),
		})
		return
	}

	// Send file
	h.botService.SendDocument(ctx, query.Message.Chat.ID, data, filename)

	// Update message
	b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
		ChatID:    query.Message.Chat.ID,
		MessageID: query.Message.MessageID,
		Text:      fmt.Sprintf("✅ Экспорт завершен!\nФайл: %s", filename),
	})
}

func (h *CallbackHandlers) handleStatsCallback(ctx context.Context, b *bot.Bot, query *botmodels.CallbackQuery, params []string) {
	if len(params) < 1 {
		return
	}

	action := params[0]

	switch action {
	case "refresh":
		// Refresh stats
		stats, err := h.statsService.GetOverallStats(ctx)
		if err != nil {
			b.AnswerCallbackQuery(ctx, &botmodels.AnswerCallbackQueryParams{
				CallbackQueryID: query.ID,
				Text:            "❌ Ошибка обновления",
				ShowAlert:       true,
			})
			return
		}

		text := utils.FormatOverallStats(stats)
		keyboard := utils.StatsActionsKeyboard()

		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			Text:        text,
			ParseMode:   botmodels.ParseModeMarkdown,
			ReplyMarkup: keyboard,
		})

		b.AnswerCallbackQuery(ctx, &botmodels.AnswerCallbackQueryParams{
			CallbackQueryID: query.ID,
			Text:            "✅ Обновлено",
		})

	case "platform":
		if len(params) < 2 {
			return
		}
		platform := params[1]

		stats, err := h.statsService.GetDetailedStats(ctx, platform)
		if err != nil {
			b.AnswerCallbackQuery(ctx, &botmodels.AnswerCallbackQueryParams{
				CallbackQueryID: query.ID,
				Text:            "❌ Ошибка получения статистики",
				ShowAlert:       true,
			})
			return
		}

		text := utils.FormatDetailedStats(stats)

		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:    query.Message.Chat.ID,
			MessageID: query.Message.MessageID,
			Text:      text,
			ParseMode: botmodels.ParseModeMarkdown,
		})
	}
}

func (h *CallbackHandlers) handleWarmingCallback(ctx context.Context, b *bot.Bot, query *botmodels.CallbackQuery, params []string) {
	if len(params) < 1 {
		return
	}

	action := params[0]

	switch action {
	case "start":
		// Show warming start form
		keyboard := utils.WarmingStartKeyboard()
		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			Text:        "🔥 Запуск прогрева\n\nВыберите параметры:",
			ReplyMarkup: keyboard,
		})

	case "scenario":
		if len(params) < 2 {
			return
		}
		// Show duration selection for scenario
		scenario := params[1]
		keyboard := utils.WarmingDurationKeyboard(scenario)
		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			Text:        fmt.Sprintf("🔥 Сценарий: %s\n\nВыберите длительность:", scenario),
			ReplyMarkup: keyboard,
		})
	}
}

func (h *CallbackHandlers) handleProxyCallback(ctx context.Context, b *bot.Bot, query *botmodels.CallbackQuery, params []string) {
	if len(params) < 1 {
		return
	}

	action := params[0]

	switch action {
	case "allocate":
		// Show proxy type selection
		keyboard := utils.ProxyTypeKeyboard()
		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			Text:        "🌐 Выберите тип прокси:",
			ReplyMarkup: keyboard,
		})

	case "type":
		if len(params) < 2 {
			return
		}
		proxyType := params[1]

		// Here would be account selection, for now just simulate
		err := h.commandService.AllocateProxy(ctx, "sample_account_id", proxyType)
		if err != nil {
			b.AnswerCallbackQuery(ctx, &botmodels.AnswerCallbackQueryParams{
				CallbackQueryID: query.ID,
				Text:            "❌ Ошибка выделения прокси",
				ShowAlert:       true,
			})
			return
		}

		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:    query.Message.Chat.ID,
			MessageID: query.Message.MessageID,
			Text:      fmt.Sprintf("✅ Прокси типа %s выделен", proxyType),
		})
	}
}

func (h *CallbackHandlers) handleSMSCallback(ctx context.Context, b *bot.Bot, query *botmodels.CallbackQuery, params []string) {
	if len(params) < 1 {
		return
	}

	action := params[0]

	switch action {
	case "purchase":
		// Show service selection
		keyboard := utils.SMSServiceKeyboard()
		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			Text:        "📱 Выберите сервис:",
			ReplyMarkup: keyboard,
		})

	case "service":
		if len(params) < 2 {
			return
		}
		service := params[1]

		// Show country selection
		keyboard := utils.SMSCountryKeyboard(service)
		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			Text:        fmt.Sprintf("📱 Сервис: %s\n\nВыберите страну:", strings.ToUpper(service)),
			ReplyMarkup: keyboard,
		})

	case "country":
		if len(params) < 3 {
			return
		}
		service := params[1]
		country := params[2]

		err := h.commandService.PurchaseNumber(ctx, service, country)
		if err != nil {
			b.AnswerCallbackQuery(ctx, &botmodels.AnswerCallbackQueryParams{
				CallbackQueryID: query.ID,
				Text:            "❌ Ошибка покупки номера",
				ShowAlert:       true,
			})
			return
		}

		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:    query.Message.Chat.ID,
			MessageID: query.Message.MessageID,
			Text:      fmt.Sprintf("✅ Номер для %s (%s) куплен", strings.ToUpper(service), country),
		})
	}
}

func (h *CallbackHandlers) handleMenuCallback(ctx context.Context, b *bot.Bot, query *botmodels.CallbackQuery, params []string) {
	if len(params) < 1 {
		return
	}

	section := params[0]
	user, _ := GetUserFromContext(ctx)

	switch section {
	case "stats":
		stats, err := h.statsService.GetOverallStats(ctx)
		if err != nil {
			return
		}
		text := utils.FormatOverallStats(stats)
		keyboard := utils.StatsActionsKeyboard()

		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			Text:        text,
			ParseMode:   botmodels.ParseModeMarkdown,
			ReplyMarkup: keyboard,
		})

	case "accounts":
		keyboard := utils.PlatformSelectionKeyboard()
		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			Text:        "👥 Выберите платформу:",
			ReplyMarkup: keyboard,
		})

	case "management":
		keyboard := utils.ManagementMenuKeyboard(user)
		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			Text:        "⚙️ Управление:",
			ReplyMarkup: keyboard,
		})

	case "export":
		keyboard := utils.PlatformSelectionKeyboard()
		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			Text:        "📤 Выберите платформу для экспорта:",
			ReplyMarkup: keyboard,
		})

	case "back":
		// Return to main menu
		welcomeText := "👋 *Главное меню*\n\nВыберите раздел:"
		keyboard := utils.MainMenuKeyboard(user)

		b.EditMessageText(ctx, &botmodels.EditMessageTextParams{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			Text:        welcomeText,
			ParseMode:   botmodels.ParseModeMarkdown,
			ReplyMarkup: keyboard,
		})
	}
}