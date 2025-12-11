package handlers

import (
	"net/http"
	"strconv"

	"conveer/pkg/logger"
	"conveer/services/telegram-service/internal/models"
	"conveer/services/telegram-service/internal/service"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type HTTPHandler struct {
	service service.TelegramService
	logger  logger.Logger
}

func NewHTTPHandler(service service.TelegramService, logger logger.Logger) *HTTPHandler {
	return &HTTPHandler{
		service: service,
		logger:  logger,
	}
}

func (h *HTTPHandler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	{
		accounts := api.Group("/accounts")
		{
			accounts.POST("", h.CreateAccount)
			accounts.GET("", h.ListAccounts)
			accounts.GET("/:id", h.GetAccount)
			accounts.PUT("/:id/status", h.UpdateAccountStatus)
			accounts.POST("/:id/retry", h.RetryRegistration)
			accounts.DELETE("/:id", h.DeleteAccount)
		}

		api.GET("/statistics", h.GetStatistics)
	}
}

func (h *HTTPHandler) CreateAccount(c *gin.Context) {
	var req models.RegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	account, err := h.service.CreateAccount(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to create account", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, account)
}

func (h *HTTPHandler) GetAccount(c *gin.Context) {
	idParam := c.Param("id")
	accountID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid account ID",
		})
		return
	}

	account, err := h.service.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		h.logger.Error("Failed to get account", "id", idParam, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Account not found",
		})
		return
	}

	c.JSON(http.StatusOK, account)
}

func (h *HTTPHandler) ListAccounts(c *gin.Context) {
	// Parse query parameters
	status := c.Query("status")
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	var accountStatus models.AccountStatus
	if status != "" {
		accountStatus = models.AccountStatus(status)
	}

	accounts, total, err := h.service.ListAccounts(c.Request.Context(), accountStatus, limit, offset)
	if err != nil {
		h.logger.Error("Failed to list accounts", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list accounts",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accounts": accounts,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *HTTPHandler) UpdateAccountStatus(c *gin.Context) {
	idParam := c.Param("id")
	accountID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid account ID",
		})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	account, err := h.service.UpdateAccountStatus(c.Request.Context(), accountID, models.AccountStatus(req.Status))
	if err != nil {
		h.logger.Error("Failed to update account status", "id", idParam, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, account)
}

func (h *HTTPHandler) RetryRegistration(c *gin.Context) {
	idParam := c.Param("id")
	accountID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid account ID",
		})
		return
	}

	account, err := h.service.RetryRegistration(c.Request.Context(), accountID)
	if err != nil {
		h.logger.Error("Failed to retry registration", "id", idParam, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, account)
}

func (h *HTTPHandler) DeleteAccount(c *gin.Context) {
	idParam := c.Param("id")
	accountID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid account ID",
		})
		return
	}

	if err := h.service.DeleteAccount(c.Request.Context(), accountID); err != nil {
		h.logger.Error("Failed to delete account", "id", idParam, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *HTTPHandler) GetStatistics(c *gin.Context) {
	stats, err := h.service.GetStatistics(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get statistics", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get statistics",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}