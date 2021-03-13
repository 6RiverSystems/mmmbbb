package services

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.6river.tech/gosix/registry"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/grpc/health"
)

// TODO: this could be moved to common and implemented in terms of the generic
// EntClient

type healthServer struct {
	health.UnimplementedHealthServer

	client   *ent.Client
	services *registry.Registry
}

func (s *healthServer) Check(ctx context.Context, req *health.HealthCheckRequest) (*health.HealthCheckResponse, error) {
	if s.client == nil || s.services == nil {
		return &health.HealthCheckResponse{
			Status: health.HealthCheckResponse_NOT_SERVING,
		}, nil
	}

	// if we can't verify readiness within a second, things are probably in pretty
	// bad shape
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	if err := s.client.DB().PingContext(ctx); err != nil {
		return &health.HealthCheckResponse{
			Status: health.HealthCheckResponse_NOT_SERVING,
		}, nil
	}

	if !s.services.ServicesInitialized() || !s.services.ServicesStarted() {
		return &health.HealthCheckResponse{
			Status: health.HealthCheckResponse_NOT_SERVING,
		}, nil
	}
	if req.GetService() == "" {
		if err := s.services.WaitAllReady(ctx); err != nil {
			return &health.HealthCheckResponse{
				Status: health.HealthCheckResponse_NOT_SERVING,
			}, nil
		}
	} else {
		// TODO: lookup service by name and check if it's ready
		return nil, status.Error(codes.Unimplemented, "Health check for specific services not yet implemented")
	}

	return &health.HealthCheckResponse{
		Status: health.HealthCheckResponse_SERVING,
	}, nil
}
