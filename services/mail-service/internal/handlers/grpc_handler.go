package handlers

import (
	"context"

	"github.com/conveer/mail-service/internal/models"
	"github.com/conveer/mail-service/internal/service"
	pb "github.com/conveer/mail-service/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCHandler handles gRPC requests
type GRPCHandler struct {
	pb.UnimplementedMailServiceServer
	service *service.MailService
}

// NewGRPCHandler creates a new gRPC handler
func NewGRPCHandler(service *service.MailService) *GRPCHandler {
	return &GRPCHandler{
		service: service,
	}
}

// CreateAccount creates a new account
func (h *GRPCHandler) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.CreateAccountResponse, error) {
	registrationReq := &models.RegistrationRequest{
		FirstName:            req.FirstName,
		LastName:             req.LastName,
		BirthDate:            req.BirthDate,
		Gender:               req.Gender,
		PreferredCountry:     req.PreferredCountry,
		UsePhoneVerification: req.UsePhoneVerification,
		CustomEmailPrefix:    req.CustomEmailPrefix,
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
		Email:        account.Email,
		FirstName:    account.FirstName,
		LastName:     account.LastName,
		BirthDate:    account.BirthDate,
		Gender:       account.Gender,
		Status:       string(account.Status),
		Phone:        account.Phone,
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
			Email:        account.Email,
			FirstName:    account.FirstName,
			LastName:     account.LastName,
			BirthDate:    account.BirthDate,
			Gender:       account.Gender,
			Status:       string(account.Status),
			Phone:        account.Phone,
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
		LastHour:         stats.LastHour,
		Last_24Hours:     stats.Last24Hours,
	}, nil
}
