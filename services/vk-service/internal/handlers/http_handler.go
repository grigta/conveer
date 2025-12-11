package handlers

import (
	"net/http"
	"strconv"

	"conveer/pkg/logger"
	"conveer/services/vk-service/internal/models"
	"conveer/services/vk-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type HTTPHandler struct {
	vkService service.VKService
	logger    logger.Logger
}

func NewHTTPHandler(vkService service.VKService, logger logger.Logger) *HTTPHandler {
	return &HTTPHandler{
		vkService: vkService,
		logger:    logger,
	}
}

func (h *HTTPHandler) RegisterRoutes(router *gin.Engine) {
	// Health check
	router.GET("/health", h.HealthCheck)

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes
	api := router.Group("/api/v1")
	{
		accounts := api.Group("/accounts")
		{
			accounts.POST("", h.CreateAccount)
			accounts.GET("/:id", h.GetAccount)
			accounts.GET("", h.ListAccounts)
			accounts.PUT("/:id/status", h.UpdateAccountStatus)
			accounts.POST("/:id/retry", h.RetryRegistration)
			accounts.DELETE("/:id", h.DeleteAccount)
		}

		api.GET("/statistics", h.GetStatistics)
	}
}

func (h *HTTPHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "vk-service",
	})
}

func (h *HTTPHandler) CreateAccount(c *gin.Context) {
	var request models.RegistrationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Error("Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	account, err := h.vkService.CreateAccount(c.Request.Context(), &request)
	if err != nil {
		h.logger.Error("Failed to create account", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create account",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Account creation initiated",
		"account": account,
	})
}

func (h *HTTPHandler) GetAccount(c *gin.Context) {
	idStr := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid account ID",
		})
		return
	}

	account, err := h.vkService.GetAccount(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get account", "error", err, "id", idStr)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Account not found",
		})
		return
	}

	c.JSON(http.StatusOK, account)
}

func (h *HTTPHandler) ListAccounts(c *gin.Context) {
	status := c.Query("status")
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.ParseInt(limitStr, 10, 64)

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	var accountStatus models.AccountStatus
	if status != "" {
		accountStatus = models.AccountStatus(status)
	}

	accounts, err := h.vkService.GetAccountsByStatus(c.Request.Context(), accountStatus, limit)
	if err != nil {
		h.logger.Error("Failed to list accounts", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list accounts",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accounts": accounts,
		"total":    len(accounts),
		"limit":    limit,
		"status":   status,
	})
}

func (h *HTTPHandler) UpdateAccountStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid account ID",
		})
		return
	}

	var request struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	status := models.AccountStatus(request.Status)
	if err := h.vkService.UpdateAccountStatus(c.Request.Context(), id, status); err != nil {
		h.logger.Error("Failed to update account status", "error", err, "id", idStr)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update account status",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Account status updated",
		"id":      idStr,
		"status":  status,
	})
}

func (h *HTTPHandler) RetryRegistration(c *gin.Context) {
	idStr := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid account ID",
		})
		return
	}

	if err := h.vkService.RetryRegistration(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to retry registration", "error", err, "id", idStr)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retry registration",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Registration retry queued",
		"id":      idStr,
	})
}

func (h *HTTPHandler) DeleteAccount(c *gin.Context) {
	idStr := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid account ID",
		})
		return
	}

	if err := h.vkService.DeleteAccount(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to delete account", "error", err, "id", idStr)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete account",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Account deleted successfully",
		"id":      idStr,
	})
}

func (h *HTTPHandler) GetStatistics(c *gin.Context) {
	stats, err := h.vkService.GetStatistics(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get statistics", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get statistics",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}