package handlers

import (
	"context"

	"github.com/grigta/conveer/services/proxy-service/internal/models"
	pb "github.com/grigta/conveer/services/proxy-service/proto"
	"github.com/grigta/conveer/services/proxy-service/internal/repository"
	"github.com/grigta/conveer/services/proxy-service/internal/service"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCHandler struct {
	pb.UnimplementedProxyServiceServer
	proxyService *service.ProxyService
	proxyRepo    *repository.ProxyRepository
	logger       *logrus.Logger
}

func NewGRPCHandler(
	proxyService *service.ProxyService,
	proxyRepo *repository.ProxyRepository,
	logger *logrus.Logger,
) *GRPCHandler {
	return &GRPCHandler{
		proxyService: proxyService,
		proxyRepo:    proxyRepo,
		logger:       logger,
	}
}

func (h *GRPCHandler) AllocateProxy(ctx context.Context, req *pb.AllocateProxyRequest) (*pb.ProxyResponse, error) {
	request := models.ProxyAllocationRequest{
		AccountID: req.AccountId,
		Type:      models.ProxyType(req.Type),
		Country:   req.Country,
		Protocol:  models.ProxyProtocol(req.Protocol),
	}

	proxy, err := h.proxyService.AllocateProxy(ctx, request)
	if err != nil {
		h.logger.WithError(err).Error("Failed to allocate proxy")
		return nil, status.Errorf(codes.Internal, "failed to allocate proxy: %v", err)
	}

	return &pb.ProxyResponse{
		Id:        proxy.ID.Hex(),
		Ip:        proxy.IP,
		Port:      int32(proxy.Port),
		Username:  proxy.Username,
		Password:  proxy.Password,
		Protocol:  string(proxy.Protocol),
		Type:      string(proxy.Type),
		Country:   proxy.Country,
		City:      proxy.City,
		Status:    string(proxy.Status),
		ExpiresAt: proxy.ExpiresAt.Unix(),
		Provider:  proxy.Provider,
	}, nil
}

func (h *GRPCHandler) ReleaseProxy(ctx context.Context, req *pb.ReleaseProxyRequest) (*pb.ReleaseProxyResponse, error) {
	if err := h.proxyService.ReleaseProxy(ctx, req.AccountId); err != nil {
		h.logger.WithError(err).Error("Failed to release proxy")
		return nil, status.Errorf(codes.Internal, "failed to release proxy: %v", err)
	}

	return &pb.ReleaseProxyResponse{
		Success: true,
		Message: "Proxy released successfully",
	}, nil
}

func (h *GRPCHandler) GetProxyForAccount(ctx context.Context, req *pb.GetProxyRequest) (*pb.ProxyResponse, error) {
	proxy, err := h.proxyService.GetProxyForAccount(ctx, req.AccountId)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get proxy for account")
		return nil, status.Errorf(codes.Internal, "failed to get proxy: %v", err)
	}

	if proxy == nil {
		return nil, status.Error(codes.NotFound, "no proxy found for account")
	}

	return &pb.ProxyResponse{
		Id:        proxy.ID.Hex(),
		Ip:        proxy.IP,
		Port:      int32(proxy.Port),
		Username:  proxy.Username,
		Password:  proxy.Password,
		Protocol:  string(proxy.Protocol),
		Type:      string(proxy.Type),
		Country:   proxy.Country,
		City:      proxy.City,
		Status:    string(proxy.Status),
		ExpiresAt: proxy.ExpiresAt.Unix(),
		Provider:  proxy.Provider,
	}, nil
}

func (h *GRPCHandler) GetProxyHealth(ctx context.Context, req *pb.GetProxyHealthRequest) (*pb.ProxyHealthResponse, error) {
	proxyID, err := primitive.ObjectIDFromHex(req.ProxyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid proxy ID")
	}

	health, err := h.proxyRepo.GetProxyHealthByID(ctx, proxyID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get proxy health")
		return nil, status.Errorf(codes.Internal, "failed to get proxy health: %v", err)
	}

	if health == nil {
		return nil, status.Error(codes.NotFound, "proxy health data not found")
	}

	return &pb.ProxyHealthResponse{
		ProxyId:         health.ProxyID.Hex(),
		Latency:         int32(health.Latency),
		FraudScore:      health.FraudScore,
		IsVpn:           health.IsVPN,
		IsProxy:         health.IsProxy,
		IsTor:           health.IsTor,
		BlacklistStatus: health.BlacklistStatus,
		LastCheck:       health.LastCheck.Unix(),
		FailedChecks:    int32(health.FailedChecks),
	}, nil
}

func (h *GRPCHandler) RotateProxy(ctx context.Context, req *pb.RotateProxyRequest) (*pb.ProxyResponse, error) {
	newProxy, err := h.proxyService.ForceRotateProxy(ctx, req.AccountId)
	if err != nil {
		h.logger.WithError(err).Error("Failed to rotate proxy")
		return nil, status.Errorf(codes.Internal, "failed to rotate proxy: %v", err)
	}

	return &pb.ProxyResponse{
		Id:        newProxy.ID.Hex(),
		Ip:        newProxy.IP,
		Port:      int32(newProxy.Port),
		Username:  newProxy.Username,
		Password:  newProxy.Password,
		Protocol:  string(newProxy.Protocol),
		Type:      string(newProxy.Type),
		Country:   newProxy.Country,
		City:      newProxy.City,
		Status:    string(newProxy.Status),
		ExpiresAt: newProxy.ExpiresAt.Unix(),
		Provider:  newProxy.Provider,
	}, nil
}

func (h *GRPCHandler) GetProxyStatistics(ctx context.Context, req *pb.GetStatisticsRequest) (*pb.ProxyStatisticsResponse, error) {
	stats, err := h.proxyService.GetProxyStatistics(ctx)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get statistics")
		return nil, status.Errorf(codes.Internal, "failed to get statistics: %v", err)
	}

	response := &pb.ProxyStatisticsResponse{
		TotalProxies:     stats.TotalProxies,
		ActiveProxies:    stats.ActiveProxies,
		ExpiredProxies:   stats.ExpiredProxies,
		BannedProxies:    stats.BannedProxies,
		TotalBindings:    stats.TotalBindings,
		ProxiesByType:    make(map[string]int64),
		ProxiesByCountry: make(map[string]int64),
		AvgFraudScore:    stats.AvgFraudScore,
		AvgLatency:       stats.AvgLatency,
	}

	for k, v := range stats.ProxiesByType {
		response.ProxiesByType[k] = v
	}

	for k, v := range stats.ProxiesByCountry {
		response.ProxiesByCountry[k] = v
	}

	return response, nil
}
