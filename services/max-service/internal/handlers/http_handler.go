package handlers

import (
	"net/http"
	"strconv"

	"github.com/grigta/conveer/services/max-service/internal/models"
	"github.com/grigta/conveer/services/max-service/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPHandler handles HTTP requests
type HTTPHandler struct {
	service *service.MaxService
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler(service *service.MaxService) *HTTPHandler {
	return &HTTPHandler{
		service: service,
	}
}

// RegisterRoutes registers HTTP routes
func (h *HTTPHandler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api")
	{
		api.POST("/accounts", h.CreateAccount)
		api.GET("/accounts", h.ListAccounts)
		api.GET("/accounts/:id", h.GetAccount)
		api.PUT("/accounts/:id/status", h.UpdateAccountStatus)
		api.POST("/accounts/:id/retry", h.RetryRegistration)
		api.POST("/accounts/:id/link-vk", h.LinkVKAccount)
		api.DELETE("/accounts/:id", h.DeleteAccount)
		api.GET("/statistics", h.GetStatistics)
	}
	
	// Health check
	router.GET("/health", h.HealthCheck)
	
	// Metrics
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// CreateAccount creates a new account
func (h *HTTPHandler) CreateAccount(c *gin.Context) {
	var req models.RegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	result, err := h.service.CreateAccount(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, result)
}

// GetAccount retrieves an account
func (h *HTTPHandler) GetAccount(c *gin.Context) {
	id := c.Param("id")
	
	account, err := h.service.GetAccount(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	
	c.JSON(http.StatusOK, account)
}

// ListAccounts lists accounts
func (h *HTTPHandler) ListAccounts(c *gin.Context) {
	// Parse query parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	status := c.Query("status")
	
	filter := make(map[string]interface{})
	if status != "" {
		filter["status"] = status
	}
	filter["deleted_at"] = nil
	
	accounts, total, err := h.service.ListAccounts(c.Request.Context(), filter, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"accounts": accounts,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// UpdateAccountStatus updates account status
func (h *HTTPHandler) UpdateAccountStatus(c *gin.Context) {
	id := c.Param("id")
	
	var req struct {
		Status       string `json:"status" binding:"required"`
		ErrorMessage string `json:"error_message,omitempty"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	err := h.service.UpdateAccountStatus(
		c.Request.Context(),
		id,
		models.AccountStatus(req.Status),
		req.ErrorMessage,
	)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RetryRegistration retries a failed registration
func (h *HTTPHandler) RetryRegistration(c *gin.Context) {
	id := c.Param("id")
	
	err := h.service.RetryRegistration(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// LinkVKAccount links a VK account to Max account
func (h *HTTPHandler) LinkVKAccount(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		VKAccountID string `json:"vk_account_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.LinkVKAccount(c.Request.Context(), id, req.VKAccountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteAccount deletes an account
func (h *HTTPHandler) DeleteAccount(c *gin.Context) {
	id := c.Param("id")

	err := h.service.DeleteAccount(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// GetStatistics returns statistics
func (h *HTTPHandler) GetStatistics(c *gin.Context) {
	stats, err := h.service.GetStatistics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, stats)
}

// HealthCheck returns service health
func (h *HTTPHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "max-service",
	})
}
