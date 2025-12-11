package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/grigta/conveer/services/sms-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type HTTPHandler struct {
	smsService *service.SMSService
	logger     *logrus.Logger
}

func NewHTTPHandler(smsService *service.SMSService, logger *logrus.Logger) *HTTPHandler {
	return &HTTPHandler{
		smsService: smsService,
		logger:     logger,
	}
}

func (h *HTTPHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"time":   time.Now().Unix(),
	})
}

func (h *HTTPHandler) PurchaseNumber(c *gin.Context) {
	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		Service  string `json:"service" binding:"required"`
		Country  string `json:"country" binding:"required"`
		Operator string `json:"operator"`
		Provider string `json:"provider"`
		MaxPrice int32  `json:"max_price"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	activation, err := h.smsService.PurchaseNumber(
		c.Request.Context(),
		req.UserID,
		req.Service,
		req.Country,
		req.Operator,
		req.Provider,
		req.MaxPrice,
	)

	if err != nil {
		h.logger.Errorf("Failed to purchase number: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"activation_id": activation.ActivationID,
		"phone_number":  activation.PhoneNumber,
		"price":         activation.Price,
		"provider":      activation.Provider,
		"expires_at":    activation.ExpiresAt.Unix(),
	})
}

func (h *HTTPHandler) GetSMSCode(c *gin.Context) {
	activationID := c.Param("activation_id")
	userID := c.Query("user_id")

	if activationID == "" || userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "activation_id and user_id are required"})
		return
	}

	code, fullSMS, err := h.smsService.GetSMSCode(c.Request.Context(), activationID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":     code,
		"full_sms": fullSMS,
	})
}

func (h *HTTPHandler) CancelActivation(c *gin.Context) {
	activationID := c.Param("activation_id")

	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Reason string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	refunded, refundAmount, err := h.smsService.CancelActivation(
		c.Request.Context(),
		activationID,
		req.UserID,
		req.Reason,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"refunded":      refunded,
		"refund_amount": refundAmount,
	})
}

func (h *HTTPHandler) GetActivationStatus(c *gin.Context) {
	activationID := c.Param("activation_id")
	userID := c.Query("user_id")

	if activationID == "" || userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "activation_id and user_id are required"})
		return
	}

	activation, err := h.smsService.GetActivationStatus(c.Request.Context(), activationID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := gin.H{
		"activation_id": activation.ActivationID,
		"status":        activation.Status,
		"phone_number":  activation.PhoneNumber,
		"service":       activation.Service,
		"created_at":    activation.CreatedAt.Unix(),
		"expires_at":    activation.ExpiresAt.Unix(),
	}

	if activation.Code != "" {
		response["code"] = activation.Code
	}

	if activation.CompletedAt != nil {
		response["completed_at"] = activation.CompletedAt.Unix()
	}

	c.JSON(http.StatusOK, response)
}

func (h *HTTPHandler) GetStatistics(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	var fromDate, toDate time.Time

	if from := c.Query("from_date"); from != "" {
		if ts, err := strconv.ParseInt(from, 10, 64); err == nil {
			fromDate = time.Unix(ts, 0)
		}
	}

	if to := c.Query("to_date"); to != "" {
		if ts, err := strconv.ParseInt(to, 10, 64); err == nil {
			toDate = time.Unix(ts, 0)
		}
	}

	service := c.Query("service")
	country := c.Query("country")

	stats, err := h.smsService.GetStatistics(
		c.Request.Context(),
		userID,
		fromDate,
		toDate,
		service,
		country,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *HTTPHandler) GetProviderBalance(c *gin.Context) {
	provider := c.Query("provider")
	if provider == "" {
		provider = "smsactivate" // Default provider
	}

	balance, currency, err := h.smsService.GetProviderBalance(c.Request.Context(), provider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"provider":    provider,
		"balance":     balance,
		"currency":    currency,
		"updated_at":  time.Now().Unix(),
	})
}
