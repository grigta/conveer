package handlers

import (
	"context"
	"time"

	"conveer/pkg/logger"
	"conveer/services/vk-service/internal/models"
	"conveer/services/vk-service/internal/service"
	pb "conveer/services/vk-service/proto"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GRPCHandler struct {
	pb.UnimplementedVKServiceServer
	vkService service.VKService
	logger    logger.Logger
}

func NewGRPCHandler(vkService service.VKService, logger logger.Logger) *GRPCHandler {
	return &GRPCHandler{
		vkService: vkService,
		logger:    logger,
	}
}

func (h *GRPCHandler) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.Account, error) {
	registrationReq := &models.RegistrationRequest{
		FirstName:        req.FirstName,
		LastName:         req.LastName,
		PreferredCountry: req.PreferredCountry,
		UseRandomProfile: req.UseRandomProfile,
	}

	if req.BirthDate != nil {
		registrationReq.BirthDate = req.BirthDate.AsTime()
	}

	if req.Gender != "" {
		registrationReq.Gender = models.Gender(req.Gender)
	}

	account, err := h.vkService.CreateAccount(ctx, registrationReq)
	if err != nil {
		h.logger.Error("Failed to create account", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to create account: %v", err)
	}

	return h.accountToProto(account), nil
}

func (h *GRPCHandler) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.Account, error) {
	id, err := primitive.ObjectIDFromHex(req.AccountId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid account ID: %v", err)
	}

	account, err := h.vkService.GetAccount(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "account not found: %v", err)
	}

	return h.accountToProto(account), nil
}

func (h *GRPCHandler) ListAccounts(ctx context.Context, req *pb.ListAccountsRequest) (*pb.ListAccountsResponse, error) {
	limit := int64(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	var accountStatus models.AccountStatus
	if req.Status != "" {
		accountStatus = models.AccountStatus(req.Status)
	}

	accounts, err := h.vkService.GetAccountsByStatus(ctx, accountStatus, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list accounts: %v", err)
	}

	protoAccounts := make([]*pb.Account, 0, len(accounts))
	for _, account := range accounts {
		protoAccounts = append(protoAccounts, h.accountToProto(account))
	}

	return &pb.ListAccountsResponse{
		Accounts: protoAccounts,
		Total:    int32(len(accounts)),
		Limit:    int32(limit),
		Offset:   req.Offset,
	}, nil
}

func (h *GRPCHandler) UpdateAccountStatus(ctx context.Context, req *pb.UpdateStatusRequest) (*pb.Account, error) {
	id, err := primitive.ObjectIDFromHex(req.AccountId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid account ID: %v", err)
	}

	accountStatus := models.AccountStatus(req.Status)
	if err := h.vkService.UpdateAccountStatus(ctx, id, accountStatus); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update account status: %v", err)
	}

	// Get updated account
	account, err := h.vkService.GetAccount(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get updated account: %v", err)
	}

	return h.accountToProto(account), nil
}

func (h *GRPCHandler) RetryRegistration(ctx context.Context, req *pb.RetryRequest) (*pb.Account, error) {
	id, err := primitive.ObjectIDFromHex(req.AccountId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid account ID: %v", err)
	}

	if err := h.vkService.RetryRegistration(ctx, id); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retry registration: %v", err)
	}

	// Get account
	account, err := h.vkService.GetAccount(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get account: %v", err)
	}

	return h.accountToProto(account), nil
}

func (h *GRPCHandler) DeleteAccount(ctx context.Context, req *pb.DeleteAccountRequest) (*emptypb.Empty, error) {
	id, err := primitive.ObjectIDFromHex(req.AccountId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid account ID: %v", err)
	}

	if err := h.vkService.DeleteAccount(ctx, id); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete account: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (h *GRPCHandler) GetStatistics(ctx context.Context, req *emptypb.Empty) (*pb.Statistics, error) {
	stats, err := h.vkService.GetStatistics(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get statistics: %v", err)
	}

	byStatus := make(map[string]int64)
	for k, v := range stats.ByStatus {
		byStatus[string(k)] = v
	}

	return &pb.Statistics{
		Total:          stats.Total,
		ByStatus:       byStatus,
		SuccessRate:    stats.SuccessRate,
		AverageRetries: stats.AverageRetries,
		LastHour:       stats.LastHour,
		Last_24Hours:   stats.Last24Hours,
	}, nil
}

func (h *GRPCHandler) accountToProto(account *models.VKAccount) *pb.Account {
	protoAccount := &pb.Account{
		Id:             account.ID.Hex(),
		Phone:          account.Phone,
		Email:          account.Email,
		FirstName:      account.FirstName,
		LastName:       account.LastName,
		Username:       account.Username,
		UserId:         account.UserID,
		Status:         string(account.Status),
		ActivationId:   account.ActivationID,
		UserAgent:      account.UserAgent,
		RegistrationIp: account.RegistrationIP,
		ErrorMessage:   account.ErrorMessage,
		RetryCount:     int32(account.RetryCount),
		CreatedAt:      timestamppb.New(account.CreatedAt),
		UpdatedAt:      timestamppb.New(account.UpdatedAt),
	}

	if !account.ProxyID.IsZero() {
		protoAccount.ProxyId = account.ProxyID.Hex()
	}

	if account.LastLoginAt != nil {
		protoAccount.LastLoginAt = timestamppb.New(*account.LastLoginAt)
	}

	// Convert fingerprint to string map
	if account.Fingerprint != nil {
		protoAccount.Fingerprint = make(map[string]string)
		for k, v := range account.Fingerprint {
			if strVal, ok := v.(string); ok {
				protoAccount.Fingerprint[k] = strVal
			}
		}
	}

	return protoAccount
}
