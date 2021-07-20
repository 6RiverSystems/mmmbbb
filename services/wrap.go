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
