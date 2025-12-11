package handlers

import (
	"context"

	"github.com/grigta/conveer/services/max-service/internal/models"
	"github.com/grigta/conveer/services/max-service/internal/service"
	pb "github.com/grigta/conveer/services/max-service/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCHandler handles gRPC requests
type GRPCHandler struct {
	pb.UnimplementedMaxServiceServer
	service *service.MaxService
}

// NewGRPCHandler creates a new gRPC handler
func NewGRPCHandler(service *service.MaxService) *GRPCHandler {
	return &GRPCHandler{
		service: service,
	}
}

// CreateAccount creates a new account
func (h *GRPCHandler) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.CreateAccountResponse, error) {
	registrationReq := &models.RegistrationRequest{
		VKAccountID:        req.VkAccountId,
		FirstName:          req.FirstName,
		LastName:           req.LastName,
		Username:           req.Username,
		AvatarURL:          req.AvatarUrl,
		PreferredCountry:   req.PreferredCountry,
		CreateNewVKAccount: req.CreateNewVkAccount,
	}
	
	result, err := h.service.CreateAccount(ctx, registrationReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	
	return &pb.CreateAccountResponse{
		Success:   result.Success,
		AccountId: result.AccountID,
		Status:    result.Status,
	}, nil
}

// GetAccount retrieves an account
func (h *GRPCHandler) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.Account, error) {
	account, err := h.service.GetAccount(ctx, req.AccountId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "account not found")
	}

	return &pb.Account{
		Id:           account.ID.Hex(),
		VkAccountId:  account.VKAccountID,
		VkUserId:     account.VKUserID,
		FirstName:    account.FirstName,
		LastName:     account.LastName,
		Username:     account.Username,
		AvatarUrl:    account.AvatarURL,
		Status:       string(account.Status),
		Phone:        account.Phone,
		IsVkLinked:   account.IsVKLinked,
		CreatedAt:    account.CreatedAt.Unix(),
		UpdatedAt:    account.UpdatedAt.Unix(),
		ErrorMessage: account.ErrorMessage,
		RetryCount:   int32(account.RetryCount),
	}, nil
}

// ListAccounts lists accounts
func (h *GRPCHandler) ListAccounts(ctx context.Context, req *pb.ListAccountsRequest) (*pb.AccountList, error) {
	filter := make(map[string]interface{})
	if req.Status != "" {
		filter["status"] = req.Status
	}
	filter["deleted_at"] = nil
	
	accounts, total, err := h.service.ListAccounts(ctx, filter, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	
	pbAccounts := make([]*pb.Account, len(accounts))
	for i, account := range accounts {
		pbAccounts[i] = &pb.Account{
			Id:           account.ID.Hex(),
			VkAccountId:  account.VKAccountID,
			VkUserId:     account.VKUserID,
			FirstName:    account.FirstName,
			LastName:     account.LastName,
			Username:     account.Username,
			AvatarUrl:    account.AvatarURL,
			Status:       string(account.Status),
			Phone:        account.Phone,
			IsVkLinked:   account.IsVKLinked,
			CreatedAt:    account.CreatedAt.Unix(),
			UpdatedAt:    account.UpdatedAt.Unix(),
			ErrorMessage: account.ErrorMessage,
			RetryCount:   int32(account.RetryCount),
		}
	}
	
	return &pb.AccountList{
		Accounts: pbAccounts,
		Total:    total,
	}, nil
}

// UpdateAccountStatus updates account status
func (h *GRPCHandler) UpdateAccountStatus(ctx context.Context, req *pb.UpdateAccountStatusRequest) (*pb.UpdateAccountStatusResponse, error) {
	err := h.service.UpdateAccountStatus(
		ctx,
		req.AccountId,
		models.AccountStatus(req.Status),
		req.ErrorMessage,
	)
	
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	
	return &pb.UpdateAccountStatusResponse{
		Success: true,
	}, nil
}

// RetryRegistration retries registration
func (h *GRPCHandler) RetryRegistration(ctx context.Context, req *pb.RetryRegistrationRequest) (*pb.RetryRegistrationResponse, error) {
	err := h.service.RetryRegistration(ctx, req.AccountId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	
	return &pb.RetryRegistrationResponse{
		Success: true,
	}, nil
}

// LinkVKAccount links VK account to Max account
func (h *GRPCHandler) LinkVKAccount(ctx context.Context, req *pb.LinkVKAccountRequest) (*pb.LinkVKAccountResponse, error) {
	err := h.service.LinkVKAccount(ctx, req.MaxAccountId, req.VkAccountId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.LinkVKAccountResponse{
		Success: true,
	}, nil
}

// DeleteAccount deletes an account
func (h *GRPCHandler) DeleteAccount(ctx context.Context, req *pb.DeleteAccountRequest) (*pb.DeleteAccountResponse, error) {
	err := h.service.DeleteAccount(ctx, req.AccountId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.DeleteAccountResponse{
		Success: true,
	}, nil
}

// GetStatistics returns statistics
func (h *GRPCHandler) GetStatistics(ctx context.Context, req *pb.GetStatisticsRequest) (*pb.Statistics, error) {
	stats, err := h.service.GetStatistics(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	
	statusMap := make(map[string]int64)
	for k, v := range stats.AccountsByStatus {
		statusMap[k] = v
	}
	
	return &pb.Statistics{
		TotalAccounts:    stats.TotalAccounts,
		AccountsByStatus: statusMap,
		SuccessRate:      float32(stats.SuccessRate),
		AverageRetries:   float32(stats.AverageRetries),
		VkLinkedCount:    stats.VKLinkedCount,
		LastHour:         stats.LastHour,
		Last_24Hours:     stats.Last24Hours,
	}, nil
}
