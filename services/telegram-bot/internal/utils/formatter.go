package utils

import (
	"fmt"
	"strings"
	"time"

	"github.com/conveer/conveer/services/telegram-bot/internal/models"
	"github.com/conveer/conveer/services/telegram-bot/internal/service"
)

func FormatAccountsTable(accounts []*models.Account) string {
	if len(accounts) == 0 {
		return "Нет аккаунтов для отображения"
	}

	var builder strings.Builder
	builder.WriteString("```\n")
	builder.WriteString("ID       | Phone        | Status  | Created\n")
	builder.WriteString("---------|--------------|---------|----------\n")

	for _, account := range accounts {
		id := account.ID
		if len(id) > 6 {
			id = id[:6]
		}

		statusEmoji := getStatusEmoji(account.Status)
		status := fmt.Sprintf("%s %s", statusEmoji, account.Status)

		builder.WriteString(fmt.Sprintf("%-8s | %-12s | %-9s | %s\n",
			id,
			account.Phone,
			status,
			account.CreatedAt.Format("2006-01-02"),
		))
	}

	builder.WriteString("```")
	return builder.String()
}

func FormatOverallStats(stats *service.OverallStats) string {
	var builder strings.Builder

	builder.WriteString("📊 *Общая статистика*\n\n")

	// Total accounts
	builder.WriteString(fmt.Sprintf("*Всего аккаунтов:* %d\n", stats.TotalAccounts))

	// By platform
	if len(stats.AccountsByPlatform) > 0 {
		for platform, count := range stats.AccountsByPlatform {
			percentage := float64(count) * 100 / float64(stats.TotalAccounts)
			builder.WriteString(fmt.Sprintf("├─ %s: %d (%.0f%%)\n", strings.ToUpper(platform), count, percentage))
		}
	}

	builder.WriteString("\n*По статусам:*\n")
	totalByStatus := int64(0)
	for _, count := range stats.AccountsByStatus {
		totalByStatus += count
	}

	for status, count := range stats.AccountsByStatus {
		emoji := getStatusEmoji(status)
		percentage := float64(count) * 100 / float64(totalByStatus)
		builder.WriteString(fmt.Sprintf("%s %s: %d (%.0f%%)\n", emoji, capitalizeFirst(status), count, percentage))
	}

	// Warming stats
	builder.WriteString("\n*Прогрев:*\n")
	builder.WriteString(fmt.Sprintf("▶️ В процессе: %d\n", stats.WarmingTasks.InProgress))
	builder.WriteString(fmt.Sprintf("✅ Завершено: %d\n", stats.WarmingTasks.Completed))
	builder.WriteString(fmt.Sprintf("❌ Ошибки: %d\n", stats.WarmingTasks.Failed))

	// Proxy stats
	builder.WriteString("\n*Прокси:*\n")
	builder.WriteString(fmt.Sprintf("🟢 Активные: %d/%d\n", stats.ProxyStats.Active, stats.ProxyStats.Total))
	builder.WriteString(fmt.Sprintf("🔴 Истекшие: %d\n", stats.ProxyStats.Expired))
	builder.WriteString(fmt.Sprintf("⚠️ Забаненные: %d\n", stats.ProxyStats.Banned))

	// SMS stats
	builder.WriteString("\n*SMS:*\n")
	builder.WriteString(fmt.Sprintf("💰 Потрачено сегодня: %.2f руб.\n", stats.SMSStats.TotalSpent))
	builder.WriteString(fmt.Sprintf("📱 Активаций: %d\n", stats.SMSStats.ActivationsToday))

	// Success rate
	successBar := generateProgressBar(stats.SuccessRate)
	builder.WriteString(fmt.Sprintf("\n*Успешность:* %s %.0f%%\n", successBar, stats.SuccessRate*100))

	// Last 24 hours
	builder.WriteString(fmt.Sprintf("*За 24ч:* +%d аккаунтов\n", stats.Last24HoursCreated))

	return builder.String()
}

func FormatDetailedStats(stats *service.DetailedStats) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("📊 *Статистика %s*\n\n", strings.ToUpper(stats.Platform)))

	// Status distribution
	builder.WriteString("*Распределение по статусам:*\n")
	for status, count := range stats.StatusDistribution {
		emoji := getStatusEmoji(status)
		builder.WriteString(fmt.Sprintf("%s %s: %d\n", emoji, capitalizeFirst(status), count))
	}

	// Success rate
	successBar := generateProgressBar(stats.SuccessRate)
	builder.WriteString(fmt.Sprintf("\n*Успешность регистрации:* %s %.0f%%\n", successBar, stats.SuccessRate*100))

	// Average warming duration
	builder.WriteString(fmt.Sprintf("*Средняя длительность прогрева:* %.1f дней\n", stats.AvgWarmingDuration))

	// Top errors
	if len(stats.TopErrors) > 0 {
		builder.WriteString("\n*Топ ошибок:*\n")
		for i, err := range stats.TopErrors {
			if i >= 5 {
				break
			}
			builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, err))
		}
	}

	// Activity graph
	if len(stats.Last7DaysActivity) > 0 {
		builder.WriteString("\n*Активность за 7 дней:*\n")
		for date, count := range stats.Last7DaysActivity {
			builder.WriteString(fmt.Sprintf("%s: %d\n", date, count))
		}
	}

	return builder.String()
}

