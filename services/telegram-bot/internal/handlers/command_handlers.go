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

type CommandHandlers struct {
	authService    service.AuthService
	commandService service.CommandService
	exportService  service.ExportService
	statsService   service.StatsService
	botService     service.BotService
}

func NewCommandHandlers(
	authService service.AuthService,
	commandService service.CommandService,
	exportService service.ExportService,
	statsService service.StatsService,
	botService service.BotService,
) *CommandHandlers {
	return &CommandHandlers{
		authService:    authService,
		commandService: commandService,
		exportService:  exportService,
		statsService:   statsService,
		botService:     botService,
	}
}

func (h *CommandHandlers) HandleStart(ctx context.Context, b *bot.Bot, update *botmodels.Update) {
	chatID := update.Message.Chat.ID
	user, ok := GetUserFromContext(ctx)

	var roleText string
	if ok && user != nil {
		switch user.Role {
		case models.RoleAdmin:
			roleText = "Администратор"
		case models.RoleOperator:
			roleText = "Оператор"
		case models.RoleViewer:
			roleText = "Наблюдатель"
		default:
			roleText = "Неизвестная роль"
		}
	}

	welcomeText := fmt.Sprintf(`👋 *Добро пожаловать в панель управления Conveer!*

Вы авторизованы как: *%s*

Выберите раздел:`, roleText)

	keyboard := utils.MainMenuKeyboard(user)

	b.SendMessage(ctx, &botmodels.SendMessageParams{
		ChatID:      chatID,
		Text:        welcomeText,
		ParseMode:   botmodels.ParseModeMarkdown,
		ReplyMarkup: keyboard,
	})
}

func (h *CommandHandlers) HandleHelp(ctx context.Context, b *bot.Bot, update *botmodels.Update) {
	chatID := update.Message.Chat.ID
	user, _ := GetUserFromContext(ctx)

	var helpText strings.Builder
	helpText.WriteString("📚 *Доступные команды:*\n\n")
	helpText.WriteString("/start - Главное меню\n")
	helpText.WriteString("/help - Список команд\n")
	helpText.WriteString("/accounts [platform] - Список аккаунтов\n")
	helpText.WriteString("/stats [platform] - Статистика\n")

	if user != nil && user.Role != models.RoleViewer {
		helpText.WriteString("/export [platform] [format] - Экспорт аккаунтов\n")
		helpText.WriteString("/register [platform] [count] - Регистрация аккаунтов\n")
		helpText.WriteString("/warming [action] - Управление прогревом\n")
		helpText.WriteString("/proxies - Управление прокси\n")
		helpText.WriteString("/sms - Управление SMS\n")
	}

	if user != nil && user.Role == models.RoleAdmin {
		helpText.WriteString("/users - Управление пользователями\n")
	}

	b.SendMessage(ctx, &botmodels.SendMessageParams{
		ChatID:    chatID,
		Text:      helpText.String(),
		ParseMode: botmodels.ParseModeMarkdown,
	})
}

