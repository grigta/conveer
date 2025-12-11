package service

import (
	"context"
	"fmt"
	"time"

	"github.com/conveer/conveer/services/telegram-bot/internal/config"
	vkpb "github.com/conveer/conveer/services/vk-service/proto"
	telegrampb "github.com/conveer/conveer/services/telegram-service/proto"
	analyticspb "github.com/conveer/conveer/services/analytics-service/proto"
	"github.com/conveer/conveer/pkg/crypto"
	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type GRPCClients struct {
	VKClient       *grpc.ClientConn
	TelegramClient *grpc.ClientConn
	MailClient     *grpc.ClientConn
	MaxClient      *grpc.ClientConn
	WarmingClient  *grpc.ClientConn
	ProxyClient    *grpc.ClientConn
	SMSClient      *grpc.ClientConn
	AnalyticsClient *grpc.ClientConn

	// Protobuf clients
	VKServiceClient       vkpb.VKServiceClient
	TelegramServiceClient telegrampb.TelegramServiceClient
	AnalyticsServiceClient analyticspb.AnalyticsServiceClient

	// Encryption
	Encryptor *crypto.Encryptor

	// Redis for caching
	RedisClient *redis.Client
}

func InitializeGRPCClients(cfg *config.Config) (*GRPCClients, error) {
	clients := &GRPCClients{}

	// Initialize encryption
	encryptor, err := crypto.NewEncryptor(cfg.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize encryptor: %w", err)
	}
	clients.Encryptor = encryptor

	// Initialize Redis client
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}
	clients.RedisClient = redis.NewClient(opt)

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := clients.RedisClient.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Helper function to create a connection with timeout
	createConn := func(service, url string) (*grpc.ClientConn, error) {
		if url == "" {
			return nil, nil // Service not configured
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, err := grpc.DialContext(ctx, url,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to %s service at %s: %w", service, url, err)
		}
		return conn, nil
	}

	// Initialize VK client
	if clients.VKClient, err = createConn("vk", cfg.GRPCServices["vk"]); err != nil {
		return nil, err
	}
	if clients.VKClient != nil {
		clients.VKServiceClient = vkpb.NewVKServiceClient(clients.VKClient)
	}

	// Initialize Telegram client
	if clients.TelegramClient, err = createConn("telegram", cfg.GRPCServices["telegram"]); err != nil {
		return nil, err
	}
	if clients.TelegramClient != nil {
		clients.TelegramServiceClient = telegrampb.NewTelegramServiceClient(clients.TelegramClient)
	}

	// Initialize Mail client
	if clients.MailClient, err = createConn("mail", cfg.GRPCServices["mail"]); err != nil {
		return nil, err
	}

	// Initialize Max client
	if clients.MaxClient, err = createConn("max", cfg.GRPCServices["max"]); err != nil {
		return nil, err
	}

	// Initialize Warming client
	if clients.WarmingClient, err = createConn("warming", cfg.GRPCServices["warming"]); err != nil {
		return nil, err
	}

	// Initialize Proxy client
	if clients.ProxyClient, err = createConn("proxy", cfg.GRPCServices["proxy"]); err != nil {
		return nil, err
	}

	// Initialize SMS client
	if clients.SMSClient, err = createConn("sms", cfg.GRPCServices["sms"]); err != nil {
		return nil, err
	}

	// Initialize Analytics client
	if clients.AnalyticsClient, err = createConn("analytics", cfg.GRPCServices["analytics"]); err != nil {
		return nil, err
	}
	if clients.AnalyticsClient != nil {
		clients.AnalyticsServiceClient = analyticspb.NewAnalyticsServiceClient(clients.AnalyticsClient)
	}

	return clients, nil
}

func (c *GRPCClients) Close() {
	if c.VKClient != nil {
		c.VKClient.Close()
	}
	if c.TelegramClient != nil {
		c.TelegramClient.Close()
	}
	if c.MailClient != nil {
		c.MailClient.Close()
	}
	if c.MaxClient != nil {
		c.MaxClient.Close()
	}
	if c.WarmingClient != nil {
		c.WarmingClient.Close()
	}
	if c.ProxyClient != nil {
		c.ProxyClient.Close()
	}
	if c.SMSClient != nil {
		c.SMSClient.Close()
	}
	if c.AnalyticsClient != nil {
		c.AnalyticsClient.Close()
	}
	if c.RedisClient != nil {
		c.RedisClient.Close()
	}
}

// GetClientByPlatform returns the appropriate gRPC client for a platform
func (c *GRPCClients) GetClientByPlatform(platform string) (*grpc.ClientConn, error) {
	switch platform {
	case "vk":
		if c.VKClient == nil {
			return nil, fmt.Errorf("VK service not configured")
		}
		return c.VKClient, nil
	case "telegram":
		if c.TelegramClient == nil {
			return nil, fmt.Errorf("Telegram service not configured")
		}
		return c.TelegramClient, nil
	case "mail":
		if c.MailClient == nil {
			return nil, fmt.Errorf("Mail service not configured")
		}
		return c.MailClient, nil
	case "max":
		if c.MaxClient == nil {
			return nil, fmt.Errorf("Max service not configured")
		}
		return c.MaxClient, nil
	default:
		return nil, fmt.Errorf("unknown platform: %s", platform)
	}
}
