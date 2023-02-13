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
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	health "google.golang.org/grpc/health/grpc_health_v1"

	"go.6river.tech/gosix/registry"
	"go.6river.tech/gosix/server"
	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/defaults"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/enttest"
)

func Test_healthServer_Check(t *testing.T) {
	client := enttest.ClientForTest(t)
	deadClient := enttest.ClientForTest(t)
	assert.NoError(t, deadClient.Close())

	servicesNoInit := registry.New(t.Name()+"noInit", nil)
	servicesNoStart := registry.New(t.Name()+"noStart", nil)
	assert.NoError(t, servicesNoStart.InitializeServices(testutils.ContextForTest(t), client))
	servicesRunning := registry.New(t.Name()+"running", nil)
	assert.NoError(t, servicesRunning.InitializeServices(testutils.ContextForTest(t), client))
	assert.NoError(t, servicesRunning.StartServices(testutils.ContextForTest(t)))

	type fields struct {
		client   *ent.Client
		services *registry.Registry
	}
	type args struct {
		ctx context.Context
		req *health.HealthCheckRequest
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		want      *health.HealthCheckResponse
		assertion assert.ErrorAssertionFunc
	}{
		{
			"no db",
			fields{nil, servicesRunning},
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING},
			assert.NoError,
		},
		{
			"no init",
			fields{client, servicesNoInit},
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING},
			assert.NoError,
		},
		{
			"no start",
			fields{client, servicesNoStart},
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING},
			assert.NoError,
		},
		{
			"healthy db",
			fields{client, servicesRunning},
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_SERVING},
			assert.NoError,
		},
		{
			"broken db",
			fields{deadClient, servicesRunning},
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING},
			assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &healthServer{
				client:   tt.fields.client,
				services: tt.fields.services,
			}
			if tt.args.ctx == nil {
				tt.args.ctx = testutils.ContextForTest(t)
			}
			if tt.args.req == nil {
				tt.args.req = &health.HealthCheckRequest{}
			}
			got, err := s.Check(tt.args.ctx, tt.args.req)
			tt.assertion(t, err)
			assert.Equal(t, tt.want.GetStatus(), got.GetStatus())
		})
	}
}

// again, acceptance mode
func TestGrpc_healthServer_Check(t *testing.T) {
	type args struct {
		ctx context.Context
		req *health.HealthCheckRequest
	}
	type test struct {
		name      string
		before    func(t *testing.T, client *ent.Client, tt *test)
		services  *registry.Registry
		args      args
		want      *health.HealthCheckResponse
		assertion assert.ErrorAssertionFunc
	}
	tests := []test{
		// "no db" is pretty hard to do without a lot of test-specific hooks to
		// access the back-end server impl object
		{
			"not init",
			func(t *testing.T, client *ent.Client, tt *test) {},
			registry.New(t.Name()+"not-init", nil),
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING},
			assert.NoError,
		},
		{
			"not started",
			func(t *testing.T, client *ent.Client, tt *test) {
				assert.NoError(t, tt.services.InitializeServices(tt.args.ctx, client))
			},
			registry.New(t.Name()+"not-started", nil),
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING},
			assert.NoError,
		},
		{
			"healthy",
			func(t *testing.T, client *ent.Client, tt *test) {
				assert.NoError(t, tt.services.InitializeServices(tt.args.ctx, client))
				assert.NoError(t, tt.services.StartServices(tt.args.ctx))
			},
			registry.New(t.Name()+"healthy", nil),
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_SERVING},
			assert.NoError,
		},
		{
			"broken db",
			func(t *testing.T, client *ent.Client, tt *test) {
				require.NoError(t, client.Close())
			},
			registry.New(t.Name()+"broken-db", nil),
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING},
			assert.NoError,
		},
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// retry if we get connection refused, as this proxy might start before
		// the grpc server starts ... this doesn't really seem to work however
		grpc.FailOnNonTempDialError(false),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entClient := initGrpcService(t, tt.services)
			conn, err := grpc.Dial(
				"localhost:"+strconv.Itoa(server.ResolvePort(defaults.Port, defaults.GRPCOffset)),
				opts...,
			)
			require.NoError(t, err)
			grpcClient := health.NewHealthClient(conn)

			if tt.args.ctx == nil {
				tt.args.ctx = testutils.ContextForTest(t)
			}
			tt.before(t, entClient, &tt)
			if tt.args.req == nil {
				tt.args.req = &health.HealthCheckRequest{}
			}
			got, err := grpcClient.Check(tt.args.ctx, tt.args.req)
			tt.assertion(t, err)
			assert.Equal(t, tt.want.GetStatus(), got.GetStatus())
		})
	}
}
