package handlers

import (
	"context"
	"encoding/json"
	"time"

	"conveer/pkg/logger"
	"conveer/services/warming-service/internal/models"
	"conveer/services/warming-service/internal/service"
	pb "conveer/services/warming-service/proto"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GRPCHandler struct {
	pb.UnimplementedWarmingServiceServer
	service service.WarmingService
	logger  logger.Logger
}

func NewGRPCHandler(service service.WarmingService, logger logger.Logger) *GRPCHandler {
	return &GRPCHandler{
		service: service,
		logger:  logger,
	}
}

func (h *GRPCHandler) StartWarming(ctx context.Context, req *pb.StartWarmingRequest) (*pb.WarmingTask, error) {
	// Validate request
	if req.AccountId == "" {
		return nil, status.Error(codes.InvalidArgument, "account_id is required")
	}
	if req.Platform == "" {
		return nil, status.Error(codes.InvalidArgument, "platform is required")
	}
	if req.DurationDays < 14 || req.DurationDays > 60 {
		return nil, status.Error(codes.InvalidArgument, "duration_days must be between 14 and 60")
	}

	accountID, err := primitive.ObjectIDFromHex(req.AccountId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid account_id format")
	}

	var scenarioID *primitive.ObjectID
	if req.ScenarioId != "" {
		sid, err := primitive.ObjectIDFromHex(req.ScenarioId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid scenario_id format")
		}
		scenarioID = &sid
	}

	// Start warming
	task, err := h.service.StartWarming(ctx, accountID, req.Platform, req.ScenarioType, scenarioID, int(req.DurationDays))
	if err != nil {
		h.logger.Error("Failed to start warming: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return h.taskToProto(task), nil
}

func (h *GRPCHandler) PauseWarming(ctx context.Context, req *pb.TaskRequest) (*pb.WarmingTask, error) {
	taskID, err := primitive.ObjectIDFromHex(req.TaskId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid task_id format")
	}

	task, err := h.service.PauseWarming(ctx, taskID)
	if err != nil {
		h.logger.Error("Failed to pause warming: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return h.taskToProto(task), nil
}

func (h *GRPCHandler) ResumeWarming(ctx context.Context, req *pb.TaskRequest) (*pb.WarmingTask, error) {
	taskID, err := primitive.ObjectIDFromHex(req.TaskId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid task_id format")
	}

	task, err := h.service.ResumeWarming(ctx, taskID)
	if err != nil {
		h.logger.Error("Failed to resume warming: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return h.taskToProto(task), nil
}

func (h *GRPCHandler) StopWarming(ctx context.Context, req *pb.TaskRequest) (*pb.WarmingTask, error) {
	taskID, err := primitive.ObjectIDFromHex(req.TaskId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid task_id format")
	}

	task, err := h.service.StopWarming(ctx, taskID)
	if err != nil {
		h.logger.Error("Failed to stop warming: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return h.taskToProto(task), nil
}

func (h *GRPCHandler) GetWarmingStatus(ctx context.Context, req *pb.TaskRequest) (*pb.WarmingTask, error) {
	taskID, err := primitive.ObjectIDFromHex(req.TaskId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid task_id format")
	}

	task, err := h.service.GetWarmingStatus(ctx, taskID)
	if err != nil {
		h.logger.Error("Failed to get warming status: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return h.taskToProto(task), nil
}

func (h *GRPCHandler) GetWarmingStatistics(ctx context.Context, req *pb.StatisticsRequest) (*pb.WarmingStatistics, error) {
	startDate := time.Now().AddDate(0, -1, 0) // Default: last month
	endDate := time.Now()

	if req.StartDate != nil {
		startDate = req.StartDate.AsTime()
	}
	if req.EndDate != nil {
		endDate = req.EndDate.AsTime()
	}

	stats, err := h.service.GetWarmingStatistics(ctx, req.Platform, startDate, endDate)
	if err != nil {
		h.logger.Error("Failed to get warming statistics: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return h.statsToProto(stats), nil
}

func (h *GRPCHandler) CreateCustomScenario(ctx context.Context, req *pb.CreateScenarioRequest) (*pb.WarmingScenario, error) {
	// Parse JSON actions and schedule
	var actions []models.ScenarioAction
	if err := json.Unmarshal([]byte(req.ActionsJson), &actions); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid actions_json format")
	}

	var schedule models.ScenarioSchedule
	if err := json.Unmarshal([]byte(req.ScheduleJson), &schedule); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid schedule_json format")
	}

	scenario := &models.WarmingScenario{
		Name:        req.Name,
		Description: req.Description,
		Platform:    req.Platform,
		Actions:     actions,
		Schedule:    schedule,
		CreatedBy:   req.CreatedBy,
	}

	createdScenario, err := h.service.CreateCustomScenario(ctx, scenario)
	if err != nil {
		h.logger.Error("Failed to create custom scenario: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return h.scenarioToProto(createdScenario), nil
}

func (h *GRPCHandler) UpdateCustomScenario(ctx context.Context, req *pb.UpdateScenarioRequest) (*pb.WarmingScenario, error) {
	scenarioID, err := primitive.ObjectIDFromHex(req.ScenarioId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid scenario_id format")
	}

	// Parse JSON if provided
	scenario := &models.WarmingScenario{
		Name:        req.Name,
		Description: req.Description,
	}

	if req.ActionsJson != "" {
		var actions []models.ScenarioAction
		if err := json.Unmarshal([]byte(req.ActionsJson), &actions); err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid actions_json format")
		}
		scenario.Actions = actions
	}

	if req.ScheduleJson != "" {
		var schedule models.ScenarioSchedule
		if err := json.Unmarshal([]byte(req.ScheduleJson), &schedule); err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid schedule_json format")
		}
		scenario.Schedule = schedule
	}

	updatedScenario, err := h.service.UpdateCustomScenario(ctx, scenarioID, scenario)
	if err != nil {
		h.logger.Error("Failed to update custom scenario: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return h.scenarioToProto(updatedScenario), nil
}

func (h *GRPCHandler) ListScenarios(ctx context.Context, req *pb.ListScenariosRequest) (*pb.ListScenariosResponse, error) {
	scenarios, err := h.service.ListScenarios(ctx, req.Platform)
	if err != nil {
		h.logger.Error("Failed to list scenarios: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var protoScenarios []*pb.WarmingScenario
	for _, scenario := range scenarios {
		protoScenarios = append(protoScenarios, h.scenarioToProto(scenario))
	}

	return &pb.ListScenariosResponse{
		Scenarios: protoScenarios,
	}, nil
}

func (h *GRPCHandler) ListTasks(ctx context.Context, req *pb.ListTasksRequest) (*pb.ListTasksResponse, error) {
	filter := models.TaskFilter{
		Platform: req.Platform,
		Status:   req.Status,
		Limit:    int(req.Limit),
		Offset:   int(req.Offset),
	}

	if req.AccountId != "" {
		accountID, err := primitive.ObjectIDFromHex(req.AccountId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid account_id format")
		}
		filter.AccountID = &accountID
	}

	tasks, err := h.service.ListTasks(ctx, filter)
	if err != nil {
		h.logger.Error("Failed to list tasks: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var protoTasks []*pb.WarmingTask
	for _, task := range tasks {
		protoTasks = append(protoTasks, h.taskToProto(task))
	}

	return &pb.ListTasksResponse{
		Tasks:      protoTasks,
		TotalCount: int64(len(tasks)),
	}, nil
}

// Helper functions
func (h *GRPCHandler) taskToProto(task *models.WarmingTask) *pb.WarmingTask {
	protoTask := &pb.WarmingTask{
		Id:               task.ID.Hex(),
		AccountId:        task.AccountID.Hex(),
		Platform:         task.Platform,
		ScenarioType:     task.ScenarioType,
		ScenarioId:       task.ScenarioID.Hex(),
		DurationDays:     int32(task.DurationDays),
		Status:           task.Status,
		CurrentDay:       int32(task.CurrentDay),
		ActionsCompleted: int32(task.ActionsCompleted),
		ActionsFailed:    int32(task.ActionsFailed),
		LastError:        task.LastError,
		CreatedAt:        timestamppb.New(task.CreatedAt),
		UpdatedAt:        timestamppb.New(task.UpdatedAt),
	}

	if task.NextActionAt != nil {
		protoTask.NextActionAt = timestamppb.New(*task.NextActionAt)
	}

	if task.CompletedAt != nil {
		protoTask.CompletedAt = timestamppb.New(*task.CompletedAt)
	}

	return protoTask
}

func (h *GRPCHandler) scenarioToProto(scenario *models.WarmingScenario) *pb.WarmingScenario {
	return &pb.WarmingScenario{
		Id:          scenario.ID.Hex(),
		Name:        scenario.Name,
		Description: scenario.Description,
		Platform:    scenario.Platform,
		IsActive:    scenario.IsActive,
		CreatedAt:   timestamppb.New(scenario.CreatedAt),
		UpdatedAt:   timestamppb.New(scenario.UpdatedAt),
	}
}

func (h *GRPCHandler) statsToProto(stats *models.AggregatedStats) *pb.WarmingStatistics {
	protoStats := &pb.WarmingStatistics{
		TotalTasks:      stats.TotalTasks,
		CompletedTasks:  stats.CompletedTasks,
		FailedTasks:     stats.FailedTasks,
		InProgressTasks: stats.InProgressTasks,
		SuccessRate:     stats.SuccessRate,
		AvgDurationDays: stats.AvgDurationDays,
		ByPlatform:      stats.ByPlatform,
		ByScenario:      stats.ByScenario,
	}

	// Convert top actions
	for _, action := range stats.TopActions {
		protoStats.TopActions = append(protoStats.TopActions, &pb.ActionStatistic{
			ActionType:    action.ActionType,
			Count:         action.Count,
			SuccessRate:   action.SuccessRate,
			AvgDurationMs: action.AvgDuration,
		})
	}

	// Convert common errors
	for _, err := range stats.CommonErrors {
		protoStats.CommonErrors = append(protoStats.CommonErrors, &pb.ErrorStatistic{
			ErrorType:  err.ErrorType,
			Count:      err.Count,
			Percentage: err.Percentage,
		})
	}

	// Convert daily breakdown
	for _, daily := range stats.DailyBreakdown {
		protoStats.DailyBreakdown = append(protoStats.DailyBreakdown, &pb.DailyStatistic{
			Date:            timestamppb.New(daily.Date),
			TasksStarted:    daily.TasksStarted,
			TasksCompleted:  daily.TasksCompleted,
			TasksFailed:     daily.TasksFailed,
			ActionsExecuted: daily.ActionsExecuted,
			SuccessRate:     daily.SuccessRate,
		})
	}

	return protoStats
}