func (h *CommandHandlers) HandleAccounts(ctx context.Context, b *bot.Bot, update *botmodels.Update) {
	chatID := update.Message.Chat.ID
	args := strings.Fields(update.Message.Text)

	if len(args) < 2 {
		// Show platform selection
		keyboard := utils.PlatformSelectionKeyboard()
		b.SendMessage(ctx, &botmodels.SendMessageParams{
			ChatID:      chatID,
			Text:        "👥 Выберите платформу:",
			ReplyMarkup: keyboard,
		})
		return
	}

	platform := args[1]
	page := 1
	if len(args) > 2 {
		if p, err := strconv.Atoi(args[2]); err == nil {
			page = p
		}
	}

	// Get account stats
	stats, err := h.statsService.GetAccountStats(ctx, platform)
	if err != nil {
		b.SendMessage(ctx, &botmodels.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка получения данных аккаунтов",
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

	b.SendMessage(ctx, &botmodels.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   botmodels.ParseModeMarkdown,
		ReplyMarkup: keyboard,
	})
}

func (h *CommandHandlers) HandleExport(ctx context.Context, b *bot.Bot, update *botmodels.Update) {
	chatID := update.Message.Chat.ID
	args := strings.Fields(update.Message.Text)

	if len(args) < 2 {
		// Show platform selection
		keyboard := utils.PlatformSelectionKeyboard()
		b.SendMessage(ctx, &botmodels.SendMessageParams{
			ChatID:      chatID,
			Text:        "📤 Выберите платформу для экспорта:",
			ReplyMarkup: keyboard,
		})
		return
	}

	platform := args[1]

	if len(args) < 3 {
		// Show format selection
		keyboard := utils.ExportFormatKeyboard(platform)
		b.SendMessage(ctx, &botmodels.SendMessageParams{
			ChatID:      chatID,
			Text:        "📄 Выберите формат экспорта:",
			ReplyMarkup: keyboard,
		})
		return
	}

	format := models.ExportFormat(args[2])

	// Start export process
	b.SendMessage(ctx, &botmodels.SendMessageParams{
		ChatID: chatID,
		Text:   "⏳ Экспортирую аккаунты...",
	})

	// Export all accounts (simplified)
	data, filename, err := h.exportService.ExportAccounts(ctx, platform, []string{"all"}, format)
	if err != nil {
		b.SendMessage(ctx, &botmodels.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Ошибка экспорта: %v", err),
		})
		return
	}

	// Send file
	h.botService.SendDocument(ctx, chatID, data, filename)

	b.SendMessage(ctx, &botmodels.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("✅ Экспорт завершен!\nФайл: %s", filename),
	})
}

func (h *CommandHandlers) HandleStats(ctx context.Context, b *bot.Bot, update *botmodels.Update) {
	chatID := update.Message.Chat.ID
	args := strings.Fields(update.Message.Text)

	var text string
	var err error

	if len(args) < 2 {
		// Get overall stats
		stats, err := h.statsService.GetOverallStats(ctx)
		if err != nil {
			b.SendMessage(ctx, &botmodels.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Ошибка получения статистики",
			})
			return
		}
		text = utils.FormatOverallStats(stats)
	} else {
		// Get platform-specific stats
		platform := args[1]
		stats, err := h.statsService.GetDetailedStats(ctx, platform)
		if err != nil {
			b.SendMessage(ctx, &botmodels.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Ошибка получения статистики",
			})
			return
		}
		text = utils.FormatDetailedStats(stats)
	}

	b.SendMessage(ctx, &botmodels.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: botmodels.ParseModeMarkdown,
	})
}

func (h *CommandHandlers) HandleRegister(ctx context.Context, b *bot.Bot, update *botmodels.Update) {
	chatID := update.Message.Chat.ID
	args := strings.Fields(update.Message.Text)

	if len(args) < 3 {
		b.SendMessage(ctx, &botmodels.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Использование: /register [platform] [count]\nПример: /register vk 10",
		})
		return
	}

	platform := args[1]
	count, err := strconv.Atoi(args[2])
	if err != nil {
		b.SendMessage(ctx, &botmodels.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Некорректное количество аккаунтов",
		})
		return
	}

	// Start registration
	if err := h.commandService.StartRegistration(ctx, platform, count); err != nil {
		b.SendMessage(ctx, &botmodels.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("❌ Ошибка запуска регистрации: %v", err),
		})
		return
	}

	b.SendMessage(ctx, &botmodels.SendMessageParams{
		ChatID: chatID,
		Text:   fmt.Sprintf("✅ Запущена регистрация %d аккаунтов на %s.\n\nВы получите уведомление по завершении.", count, strings.ToUpper(platform)),
	})
}

