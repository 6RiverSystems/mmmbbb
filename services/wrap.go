package services

import (
	"context"

	entcommon "go.6river.tech/gosix/ent"
	"go.6river.tech/gosix/registry"
	"go.6river.tech/mmmbbb/ent"
)

// mmmbbbService is like registry.Service, but with common types replaced with
// mmmbbb-specific ones
type mmmbbbService interface {
	Name() string
	Initialize(context.Context, *registry.Registry, *ent.Client) error
	Start(context.Context, chan<- struct{}) error
	Cleanup(context.Context, *registry.Registry) error
}

type wrappedService struct {
	mmmbbbService
}

func (s *wrappedService) Initialize(ctx context.Context, services *registry.Registry, client_ entcommon.EntClient) error {
	client := client_.(*ent.Client)
	return s.mmmbbbService.Initialize(ctx, services, client)
}

func (s *wrappedService) Start(ctx context.Context, ready chan<- struct{}) error {
	return s.mmmbbbService.Start(ctx, ready)
}
func (s *wrappedService) Cleanup(ctx context.Context, reg *registry.Registry) error {
	return s.mmmbbbService.Cleanup(ctx, reg)
}

func wrapService(service mmmbbbService) registry.Service {
	return &wrappedService{mmmbbbService: service}
}
