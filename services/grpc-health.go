package services

import (
	"context"
	"errors"
	"time"

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
	var err error
	if req.GetService() == "" {
		err = s.services.WaitAllReady(ctx)
	} else {
		err = s.services.WaitReadyByName(ctx, req.GetService())
	}

	if err != nil {
		if errors.Is(err, registry.ServiceNotFoundError) {
			return &health.HealthCheckResponse{
				Status: health.HealthCheckResponse_SERVICE_UNKNOWN,
			}, nil
		}
		return &health.HealthCheckResponse{
			Status: health.HealthCheckResponse_NOT_SERVING,
		}, nil
	}
	return &health.HealthCheckResponse{
		Status: health.HealthCheckResponse_SERVING,
	}, nil
}
