package handlers

import (
	"net/http"
	"time"

	"github.com/grigta/conveer/pkg/config"
	"github.com/grigta/conveer/pkg/logger"
	"github.com/grigta/conveer/services/api-gateway/internal/proxy"
	"github.com/gin-gonic/gin"
)

type Handlers struct {
	config      *config.Config
	proxyClient *proxy.ProxyClient
}

func NewHandlers(cfg *config.Config) *Handlers {
	return &Handlers{
		config:      cfg,
		proxyClient: proxy.NewProxyClient(cfg),
	}
}

func (h *Handlers) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "api-gateway",
	})
}

func (h *Handlers) AuthProxy(c *gin.Context) {
	logger.Info("Proxying request to auth service",
		logger.Field{Key: "method", Value: c.Request.Method},
		logger.Field{Key: "path", Value: c.Request.URL.Path},
	)

	h.proxyClient.ProxyToService(c, h.config.Services.AuthServiceURL, c.Request.URL.Path)
}

func (h *Handlers) UserProxy(c *gin.Context) {
	logger.Info("Proxying request to user service",
		logger.Field{Key: "method", Value: c.Request.Method},
		logger.Field{Key: "path", Value: c.Request.URL.Path},
	)

	h.proxyClient.ProxyToService(c, h.config.Services.UserServiceURL, c.Request.URL.Path)
}

func (h *Handlers) ProductProxy(c *gin.Context) {
	logger.Info("Proxying request to product service",
		logger.Field{Key: "method", Value: c.Request.Method},
		logger.Field{Key: "path", Value: c.Request.URL.Path},
	)

	h.proxyClient.ProxyToService(c, h.config.Services.ProductServiceURL, c.Request.URL.Path)
}

func (h *Handlers) OrderProxy(c *gin.Context) {
	logger.Info("Proxying request to order service",
		logger.Field{Key: "method", Value: c.Request.Method},
		logger.Field{Key: "path", Value: c.Request.URL.Path},
	)

	h.proxyClient.ProxyToService(c, h.config.Services.OrderServiceURL, c.Request.URL.Path)
}

func (h *Handlers) NotificationProxy(c *gin.Context) {
	logger.Info("Proxying request to notification service",
		logger.Field{Key: "method", Value: c.Request.Method},
		logger.Field{Key: "path", Value: c.Request.URL.Path},
	)

	h.proxyClient.ProxyToService(c, h.config.Services.NotificationServiceURL, c.Request.URL.Path)
}

func (h *Handlers) AnalyticsProxy(c *gin.Context) {
	logger.Info("Proxying request to analytics service",
		logger.Field{Key: "method", Value: c.Request.Method},
		logger.Field{Key: "path", Value: c.Request.URL.Path},
	)

	h.proxyClient.ProxyToService(c, h.config.Services.AnalyticsServiceURL, c.Request.URL.Path)
}

func (h *Handlers) ProxyProxy(c *gin.Context) {
	logger.Info("Proxying request to proxy service",
		logger.Field{Key: "method", Value: c.Request.Method},
		logger.Field{Key: "path", Value: c.Request.URL.Path},
	)

	h.proxyClient.ProxyToService(c, h.config.Services.ProxyServiceURL, c.Request.URL.Path)
}

func (h *Handlers) SMSProxy(c *gin.Context) {
	logger.Info("Proxying request to SMS service",
		logger.Field{Key: "method", Value: c.Request.Method},
		logger.Field{Key: "path", Value: c.Request.URL.Path},
	)

	h.proxyClient.ProxyToService(c, h.config.Services.SMSServiceURL, c.Request.URL.Path)
}

func (h *Handlers) VKProxy(c *gin.Context) {
	logger.Info("Proxying request to VK service",
		logger.Field{Key: "method", Value: c.Request.Method},
		logger.Field{Key: "path", Value: c.Request.URL.Path},
	)

	h.proxyClient.ProxyToService(c, h.config.Services.VKServiceURL, c.Request.URL.Path)
}

func (h *Handlers) TelegramProxy(c *gin.Context) {
	logger.Info("Proxying request to Telegram service",
		logger.Field{Key: "method", Value: c.Request.Method},
		logger.Field{Key: "path", Value: c.Request.URL.Path},
	)

	h.proxyClient.ProxyToService(c, h.config.Services.TelegramServiceURL, c.Request.URL.Path)
}

func (h *Handlers) MailProxy(c *gin.Context) {
	logger.Info("Proxying request to Mail service",
		logger.Field{Key: "method", Value: c.Request.Method},
		logger.Field{Key: "path", Value: c.Request.URL.Path},
	)

	h.proxyClient.ProxyToService(c, h.config.Services.MailServiceURL, c.Request.URL.Path)
}

func (h *Handlers) MaxProxy(c *gin.Context) {
	logger.Info("Proxying request to Max service",
		logger.Field{Key: "method", Value: c.Request.Method},
		logger.Field{Key: "path", Value: c.Request.URL.Path},
	)

	h.proxyClient.ProxyToService(c, h.config.Services.MaxServiceURL, c.Request.URL.Path)
}

func (h *Handlers) WarmingProxy(c *gin.Context) {
	logger.Info("Proxying request to Warming service",
		logger.Field{Key: "method", Value: c.Request.Method},
		logger.Field{Key: "path", Value: c.Request.URL.Path},
	)

	h.proxyClient.ProxyToService(c, h.config.Services.WarmingServiceURL, c.Request.URL.Path)
}

func (h *Handlers) NotFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{
		"error":   "Route not found",
		"path":    c.Request.URL.Path,
		"method":  c.Request.Method,
	})
}

func (h *Handlers) MethodNotAllowed(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{
		"error":   "Method not allowed",
		"path":    c.Request.URL.Path,
		"method":  c.Request.Method,
	})
}
