package handlers

import (
	"net/http"

	"conveer/pkg/middleware"
	"conveer/services/proxy-service/internal/models"
	"conveer/services/proxy-service/internal/repository"
	"conveer/services/proxy-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type HTTPHandler struct {
	proxyService  *service.ProxyService
	proxyRepo     *repository.ProxyRepository
	providerRepo  *repository.ProviderRepository
	logger        *logrus.Logger
}

func NewHTTPHandler(
	proxyService *service.ProxyService,
	proxyRepo *repository.ProxyRepository,
	providerRepo *repository.ProviderRepository,
	logger *logrus.Logger,
) *HTTPHandler {
	return &HTTPHandler{
		proxyService:  proxyService,
		proxyRepo:     proxyRepo,
		providerRepo:  providerRepo,
		logger:        logger,
	}
}

func (h *HTTPHandler) SetupRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	api.Use(middleware.AuthMiddleware())

	proxies := api.Group("/proxies")
	{
		proxies.POST("/allocate", h.AllocateProxy)
		proxies.POST("/release", h.ReleaseProxy)
		proxies.GET("/:id", h.GetProxyByID)
		proxies.GET("/account/:account_id", h.GetProxyByAccount)
		proxies.GET("/health/:id", h.GetProxyHealth)
		proxies.POST("/:id/rotate", h.RotateProxy)
		proxies.GET("/statistics", h.GetStatistics)
	}

	api.GET("/providers", h.GetProviders)

	router.GET("/health", h.HealthCheck)
}

func (h *HTTPHandler) AllocateProxy(c *gin.Context) {
	var request models.ProxyAllocationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.WithError(err).Error("Failed to bind request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	proxy, err := h.proxyService.AllocateProxy(c.Request.Context(), request)
	if err != nil {
		h.logger.WithError(err).Error("Failed to allocate proxy")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        proxy.ID.Hex(),
		"ip":        proxy.IP,
		"port":      proxy.Port,
		"username":  proxy.Username,
		"password":  proxy.Password,
		"protocol":  proxy.Protocol,
		"type":      proxy.Type,
		"country":   proxy.Country,
		"city":      proxy.City,
		"status":    proxy.Status,
		"expiresAt": proxy.ExpiresAt,
	})
}

func (h *HTTPHandler) ReleaseProxy(c *gin.Context) {
	var request struct {
		AccountID string `json:"account_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.WithError(err).Error("Failed to bind request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.proxyService.ReleaseProxy(c.Request.Context(), request.AccountID); err != nil {
		h.logger.WithError(err).Error("Failed to release proxy")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Proxy released successfully"})
}

func (h *HTTPHandler) GetProxyByID(c *gin.Context) {
	idStr := c.Param("id")

	proxyID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		h.logger.WithError(err).Error("Invalid proxy ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy ID"})
		return
	}

	proxy, err := h.proxyRepo.GetProxyByID(c.Request.Context(), proxyID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get proxy")
		c.JSON(http.StatusNotFound, gin.H{"error": "Proxy not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        proxy.ID.Hex(),
		"ip":        proxy.IP,
		"port":      proxy.Port,
		"username":  proxy.Username,
		"password":  proxy.Password,
		"protocol":  proxy.Protocol,
		"type":      proxy.Type,
		"country":   proxy.Country,
		"city":      proxy.City,
		"status":    proxy.Status,
		"expiresAt": proxy.ExpiresAt,
	})
}

func (h *HTTPHandler) GetProxyByAccount(c *gin.Context) {
	accountID := c.Param("account_id")

	proxy, err := h.proxyService.GetProxyForAccount(c.Request.Context(), accountID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get proxy for account")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if proxy == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No proxy found for account"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        proxy.ID.Hex(),
		"ip":        proxy.IP,
		"port":      proxy.Port,
		"username":  proxy.Username,
		"password":  proxy.Password,
		"protocol":  proxy.Protocol,
		"type":      proxy.Type,
		"country":   proxy.Country,
		"city":      proxy.City,
		"status":    proxy.Status,
		"expiresAt": proxy.ExpiresAt,
	})
}

func (h *HTTPHandler) GetProxyHealth(c *gin.Context) {
	idStr := c.Param("id")

	proxyID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		h.logger.WithError(err).Error("Invalid proxy ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy ID"})
		return
	}

	health, err := h.proxyRepo.GetProxyHealthByID(c.Request.Context(), proxyID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get proxy health")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get proxy health"})
		return
	}

	if health == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Proxy health data not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"proxy_id":         health.ProxyID.Hex(),
		"latency":          health.Latency,
		"fraud_score":      health.FraudScore,
		"is_vpn":           health.IsVPN,
		"is_proxy":         health.IsProxy,
		"is_tor":           health.IsTor,
		"blacklist_status": health.BlacklistStatus,
		"last_check":       health.LastCheck.Unix(),
		"failed_checks":    health.FailedChecks,
	})
}

func (h *HTTPHandler) RotateProxy(c *gin.Context) {
	_ = c.Param("id")

	var request struct {
		AccountID string `json:"account_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.WithError(err).Error("Failed to bind request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newProxy, err := h.proxyService.ForceRotateProxy(c.Request.Context(), request.AccountID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to rotate proxy")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        newProxy.ID.Hex(),
		"ip":        newProxy.IP,
		"port":      newProxy.Port,
		"username":  newProxy.Username,
		"password":  newProxy.Password,
		"protocol":  newProxy.Protocol,
		"type":      newProxy.Type,
		"country":   newProxy.Country,
		"city":      newProxy.City,
		"status":    newProxy.Status,
		"expiresAt": newProxy.ExpiresAt,
	})
}

func (h *HTTPHandler) GetStatistics(c *gin.Context) {
	stats, err := h.proxyService.GetProxyStatistics(c.Request.Context())
	if err != nil {
		h.logger.WithError(err).Error("Failed to get statistics")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *HTTPHandler) GetProviders(c *gin.Context) {
	stats, err := h.providerRepo.GetAllProviderStats(c.Request.Context())
	if err != nil {
		h.logger.WithError(err).Error("Failed to get provider stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get provider stats"})
		return
	}

	providers := make([]gin.H, 0, len(stats))
	for _, stat := range stats {
		providers = append(providers, gin.H{
			"name":              stat.ProviderName,
			"total_allocated":   stat.TotalAllocated,
			"total_released":    stat.TotalReleased,
			"total_rotated":     stat.TotalRotated,
			"total_failed":      stat.TotalFailed,
			"active_proxies":    stat.ActiveProxies,
			"last_request_time": stat.LastRequestTime,
			"last_success_time": stat.LastSuccessTime,
			"last_error_time":   stat.LastErrorTime,
			"last_error":        stat.LastError,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"providers": providers,
	})
}

func (h *HTTPHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "proxy-service",
	})
}