func FormatProxyStats(stats *service.ProxyStats) string {
	var builder strings.Builder

	builder.WriteString("🌐 *Статистика прокси*\n\n")
	builder.WriteString(fmt.Sprintf("*Всего:* %d\n", stats.Total))
	builder.WriteString(fmt.Sprintf("🟢 *Активные:* %d\n", stats.Active))
	builder.WriteString(fmt.Sprintf("🔴 *Истекшие:* %d\n", stats.Expired))
	builder.WriteString(fmt.Sprintf("⚠️ *Забаненные:* %d\n", stats.Banned))

	// Usage percentage
	if stats.Total > 0 {
		usage := float64(stats.Active) * 100 / float64(stats.Total)
		usageBar := generateProgressBar(usage / 100)
		builder.WriteString(fmt.Sprintf("\n*Использование:* %s %.0f%%\n", usageBar, usage))
	}

	return builder.String()
}

func FormatSMSStats(stats *service.SMSStats) string {
	var builder strings.Builder

	builder.WriteString("📱 *Статистика SMS*\n\n")
	builder.WriteString(fmt.Sprintf("💰 *Баланс:* %.2f руб.\n", stats.Balance))
	builder.WriteString(fmt.Sprintf("💸 *Потрачено сегодня:* %.2f руб.\n", stats.TotalSpent))
	builder.WriteString(fmt.Sprintf("📊 *Активаций сегодня:* %d\n", stats.ActivationsToday))

	// Balance warning
	if stats.Balance < 1000 {
		builder.WriteString("\n⚠️ *Внимание:* Низкий баланс!")
	}

	return builder.String()
}

func FormatAlert(event *models.Event) string {
	var emoji string
	switch event.Priority {
	case "critical":
		emoji = "🚨"
	case "warning":
		emoji = "⚠️"
	default:
		emoji = "ℹ️"
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%s [%s] ", emoji, strings.ToUpper(event.Priority)))

	switch event.Type {
	case "account.banned":
		builder.WriteString(fmt.Sprintf("Аккаунт %s %s забанен", event.Platform, event.AccountID))
		if event.Message != "" {
			builder.WriteString(fmt.Sprintf(". Причина: %s", event.Message))
		}
	case "task.failed":
		builder.WriteString(fmt.Sprintf("Задача %s провалилась", event.TaskID))
		if event.Error != "" {
			builder.WriteString(fmt.Sprintf(". Ошибка: %s", event.Error))
		}
	case "sms.balance.low":
		builder.WriteString("Низкий баланс SMS-Activate")
		if balance, ok := event.Metadata["balance"].(float64); ok {
			builder.WriteString(fmt.Sprintf(". Остаток: %.2f руб.", balance))
		}
	case "proxy.rotation.failed":
		builder.WriteString("Ошибка ротации прокси")
		if event.AccountID != "" {
			builder.WriteString(fmt.Sprintf(" для аккаунта %s", event.AccountID))
		}
	case "manual_intervention":
		builder.WriteString("Требуется ручное вмешательство")
		if event.Message != "" {
			builder.WriteString(fmt.Sprintf(": %s", event.Message))
		}
	default:
		builder.WriteString(event.Type)
		if event.Message != "" {
			builder.WriteString(fmt.Sprintf(": %s", event.Message))
		}
	}

	builder.WriteString(fmt.Sprintf("\n⏰ %s", event.Timestamp.Format("15:04:05")))

	return builder.String()
}

// Helper functions

func getStatusEmoji(status string) string {
	switch status {
	case "ready":
		return "✅"
	case "warming":
		return "🔥"
	case "banned":
		return "❌"
	case "creating":
		return "🆕"
	case "created":
		return "✔️"
	case "error":
		return "⚠️"
	default:
		return "❓"
	}
}

func generateProgressBar(percentage float64) string {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 1 {
		percentage = 1
	}

	filled := int(percentage * 10)
	empty := 10 - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return bar
}

func capitalizeFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dд %dч %dм", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dч %dм", hours, minutes)
	}
	return fmt.Sprintf("%dм", minutes)
}
