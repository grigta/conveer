package handlers

import (
	"context"

	"conveer/pkg/logger"
	"conveer/services/telegram-service/internal/models"
	"conveer/services/telegram-service/internal/service"
	pb "conveer/services/telegram-service/proto"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GRPCHandler struct {
	pb.UnimplementedTelegramServiceServer
	service service.TelegramService
	logger  logger.Logger
}

func NewGRPCHandler(service service.TelegramService, logger logger.Logger) *GRPCHandler {
	return &GRPCHandler{
		service: service,
		logger:  logger,
	}
}

func (h *GRPCHandler) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.Account, error) {
	registrationReq := &models.RegistrationRequest{
		FirstName:        req.FirstName,
		LastName:         req.LastName,
		Username:         req.Username,
		Bio:              req.Bio,
		AvatarURL:        req.AvatarUrl,
		EnableTwoFactor:  req.EnableTwoFactor,
		PreferredCountry: req.PreferredCountry,
		UseRandomProfile: req.UseRandomProfile,
		ApiID:            int(req.ApiId),
		ApiHash:          req.ApiHash,
	}

	account, err := h.service.CreateAccount(ctx, registrationReq)
	if err != nil {
		h.logger.Error("Failed to create account", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to create account: %v", err)
	}

	return h.accountToProto(account), nil
}

func (h *GRPCHandler) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.Account, error) {
	accountID, err := primitive.ObjectIDFromHex(req.AccountId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid account ID: %v", err)
	}

	account, err := h.service.GetAccount(ctx, accountID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "account not found: %v", err)
	}

	return h.accountToProto(account), nil
}

func (h *GRPCHandler) ListAccounts(ctx context.Context, req *pb.ListAccountsRequest) (*pb.ListAccountsResponse, error) {
	var accountStatus models.AccountStatus
	if req.Status != "" {
		accountStatus = models.AccountStatus(req.Status)
	}

	accounts, total, err := h.service.ListAccounts(ctx, accountStatus, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list accounts: %v", err)
	}

	protoAccounts := make([]*pb.Account, len(accounts))
	for i, account := range accounts {
		protoAccounts[i] = h.accountToProto(account)
	}

	return &pb.ListAccountsResponse{
		Accounts: protoAccounts,
		Total:    int32(total),
		Limit:    req.Limit,
		Offset:   req.Offset,
	}, nil
}

func (h *GRPCHandler) UpdateAccountStatus(ctx context.Context, req *pb.UpdateStatusRequest) (*pb.Account, error) {
	accountID, err := primitive.ObjectIDFromHex(req.AccountId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid account ID: %v", err)
	}

	account, err := h.service.UpdateAccountStatus(ctx, accountID, models.AccountStatus(req.Status))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update account status: %v", err)
	}

	return h.accountToProto(account), nil
}

func (h *GRPCHandler) RetryRegistration(ctx context.Context, req *pb.RetryRequest) (*pb.Account, error) {
	accountID, err := primitive.ObjectIDFromHex(req.AccountId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid account ID: %v", err)
	}

	account, err := h.service.RetryRegistration(ctx, accountID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retry registration: %v", err)
	}

	return h.accountToProto(account), nil
}

func (h *GRPCHandler) DeleteAccount(ctx context.Context, req *pb.DeleteAccountRequest) (*emptypb.Empty, error) {
	accountID, err := primitive.ObjectIDFromHex(req.AccountId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid account ID: %v", err)
	}

	if err := h.service.DeleteAccount(ctx, accountID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete account: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (h *GRPCHandler) GetStatistics(ctx context.Context, req *emptypb.Empty) (*pb.Statistics, error) {
	stats, err := h.service.GetStatistics(ctx)
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

func (h *GRPCHandler) accountToProto(account *models.TelegramAccount) *pb.Account {
	protoAccount := &pb.Account{
		Id:             account.ID.Hex(),
		Phone:          account.Phone,
		FirstName:      account.FirstName,
		LastName:       account.LastName,
		Username:       account.Username,
		UserId:         account.UserID,
		Bio:            account.Bio,
		AvatarUrl:      account.AvatarURL,
		Status:         string(account.Status),
		UserAgent:      account.UserAgent,
		RegistrationIp: account.RegistrationIP,
		ErrorMessage:   account.ErrorMessage,
		RetryCount:     int32(account.RetryCount),
		HasTwoFactor:   account.TwoFactorSecret != "",
		CreatedAt:      timestamppb.New(account.CreatedAt),
		UpdatedAt:      timestamppb.New(account.UpdatedAt),
	}

	if account.ProxyID != primitive.NilObjectID {
		protoAccount.ProxyId = account.ProxyID.Hex()
	}

	if account.ActivationID != "" {
		protoAccount.ActivationId = account.ActivationID
	}

	if account.LastLoginAt != nil {
		protoAccount.LastLoginAt = timestamppb.New(*account.LastLoginAt)
	}

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