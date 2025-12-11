package utils

import (
	"fmt"

	"github.com/conveer/conveer/services/telegram-bot/internal/models"
	botmodels "github.com/go-telegram/bot/models"
)

func MainMenuKeyboard(user *models.TelegramBotUser) *botmodels.InlineKeyboardMarkup {
	buttons := [][]botmodels.InlineKeyboardButton{
		{
			{Text: "📊 Статистика", CallbackData: "menu:stats"},
			{Text: "👥 Аккаунты", CallbackData: "menu:accounts"},
		},
	}

	if user != nil && user.Role != models.RoleViewer {
		buttons = append(buttons, []botmodels.InlineKeyboardButton{
			{Text: "⚙️ Управление", CallbackData: "menu:management"},
			{Text: "📤 Экспорт", CallbackData: "menu:export"},
		})
	}

	if user != nil && user.Role == models.RoleAdmin {
		buttons = append(buttons, []botmodels.InlineKeyboardButton{
			{Text: "🔧 Настройки", CallbackData: "menu:settings"},
		})
	}

	return &botmodels.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func PlatformSelectionKeyboard() *botmodels.InlineKeyboardMarkup {
	return &botmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botmodels.InlineKeyboardButton{
			{
				{Text: "VK", CallbackData: "accounts:vk"},
				{Text: "Telegram", CallbackData: "accounts:telegram"},
			},
			{
				{Text: "Mail.ru", CallbackData: "accounts:mail"},
				{Text: "Max", CallbackData: "accounts:max"},
			},
			{
				{Text: "Все", CallbackData: "accounts:all"},
			},
			{
				{Text: "◀️ Назад", CallbackData: "menu:back"},
			},
		},
	}
}

func ExportFormatKeyboard(platform string) *botmodels.InlineKeyboardMarkup {
	var buttons [][]botmodels.InlineKeyboardButton

	if platform == "telegram" {
		buttons = [][]botmodels.InlineKeyboardButton{
			{
				{Text: "TData", CallbackData: fmt.Sprintf("export:%s:tdata", platform)},
				{Text: "Telethon .session", CallbackData: fmt.Sprintf("export:%s:telethon", platform)},
			},
			{
				{Text: "Pyrogram .session", CallbackData: fmt.Sprintf("export:%s:pyrogram", platform)},
				{Text: "JSON", CallbackData: fmt.Sprintf("export:%s:json", platform)},
			},
		}
	} else {
		buttons = [][]botmodels.InlineKeyboardButton{
			{
				{Text: "JSON", CallbackData: fmt.Sprintf("export:%s:json", platform)},
				{Text: "CSV", CallbackData: fmt.Sprintf("export:%s:csv", platform)},
			},
		}
	}

	buttons = append(buttons, []botmodels.InlineKeyboardButton{
		{Text: "◀️ Назад", CallbackData: "menu:export"},
	})

	return &botmodels.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func PaginationKeyboard(page, totalPages int, prefix string) *botmodels.InlineKeyboardMarkup {
	buttons := [][]botmodels.InlineKeyboardButton{}

	navigationButtons := []botmodels.InlineKeyboardButton{}

	if page > 1 {
		navigationButtons = append(navigationButtons, botmodels.InlineKeyboardButton{
			Text:         "◀️ Назад",
			CallbackData: fmt.Sprintf("%s:page:%d", prefix, page-1),
		})
	}

	navigationButtons = append(navigationButtons, botmodels.InlineKeyboardButton{
		Text:         fmt.Sprintf("Страница %d/%d", page, totalPages),
		CallbackData: "noop",
	})

	if page < totalPages {
		navigationButtons = append(navigationButtons, botmodels.InlineKeyboardButton{
			Text:         "Вперед ▶️",
			CallbackData: fmt.Sprintf("%s:page:%d", prefix, page+1),
		})
	}

	buttons = append(buttons, navigationButtons)

	// Add export and back buttons
	buttons = append(buttons, []botmodels.InlineKeyboardButton{
		{Text: "📤 Экспорт", CallbackData: fmt.Sprintf("export:platform:%s", prefix)},
		{Text: "◀️ В меню", CallbackData: "menu:back"},
	})

	return &botmodels.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func StatsActionsKeyboard() *botmodels.InlineKeyboardMarkup {
	return &botmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botmodels.InlineKeyboardButton{
			{
				{Text: "🔄 Обновить", CallbackData: "stats:refresh"},
				{Text: "📊 Графики", CallbackData: "stats:graphs"},
			},
			{
				{Text: "VK", CallbackData: "stats:platform:vk"},
				{Text: "Telegram", CallbackData: "stats:platform:telegram"},
			},
			{
				{Text: "Mail.ru", CallbackData: "stats:platform:mail"},
				{Text: "Max", CallbackData: "stats:platform:max"},
			},
			{
				{Text: "◀️ В меню", CallbackData: "menu:back"},
			},
		},
	}
}

func ManagementMenuKeyboard(user *models.TelegramBotUser) *botmodels.InlineKeyboardMarkup {
	buttons := [][]botmodels.InlineKeyboardButton{
		{
			{Text: "➕ Регистрация", CallbackData: "management:register"},
			{Text: "🔥 Прогрев", CallbackData: "management:warming"},
		},
		{
			{Text: "🌐 Прокси", CallbackData: "management:proxies"},
			{Text: "📱 SMS", CallbackData: "management:sms"},
		},
	}

	if user != nil && user.Role == models.RoleAdmin {
		buttons = append(buttons, []botmodels.InlineKeyboardButton{
			{Text: "👤 Пользователи", CallbackData: "management:users"},
		})
	}

	buttons = append(buttons, []botmodels.InlineKeyboardButton{
		{Text: "◀️ В меню", CallbackData: "menu:back"},
	})

	return &botmodels.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func WarmingActionsKeyboard() *botmodels.InlineKeyboardMarkup {
	return &botmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botmodels.InlineKeyboardButton{
			{
				{Text: "▶️ Запустить", CallbackData: "warming:start"},
				{Text: "⏸️ Приостановить", CallbackData: "warming:pause"},
			},
			{
				{Text: "▶️ Возобновить", CallbackData: "warming:resume"},
				{Text: "⏹️ Остановить", CallbackData: "warming:stop"},
			},
			{
				{Text: "📊 Статус", CallbackData: "warming:status"},
			},
			{
				{Text: "◀️ Назад", CallbackData: "menu:management"},
			},
		},
	}
}

func WarmingStartKeyboard() *botmodels.InlineKeyboardMarkup {
	return &botmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botmodels.InlineKeyboardButton{
			{
				{Text: "Базовый", CallbackData: "warming:scenario:basic"},
				{Text: "Продвинутый", CallbackData: "warming:scenario:advanced"},
			},
			{
				{Text: "Кастомный", CallbackData: "warming:scenario:custom"},
			},
			{
				{Text: "◀️ Назад", CallbackData: "management:warming"},
			},
		},
	}
}