func (h *CommandHandlers) HandleWarming(ctx context.Context, b *bot.Bot, update *botmodels.Update) {
	chatID := update.Message.Chat.ID
	args := strings.Fields(update.Message.Text)

	if len(args) < 2 {
		keyboard := utils.WarmingActionsKeyboard()
		b.SendMessage(ctx, &botmodels.SendMessageParams{
			ChatID:      chatID,
			Text:        "🔥 Управление прогревом:",
			ReplyMarkup: keyboard,
		})
		return
	}

	action := args[1]

	switch action {
	case "start":
		if len(args) < 6 {
			b.SendMessage(ctx, &botmodels.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Использование: /warming start [account_id] [platform] [scenario] [days]\nПример: /warming start ACC123 vk standard 7",
			})
			return
		}
		accountID := args[2]
		platform := args[3]
		scenario := args[4]
		days, err := strconv.Atoi(args[5])
		if err != nil || days <= 0 {
			b.SendMessage(ctx, &botmodels.SendMessageParams{
				ChatID: chatID,
				Text:   "❌ Некорректное количество дней. Укажите положительное целое число.",
			})
			return
		}

		err := h.commandService.StartWarming(ctx, accountID, platform, scenario, days)
		if err != nil {
			b.SendMessage(ctx, &botmodels.SendMessageParams{
				ChatID: chatID,
				Text:   fmt.Sprintf("❌ Ошибка запуска прогрева: %v", err),
			})
			return
		}

		b.SendMessage(ctx, &botmodels.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("✅ Прогрев запущен для аккаунта %s", accountID),
		})

	case "pause", "resume", "stop":
		if len(args) < 3 {
			b.SendMessage(ctx, &botmodels.SendMessageParams{
				ChatID: chatID,
				Text:   fmt.Sprintf("❌ Использование: /warming %s [task_id]", action),
			})
			return
		}
		taskID := args[2]

		var err error
		switch action {
		case "pause":
			err = h.commandService.PauseWarming(ctx, taskID)
		case "resume":
			err = h.commandService.ResumeWarming(ctx, taskID)
		case "stop":
			err = h.commandService.StopWarming(ctx, taskID)
		}

		if err != nil {
			b.SendMessage(ctx, &botmodels.SendMessageParams{
				ChatID: chatID,
				Text:   fmt.Sprintf("❌ Ошибка: %v", err),
			})
			return
		}

		b.SendMessage(ctx, &botmodels.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("✅ Прогрев %s для задачи %s", action, taskID),
		})

	default:
		b.SendMessage(ctx, &botmodels.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Неизвестное действие. Доступны: start, pause, resume, stop",
		})
	}
}

func (h *CommandHandlers) HandleProxies(ctx context.Context, b *bot.Bot, update *botmodels.Update) {
	chatID := update.Message.Chat.ID

	stats, err := h.statsService.GetProxyStats(ctx)
	if err != nil {
		b.SendMessage(ctx, &botmodels.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка получения статистики прокси",
		})
		return
	}

	text := utils.FormatProxyStats(stats)
	keyboard := utils.ProxyActionsKeyboard()

	b.SendMessage(ctx, &botmodels.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   botmodels.ParseModeMarkdown,
		ReplyMarkup: keyboard,
	})
}

func (h *CommandHandlers) HandleSMS(ctx context.Context, b *bot.Bot, update *botmodels.Update) {
	chatID := update.Message.Chat.ID

	stats, err := h.statsService.GetSMSStats(ctx)
	if err != nil {
		b.SendMessage(ctx, &botmodels.SendMessageParams{
			ChatID: chatID,
			Text:   "❌ Ошибка получения статистики SMS",
		})
		return
	}

	text := utils.FormatSMSStats(stats)
	keyboard := utils.SMSActionsKeyboard()

	b.SendMessage(ctx, &botmodels.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   botmodels.ParseModeMarkdown,
		ReplyMarkup: keyboard,
	})
}