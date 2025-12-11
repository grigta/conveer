package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/grigta/conveer/pkg/logger"
	"github.com/grigta/conveer/services/warming-service/internal/models"
	"github.com/grigta/conveer/services/warming-service/internal/service"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type HTTPHandler struct {
	service service.WarmingService
	logger  logger.Logger
}

func NewHTTPHandler(service service.WarmingService, logger logger.Logger) *HTTPHandler {
	return &HTTPHandler{
		service: service,
		logger:  logger,
	}
}

func (h *HTTPHandler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1/warming")
	{
		api.POST("/start", h.StartWarming)
		api.POST("/:taskId/pause", h.PauseWarming)
		api.POST("/:taskId/resume", h.ResumeWarming)
		api.POST("/:taskId/stop", h.StopWarming)
		api.GET("/:taskId", h.GetWarmingStatus)
		api.GET("/statistics", h.GetWarmingStatistics)
		api.POST("/scenarios", h.CreateCustomScenario)
		api.PUT("/scenarios/:scenarioId", h.UpdateCustomScenario)
		api.GET("/scenarios", h.ListScenarios)
		api.GET("/tasks", h.ListTasks)
	}
}

func (h *HTTPHandler) StartWarming(c *gin.Context) {
	var req struct {
		AccountID    string `json:"account_id" binding:"required"`
		Platform     string `json:"platform" binding:"required"`
		ScenarioType string `json:"scenario_type" binding:"required"`
		ScenarioID   string `json:"scenario_id"`
		DurationDays int    `json:"duration_days" binding:"required,min=14,max=60"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	accountID, err := primitive.ObjectIDFromHex(req.AccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account_id format"})
		return
	}

	var scenarioID *primitive.ObjectID
	if req.ScenarioID != "" {
		sid, err := primitive.ObjectIDFromHex(req.ScenarioID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scenario_id format"})
			return
		}
		scenarioID = &sid
	}

	task, err := h.service.StartWarming(c.Request.Context(), accountID, req.Platform, req.ScenarioType, scenarioID, req.DurationDays)
	if err != nil {
		h.logger.Error("Failed to start warming: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *HTTPHandler) PauseWarming(c *gin.Context) {
	taskID, err := primitive.ObjectIDFromHex(c.Param("taskId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task_id format"})
		return
	}

	task, err := h.service.PauseWarming(c.Request.Context(), taskID)
	if err != nil {
		h.logger.Error("Failed to pause warming: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *HTTPHandler) ResumeWarming(c *gin.Context) {
	taskID, err := primitive.ObjectIDFromHex(c.Param("taskId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task_id format"})
		return
	}

	task, err := h.service.ResumeWarming(c.Request.Context(), taskID)
	if err != nil {
		h.logger.Error("Failed to resume warming: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *HTTPHandler) StopWarming(c *gin.Context) {
	taskID, err := primitive.ObjectIDFromHex(c.Param("taskId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task_id format"})
		return
	}

	task, err := h.service.StopWarming(c.Request.Context(), taskID)
	if err != nil {
		h.logger.Error("Failed to stop warming: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *HTTPHandler) GetWarmingStatus(c *gin.Context) {
	taskID, err := primitive.ObjectIDFromHex(c.Param("taskId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task_id format"})
		return
	}

	task, err := h.service.GetWarmingStatus(c.Request.Context(), taskID)
	if err != nil {
		h.logger.Error("Failed to get warming status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *HTTPHandler) GetWarmingStatistics(c *gin.Context) {
	platform := c.Query("platform")

	// Parse dates
	startDate := time.Now().AddDate(0, -1, 0) // Default: last month
	endDate := time.Now()

	if s := c.Query("start_date"); s != "" {
		if parsed, err := time.Parse("2006-01-02", s); err == nil {
			startDate = parsed
		}
	}

	if e := c.Query("end_date"); e != "" {
		if parsed, err := time.Parse("2006-01-02", e); err == nil {
			endDate = parsed
		}
	}

	stats, err := h.service.GetWarmingStatistics(c.Request.Context(), platform, startDate, endDate)
	if err != nil {
		h.logger.Error("Failed to get warming statistics: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *HTTPHandler) CreateCustomScenario(c *gin.Context) {
	var scenario models.WarmingScenario
	if err := c.ShouldBindJSON(&scenario); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createdScenario, err := h.service.CreateCustomScenario(c.Request.Context(), &scenario)
	if err != nil {
		h.logger.Error("Failed to create custom scenario: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, createdScenario)
}

func (h *HTTPHandler) UpdateCustomScenario(c *gin.Context) {
	scenarioID, err := primitive.ObjectIDFromHex(c.Param("scenarioId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scenario_id format"})
		return
	}

	var scenario models.WarmingScenario
	if err := c.ShouldBindJSON(&scenario); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedScenario, err := h.service.UpdateCustomScenario(c.Request.Context(), scenarioID, &scenario)
	if err != nil {
		h.logger.Error("Failed to update custom scenario: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updatedScenario)
}

func (h *HTTPHandler) ListScenarios(c *gin.Context) {
	platform := c.Query("platform")

	scenarios, err := h.service.ListScenarios(c.Request.Context(), platform)
	if err != nil {
		h.logger.Error("Failed to list scenarios: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"scenarios": scenarios,
		"total":     len(scenarios),
	})
}

func (h *HTTPHandler) ListTasks(c *gin.Context) {
	filter := models.TaskFilter{
		Platform: c.Query("platform"),
		Status:   c.Query("status"),
	}

	// Parse limit and offset
	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}

	// Parse account_id if provided
	if accountID := c.Query("account_id"); accountID != "" {
		if aid, err := primitive.ObjectIDFromHex(accountID); err == nil {
			filter.AccountID = &aid
		}
	}

	tasks, err := h.service.ListTasks(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list tasks: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"total": len(tasks),
	})
}
