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

package grpc

import (
	"context"
	"net/http"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gin-gonic/gin"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/internal"
	"go.6river.tech/mmmbbb/logging"
)

type gatewayService struct {
	name         string
	port, offset int
	routes       gin.IRoutes
	paths        []string
	initHandlers func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error

	logger   *logging.Logger
	endpoint string
	conn     *grpc.ClientConn
}

func NewGatewayService(
	name string,
	port, offset int,
	routes gin.IRoutes,
	paths []string,
	initHandlers func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error,
) *gatewayService {
	return &gatewayService{
		name: name,
		port: port, offset: offset,
		routes:       routes,
		paths:        paths,
		initHandlers: initHandlers,
	}
}

func (s *gatewayService) Name() string {
	return "grpc-http-gateway(" + s.name + ")"
}

func (s *gatewayService) Initialize(context.Context, *ent.Client) error {
	s.logger = logging.GetLogger("grpc/http-gateway")
	s.endpoint = "localhost:" + strconv.Itoa(internal.ResolvePort(s.port, s.offset))
	return nil
}

func (s *gatewayService) Start(ctx context.Context, ready chan<- struct{}) error {
	defer close(ready)

	// TODO: add an example of how to use runtime.WithHealthzEndpoint(). for now
	// this requires external info from the app, and so is something the app can
	// do from the initHandlers hook
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// retry if we get connection refused, as this proxy might start before
		// the grpc server starts ... this doesn't really seem to work however
		grpc.FailOnNonTempDialError(false),
	}

	var err error
	if s.conn, err = grpc.NewClient(s.endpoint, opts...); err != nil {
		return err
	}
	if err = s.initHandlers(ctx, mux, s.conn); err != nil {
		return err
	}
	for _, p := range s.paths {
		s.routes.Any(p, gin.WrapF(func(w http.ResponseWriter, r *http.Request) {
			mux.ServeHTTP(w, r)
		}))
	}

	// service doesn't need to do anything, just is used for delayed registration
	return nil
}

func (s *gatewayService) Cleanup(context.Context) error {
	if err := s.conn.Close(); err != nil {
		s.logger.Error().Err(err).Msgf("Failed to close conn to %s", s.endpoint)
		return err
	}
	return nil
}
