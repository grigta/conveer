package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"time"

	"github.com/conveer/conveer/services/telegram-bot/internal/models"
	"github.com/conveer/conveer/services/telegram-bot/internal/repository"
)

type ExportService interface {
	ExportAccounts(ctx context.Context, platform string, accountIDs []string, format models.ExportFormat) ([]byte, string, error)
}

type exportService struct {
	exportRepo repository.ExportRepository
}

func NewExportService(exportRepo repository.ExportRepository) ExportService {
	return &exportService{
		exportRepo: exportRepo,
	}
}

func (s *exportService) ExportAccounts(ctx context.Context, platform string, accountIDs []string, format models.ExportFormat) ([]byte, string, error) {
	// Get accounts data
	accounts, err := s.exportRepo.GetAccountsForExport(ctx, platform, accountIDs)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get accounts: %w", err)
	}

	if len(accounts) == 0 {
		return nil, "", fmt.Errorf("no accounts found for export")
	}

	// Generate filename
	timestamp := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s_%s_%s", platform, format, timestamp)

	// Export based on format
	switch platform {
	case "telegram":
		switch format {
		case models.FormatTData:
			data, err := s.exportTelegramTData(ctx, accounts)
			if err != nil {
				return nil, "", err
			}
			return data, filename + ".zip", nil

		case models.FormatTelethon:
			data, err := s.exportTelegramTelethon(ctx, accounts)
			if err != nil {
				return nil, "", err
			}
			return data, filename + ".zip", nil

		case models.FormatPyrogram:
			data, err := s.exportTelegramPyrogram(ctx, accounts)
			if err != nil {
				return nil, "", err
			}
			return data, filename + ".zip", nil

		case models.FormatJSON:
			data, err := s.exportJSON(accounts)
			if err != nil {
				return nil, "", err
			}
			return data, filename + ".json", nil

		default:
			return nil, "", fmt.Errorf("unsupported format %s for platform %s", format, platform)
		}

	case "vk", "mail", "max":
		switch format {
		case models.FormatJSON:
			data, err := s.exportJSON(accounts)
			if err != nil {
				return nil, "", err
			}
			return data, filename + ".json", nil

		case models.FormatCSV:
			data, err := s.exportCSV(accounts)
			if err != nil {
				return nil, "", err
			}
			return data, filename + ".csv", nil

		default:
			return nil, "", fmt.Errorf("unsupported format %s for platform %s", format, platform)
		}

	default:
		return nil, "", fmt.Errorf("unsupported platform: %s", platform)
	}
}

func (s *exportService) exportTelegramTData(ctx context.Context, accounts []*models.Account) ([]byte, error) {
	// Create a ZIP archive
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	for _, account := range accounts {
		// Get session data
		sessionData, err := s.exportRepo.GetSessionData(ctx, "telegram", account.ID)
		if err != nil {
			continue // Skip failed accounts
		}

		// Create TData structure for this account
		phone := account.Phone
		if phone == "" {
			phone = "unknown"
		}

		// Add TData files to ZIP
		// This is a simplified version - actual TData format is more complex
		folderName := fmt.Sprintf("telegram_%s/tdata/", phone)

		// Create key_datas file
		keyDatasPath := folderName + "key_datas"
		f, err := w.Create(keyDatasPath)
		if err != nil {
			continue
		}
		f.Write([]byte("sample_key_data")) // Placeholder

		// Create session file
		sessionPath := folderName + "D877F783D5D3EF8C"
		f, err = w.Create(sessionPath)
		if err != nil {
			continue
		}
		f.Write([]byte(sessionData.SessionString)) // Simplified

		// Create maps file
		mapsPath := folderName + "maps"
		f, err = w.Create(mapsPath)
		if err != nil {
			continue
		}
		f.Write([]byte("0")) // Placeholder

		// Create settings file
		settingsPath := folderName + "settings"
		f, err = w.Create(settingsPath)
		if err != nil {
			continue
		}
		f.Write([]byte("{}")) // Placeholder JSON
	}

	w.Close()
	return buf.Bytes(), nil
}

func (s *exportService) exportTelegramTelethon(ctx context.Context, accounts []*models.Account) ([]byte, error) {
	// Create a ZIP archive with .session files
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	for _, account := range accounts {
		sessionData, err := s.exportRepo.GetSessionData(ctx, "telegram", account.ID)
		if err != nil {
			continue
		}

		phone := account.Phone
		if phone == "" {
			phone = "unknown"
		}

		// Create .session file (SQLite format)
		// This is a placeholder - actual implementation would create a proper SQLite file
		filename := fmt.Sprintf("%s.session", phone)
		f, err := w.Create(filename)
		if err != nil {
			continue
		}

		// Write simplified session data
		sessionContent := fmt.Sprintf("SQLite format\ndc_id: %d\nauth_key: %s\nserver_address: %s\nport: %d",
			sessionData.DCID,
			sessionData.SessionString,
			sessionData.ServerAddress,
			sessionData.Port,
		)
		f.Write([]byte(sessionContent))
	}

	w.Close()
	return buf.Bytes(), nil
}

func (s *exportService) exportTelegramPyrogram(ctx context.Context, accounts []*models.Account) ([]byte, error) {
	// Similar to Telethon but with Pyrogram format
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	for _, account := range accounts {
		sessionData, err := s.exportRepo.GetSessionData(ctx, "telegram", account.ID)
		if err != nil {
			continue
		}

		phone := account.Phone
		if phone == "" {
			phone = "unknown"
		}

		// Create .session file (Pyrogram SQLite format)
		filename := fmt.Sprintf("%s.session", phone)
		f, err := w.Create(filename)
		if err != nil {
			continue
		}

		// Write simplified session data
		sessionContent := fmt.Sprintf("Pyrogram SQLite format\ndc_id: %d\nauth_key: %s\ntest_mode: false",
			sessionData.DCID,
			sessionData.SessionString,
		)
		f.Write([]byte(sessionContent))
	}

	w.Close()
	return buf.Bytes(), nil
}

func (s *exportService) exportJSON(accounts []*models.Account) ([]byte, error) {
	data, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal accounts to JSON: %w", err)
	}
	return data, nil
}

func (s *exportService) exportCSV(accounts []*models.Account) ([]byte, error) {
	buf := new(bytes.Buffer)
	w := csv.NewWriter(buf)

	// Write header
	header := []string{"ID", "Phone", "Email", "Password", "Username", "UserID", "Status", "ProxyID", "CreatedAt"}
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data
	for _, account := range accounts {
		record := []string{
			account.ID,
			account.Phone,
			account.Email,
			account.Password,
			account.Username,
			account.UserID,
			account.Status,
			account.ProxyID,
			account.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if err := w.Write(record); err != nil {
			return nil, fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("CSV writer error: %w", err)
	}

	return buf.Bytes(), nil
}
