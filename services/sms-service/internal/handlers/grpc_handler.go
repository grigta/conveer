package handlers

import (
	"context"
	"time"

	"conveer/sms-service/internal/service"
	pb "conveer/sms-service/proto"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCHandler struct {
	pb.UnimplementedSMSServiceServer
	smsService *service.SMSService
	logger     *logrus.Logger
}

func NewGRPCHandler(smsService *service.SMSService, logger *logrus.Logger) *GRPCHandler {
	return &GRPCHandler{
		smsService: smsService,
		logger:     logger,
	}
}

func (h *GRPCHandler) PurchaseNumber(ctx context.Context, req *pb.PurchaseNumberRequest) (*pb.PurchaseNumberResponse, error) {
	activation, err := h.smsService.PurchaseNumber(
		ctx,
		req.UserId,
		req.Service,
		req.Country,
		req.Operator,
		req.Provider,
		req.MaxPrice,
	)

	if err != nil {
		h.logger.Errorf("Failed to purchase number: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to purchase number: %v", err)
	}

	return &pb.PurchaseNumberResponse{
		ActivationId: activation.ActivationID,
		PhoneNumber:  activation.PhoneNumber,
		CountryCode:  activation.Country,
		Price:        float32(activation.Price),
		Provider:     activation.Provider,
		ExpiresAt:    activation.ExpiresAt.Unix(),
	}, nil
}