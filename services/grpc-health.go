// Copyright (c) 2021 6 River Systems
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package services

import (
	"context"
	"time"

	health "google.golang.org/grpc/health/grpc_health_v1"

	"go.6river.tech/mmmbbb/ent"
)

// TODO: this could be moved to common and implemented in terms of the generic
// EntClient

type healthServer struct {
	health.UnimplementedHealthServer

	client  *ent.Client
	readies []ReadyCheck
}

func (s *healthServer) Check(ctx context.Context, req *health.HealthCheckRequest) (*health.HealthCheckResponse, error) {
	if s.client == nil || s.readies == nil {
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

	// TODO: restore ready-by-service support
	if req.Service != "" {
		return &health.HealthCheckResponse{
			Status: health.HealthCheckResponse_SERVICE_UNKNOWN,
		}, nil
	}

	for _, r := range s.readies {
		if err := r.Ready(); err != nil {
			return &health.HealthCheckResponse{
				Status: health.HealthCheckResponse_NOT_SERVING,
			}, nil
		}
	}

	return &health.HealthCheckResponse{
		Status: health.HealthCheckResponse_SERVING,
	}, nil
}
