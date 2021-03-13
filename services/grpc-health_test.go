package services

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"go.6river.tech/gosix/registry"
	"go.6river.tech/gosix/server"
	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/defaults"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/enttest"
	"go.6river.tech/mmmbbb/grpc/health"
)

func Test_healthServer_Check(t *testing.T) {
	client := enttest.ClientForTest(t)
	deadClient := enttest.ClientForTest(t)
	assert.NoError(t, deadClient.Close())

	servicesNoInit := &registry.Registry{}
	servicesNoStart := &registry.Registry{}
	assert.NoError(t, servicesNoStart.InitializeServices(testutils.ContextForTest(t), client))
	servicesRunning := &registry.Registry{}
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
			&registry.Registry{},
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING},
			assert.NoError,
		},
		{
			"not started",
			func(t *testing.T, client *ent.Client, tt *test) {
				assert.NoError(t, tt.services.InitializeServices(tt.args.ctx, client))
			},
			&registry.Registry{},
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
			&registry.Registry{},
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_SERVING},
			assert.NoError,
		},
		{
			"broken db",
			func(t *testing.T, client *ent.Client, tt *test) {
				require.NoError(t, client.Close())
			},
			&registry.Registry{},
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING},
			assert.NoError,
		},
	}

	opts := []grpc.DialOption{
		grpc.WithInsecure(),
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