func WarmingDurationKeyboard(scenario string) *botmodels.InlineKeyboardMarkup {
	return &botmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botmodels.InlineKeyboardButton{
			{
				{Text: "14-30 дней", CallbackData: fmt.Sprintf("warming:duration:%s:21", scenario)},
				{Text: "30-60 дней", CallbackData: fmt.Sprintf("warming:duration:%s:45", scenario)},
			},
			{
				{Text: "◀️ Назад", CallbackData: "warming:start"},
			},
		},
	}
}

func ProxyActionsKeyboard() *botmodels.InlineKeyboardMarkup {
	return &botmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botmodels.InlineKeyboardButton{
			{
				{Text: "➕ Выделить прокси", CallbackData: "proxy:allocate"},
				{Text: "➖ Освободить прокси", CallbackData: "proxy:release"},
			},
			{
				{Text: "🏥 Проверить здоровье", CallbackData: "proxy:health"},
				{Text: "📊 Статистика", CallbackData: "proxy:stats"},
			},
			{
				{Text: "◀️ Назад", CallbackData: "menu:management"},
			},
		},
	}
}

func ProxyTypeKeyboard() *botmodels.InlineKeyboardMarkup {
	return &botmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botmodels.InlineKeyboardButton{
			{
				{Text: "📱 Мобильный", CallbackData: "proxy:type:mobile"},
				{Text: "🏠 Резидентный", CallbackData: "proxy:type:residential"},
			},
			{
				{Text: "◀️ Назад", CallbackData: "management:proxies"},
			},
		},
	}
}

func SMSActionsKeyboard() *botmodels.InlineKeyboardMarkup {
	return &botmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botmodels.InlineKeyboardButton{
			{
				{Text: "📱 Купить номер", CallbackData: "sms:purchase"},
				{Text: "❌ Отменить активацию", CallbackData: "sms:cancel"},
			},
			{
				{Text: "💰 Баланс", CallbackData: "sms:balance"},
				{Text: "📊 Статистика", CallbackData: "sms:stats"},
			},
			{
				{Text: "◀️ Назад", CallbackData: "menu:management"},
			},
		},
	}
}

func SMSServiceKeyboard() *botmodels.InlineKeyboardMarkup {
	return &botmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botmodels.InlineKeyboardButton{
			{
				{Text: "VK", CallbackData: "sms:service:vk"},
				{Text: "Telegram", CallbackData: "sms:service:telegram"},
			},
			{
				{Text: "Mail.ru", CallbackData: "sms:service:mail"},
				{Text: "Max", CallbackData: "sms:service:max"},
			},
			{
				{Text: "◀️ Назад", CallbackData: "management:sms"},
			},
		},
	}
}

func SMSCountryKeyboard(service string) *botmodels.InlineKeyboardMarkup {
	return &botmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botmodels.InlineKeyboardButton{
			{
				{Text: "🇷🇺 Россия", CallbackData: fmt.Sprintf("sms:country:%s:ru", service)},
				{Text: "🇺🇦 Украина", CallbackData: fmt.Sprintf("sms:country:%s:ua", service)},
			},
			{
				{Text: "🇰🇿 Казахстан", CallbackData: fmt.Sprintf("sms:country:%s:kz", service)},
				{Text: "🇧🇾 Беларусь", CallbackData: fmt.Sprintf("sms:country:%s:by", service)},
			},
			{
				{Text: "◀️ Назад", CallbackData: "sms:purchase"},
			},
		},
	}
}
