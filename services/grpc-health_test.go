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
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	health "google.golang.org/grpc/health/grpc_health_v1"

	"go.6river.tech/mmmbbb/defaults"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/enttest"
	"go.6river.tech/mmmbbb/internal"
	"go.6river.tech/mmmbbb/internal/testutil"
)

type mockReady struct{ error }

func (m mockReady) Ready() error {
	return m.error
}

func Test_healthServer_Check(t *testing.T) {
	client := enttest.ClientForTest(t)
	deadClient := enttest.ClientForTest(t)
	assert.NoError(t, deadClient.Close())

	// TODO: reintroduce health statuses to this test via `readies`

	type fields struct {
		client  *ent.Client
		readies []ReadyCheck
	}
	type args struct {
		ctx context.Context
		req *health.HealthCheckRequest
	}
	ready := []ReadyCheck{mockReady{nil}}
	tests := []struct {
		name      string
		fields    fields
		args      args
		want      *health.HealthCheckResponse
		assertion assert.ErrorAssertionFunc
	}{
		{
			"no db",
			fields{nil, ready},
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING},
			assert.NoError,
		},
		{
			"one not ready",
			fields{client, []ReadyCheck{mockReady{nil}, mockReady{errors.New("not ready")}}},
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING},
			assert.NoError,
		},
		{
			"healthy db",
			fields{client, ready},
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_SERVING},
			assert.NoError,
		},
		{
			"broken db",
			fields{deadClient, ready},
			args{},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING},
			assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &healthServer{
				client:  tt.fields.client,
				readies: tt.fields.readies,
			}
			if tt.args.ctx == nil {
				tt.args.ctx = testutil.Context(t)
			}
			if tt.args.req == nil {
				tt.args.req = &health.HealthCheckRequest{}
			}
			got, err := s.Check(tt.args.ctx, tt.args.req)
			tt.assertion(t, err)
			assert.Equal(t, tt.want.GetStatus().String(), got.GetStatus().String())
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
		args      args
		readies   []ReadyCheck
		want      *health.HealthCheckResponse
		assertion assert.ErrorAssertionFunc
	}
	tests := []test{
		// "no db" is pretty hard to do without a lot of test-specific hooks to
		// access the back-end server impl object
		{
			"not init",
			func(t *testing.T, client *ent.Client, tt *test) {},
			args{},
			nil,
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING},
			assert.NoError,
		},
		{
			"not ready",
			func(t *testing.T, client *ent.Client, tt *test) {
			},
			args{},
			[]ReadyCheck{mockReady{errors.New("not ready")}},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING},
			assert.NoError,
		},
		{
			"healthy",
			func(t *testing.T, client *ent.Client, tt *test) {
			},
			args{},
			[]ReadyCheck{mockReady{nil}},
			&health.HealthCheckResponse{Status: health.HealthCheckResponse_SERVING},
			assert.NoError,
		},
		{
			"broken db",
			func(t *testing.T, client *ent.Client, tt *test) {
				require.NoError(t, client.Close())
			},
			args{},
			[]ReadyCheck{mockReady{nil}},
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
			entClient := initGrpcService(t, tt.readies)
			conn, err := grpc.NewClient(
				"localhost:"+strconv.Itoa(internal.ResolvePort(defaults.Port, defaults.GRPCOffset)),
				opts...,
			)
			require.NoError(t, err)
			grpcClient := health.NewHealthClient(conn)

			if tt.args.ctx == nil {
				tt.args.ctx = testutil.Context(t)
			}
			tt.before(t, entClient, &tt)
			if tt.args.req == nil {
				tt.args.req = &health.HealthCheckRequest{}
			}
			got, err := grpcClient.Check(tt.args.ctx, tt.args.req)
			tt.assertion(t, err)
			assert.Equal(t, tt.want.GetStatus().String(), got.GetStatus().String())
		})
	}
}
