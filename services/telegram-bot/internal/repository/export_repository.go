package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/conveer/conveer/services/telegram-bot/internal/models"
	"github.com/conveer/conveer/pkg/crypto"
	vkpb "github.com/conveer/conveer/services/vk-service/proto"
	telegrampb "github.com/conveer/conveer/services/telegram-service/proto"
)

type ExportRepository interface {
	GetAccountsForExport(ctx context.Context, platform string, accountIDs []string) ([]*models.Account, error)
	GetSessionData(ctx context.Context, platform string, accountID string) (*models.SessionData, error)
}

// ExportClients contains the gRPC clients needed for export operations
type ExportClients struct {
	VKServiceClient       vkpb.VKServiceClient
	TelegramServiceClient telegrampb.TelegramServiceClient
	Encryptor             *crypto.Encryptor
}

type exportRepository struct {
	clients *ExportClients
}

func NewExportRepository(clients *ExportClients) ExportRepository {
	return &exportRepository{
		clients: clients,
	}
}

func (r *exportRepository) GetAccountsForExport(ctx context.Context, platform string, accountIDs []string) ([]*models.Account, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	switch platform {
	case "vk":
		return r.getVKAccounts(ctx, accountIDs)
	case "telegram":
		return r.getTelegramAccounts(ctx, accountIDs)
	case "mail", "max":
		return nil, fmt.Errorf("platform %s not yet implemented", platform)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}

func (r *exportRepository) getVKAccounts(ctx context.Context, accountIDs []string) ([]*models.Account, error) {
	if r.clients.VKServiceClient == nil {
		return nil, fmt.Errorf("VK service not available")
	}

	var accounts []*models.Account

	if len(accountIDs) == 0 {
		// Get all accounts
		resp, err := r.clients.VKServiceClient.ListAccounts(ctx, &vkpb.ListAccountsRequest{
			Limit: 1000,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list VK accounts: %w", err)
		}

		for _, pbAccount := range resp.Accounts {
			account, err := r.convertVKAccount(pbAccount)
			if err != nil {
				continue
			}
			accounts = append(accounts, account)
		}
	} else {
		// Get specific accounts by IDs
		for _, accountID := range accountIDs {
			pbAccount, err := r.clients.VKServiceClient.GetAccount(ctx, &vkpb.GetAccountRequest{
				AccountId: accountID,
			})
			if err != nil {
				continue
			}

			account, err := r.convertVKAccount(pbAccount)
			if err != nil {
				continue
			}
			accounts = append(accounts, account)
		}
	}

	return accounts, nil
}

func (r *exportRepository) getTelegramAccounts(ctx context.Context, accountIDs []string) ([]*models.Account, error) {
	if r.clients.TelegramServiceClient == nil {
		return nil, fmt.Errorf("Telegram service not available")
	}

	var accounts []*models.Account

	if len(accountIDs) == 0 {
		// Get all accounts
		resp, err := r.clients.TelegramServiceClient.ListAccounts(ctx, &telegrampb.ListAccountsRequest{
			Limit: 1000,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list Telegram accounts: %w", err)
		}

		for _, pbAccount := range resp.Accounts {
			account, err := r.convertTelegramAccount(pbAccount)
			if err != nil {
				continue
			}
			accounts = append(accounts, account)
		}
	} else {
		// Get specific accounts by IDs
		for _, accountID := range accountIDs {
			pbAccount, err := r.clients.TelegramServiceClient.GetAccount(ctx, &telegrampb.GetAccountRequest{
				AccountId: accountID,
			})
			if err != nil {
				continue
			}

			account, err := r.convertTelegramAccount(pbAccount)
			if err != nil {
				continue
			}
			accounts = append(accounts, account)
		}
	}

	return accounts, nil
}

func (r *exportRepository) convertVKAccount(pbAccount *vkpb.Account) (*models.Account, error) {
	account := &models.Account{
		ID:       pbAccount.Id,
		Platform: "vk",
		Username: pbAccount.Username,
		UserID:   pbAccount.UserId,
		Status:   pbAccount.Status,
		ProxyID:  pbAccount.ProxyId,
	}

	// Decrypt phone
	if pbAccount.Phone != "" && r.clients.Encryptor != nil {
		phone, err := r.clients.Encryptor.Decrypt(pbAccount.Phone)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt phone for account %s: %w", pbAccount.Id, err)
		}
		account.Phone = phone
	}

	// Decrypt email
	if pbAccount.Email != "" && r.clients.Encryptor != nil {
		email, err := r.clients.Encryptor.Decrypt(pbAccount.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt email for account %s: %w", pbAccount.Id, err)
		}
		account.Email = email
	}

	// Convert timestamps
	if pbAccount.CreatedAt != nil {
		account.CreatedAt = pbAccount.CreatedAt.AsTime()
	}

	// Copy metadata
	if account.Metadata == nil {
		account.Metadata = make(map[string]interface{})
	}
	for key, value := range pbAccount.Fingerprint {
		account.Metadata[key] = value
	}

	return account, nil
}

func (r *exportRepository) convertTelegramAccount(pbAccount *telegrampb.Account) (*models.Account, error) {
	account := &models.Account{
		ID:       pbAccount.Id,
		Platform: "telegram",
		Username: pbAccount.Username,
		UserID:   pbAccount.UserId,
		Status:   pbAccount.Status,
		ProxyID:  pbAccount.ProxyId,
	}

	// Decrypt phone
	if pbAccount.Phone != "" && r.clients.Encryptor != nil {
		phone, err := r.clients.Encryptor.Decrypt(pbAccount.Phone)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt phone for account %s: %w", pbAccount.Id, err)
		}
		account.Phone = phone
	}

	// Convert timestamps
	if pbAccount.CreatedAt != nil {
		account.CreatedAt = pbAccount.CreatedAt.AsTime()
	}

	// Copy metadata
	if account.Metadata == nil {
		account.Metadata = make(map[string]interface{})
	}
	for key, value := range pbAccount.Fingerprint {
		account.Metadata[key] = value
	}

	return account, nil
}

func (r *exportRepository) GetSessionData(ctx context.Context, platform string, accountID string) (*models.SessionData, error) {
	if platform != "telegram" {
		return nil, fmt.Errorf("session data only available for telegram")
	}

	if r.clients.TelegramServiceClient == nil {
		return nil, fmt.Errorf("Telegram service not available")
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Get session data via gRPC
	pbSessionData, err := r.clients.TelegramServiceClient.GetSessionData(ctx, &telegrampb.GetSessionDataRequest{
		AccountId: accountID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get session data for account %s: %w", accountID, err)
	}

	sessionData := &models.SessionData{
		AccountID:     pbSessionData.AccountId,
		DCID:          int(pbSessionData.DcId),
		ServerAddress: pbSessionData.ServerAddress,
		Port:          int(pbSessionData.Port),
	}

	// Decrypt phone
	if pbSessionData.Phone != "" && r.clients.Encryptor != nil {
		phone, err := r.clients.Encryptor.Decrypt(pbSessionData.Phone)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt phone for account %s: %w", accountID, err)
		}
		sessionData.Phone = phone
	}

	// Decrypt session string
	if pbSessionData.SessionString != "" && r.clients.Encryptor != nil {
		sessionString, err := r.clients.Encryptor.Decrypt(pbSessionData.SessionString)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt session string for account %s: %w", accountID, err)
		}
		sessionData.SessionString = sessionString
	}

	// Decrypt cookies
	if pbSessionData.Cookies != "" && r.clients.Encryptor != nil {
		cookies, err := r.clients.Encryptor.Decrypt(pbSessionData.Cookies)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt cookies for account %s: %w", accountID, err)
		}
		sessionData.Cookies = cookies
	}

	// Decrypt localStorage
	if pbSessionData.LocalStorage != "" && r.clients.Encryptor != nil {
		localStorage, err := r.clients.Encryptor.Decrypt(pbSessionData.LocalStorage)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt local storage for account %s: %w", accountID, err)
		}
		sessionData.LocalStorage = localStorage
	}

	return sessionData, nil
}
