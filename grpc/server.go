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
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/faults"
	"go.6river.tech/mmmbbb/internal"
	"go.6river.tech/mmmbbb/logging"
)

type GrpcInitializer func(context.Context, *grpc.Server, *ent.Client) error

type grpcServer struct {
	defaultPort  int
	offset       int
	realPort     int
	listenConfig *net.ListenConfig
	opts         []grpc.ServerOption
	faults       *faults.Set

	initializers []GrpcInitializer

	logger *logging.Logger
	server *grpc.Server
}

func (s *grpcServer) Name() string {
	if s.realPort != 0 && s.realPort != s.defaultPort+s.offset {
		return fmt.Sprintf("grpc@%d+%d=%d", s.defaultPort, s.offset, s.realPort)
	} else {
		return fmt.Sprintf("grpc@%d", s.defaultPort+s.offset)
	}
}

// this is mostly just for tests, race wouldn't likely hit this in a real app
var grpcPromInitOnce sync.Once

func (s *grpcServer) Initialize(
	ctx context.Context,
	client *ent.Client,
) error {
	s.realPort = internal.ResolvePort(s.defaultPort, s.offset)
	s.logger = logging.GetLogger("server/grpc/" + strconv.Itoa(s.realPort))

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			s.logUnary,
			grpc_prometheus.UnaryServerInterceptor,
			UnaryFaultInjector(s.faults),
		),
		grpc.ChainStreamInterceptor(
			s.logStream,
			grpc_prometheus.StreamServerInterceptor,
			StreamFaultInjector(s.faults),
		),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			// be tolerant of aggressive client keepalives
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	}
	if s.opts != nil {
		opts = append(opts, s.opts...)
	}
	grpcPromInitOnce.Do(func() {
		grpc_prometheus.EnableHandlingTimeHistogram()
	})
	if s.listenConfig == nil {
		s.listenConfig = &net.ListenConfig{}
	}

	s.server = grpc.NewServer(opts...)

	for _, i := range s.initializers {
		if err := i(ctx, s.server, client); err != nil {
			return err
		}
	}

	// this allows the (C++ based) `grpc_cli` tool to poke at us
	reflection.Register(s.server)

	return nil
}

func (s *grpcServer) logUnary(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	elapsed := time.Since(start)
	var evt *zerolog.Event
	if err == nil {
		evt = s.logger.Trace()
	} else if stat, _ := status.FromError(err); stat.Code() == codes.Canceled || stat.Code() == codes.DeadlineExceeded {
		// cancellation & deadline exceeded errors are boring, keep them at trace
		evt = s.logger.Trace()
	} else {
		evt = s.logger.Error().Err(err)
	}
	evt.
		Str("method", info.FullMethod).
		Dur("latency", elapsed).
		Msg("unary")
	return resp, err
}

func (s *grpcServer) logStream(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()
	cs := &countingStream{ServerStream: ss}
	err := handler(srv, cs)
	elapsed := time.Since(start)
	var evt *zerolog.Event
	if err == nil {
		evt = s.logger.Trace()
	} else {
		evt = s.logger.Error().Err(err)
	}
	evt.
		Str("method", info.FullMethod).
		Dur("latency", elapsed).
		Uint64("sent", cs.numSent).
		Uint64("received", cs.numReceived).
		Uint64("errd", cs.numError).
		Msg("stream")
	return err
}

type countingStream struct {
	grpc.ServerStream
	numSent, numReceived, numError uint64
}

func (cs *countingStream) SendMsg(m interface{}) error {
	err := cs.ServerStream.SendMsg(m)
	if err != nil {
		atomic.AddUint64(&cs.numError, 1)
		return err
	}
	atomic.AddUint64(&cs.numSent, 1)
	return nil
}

func (cs *countingStream) RecvMsg(m interface{}) error {
	err := cs.ServerStream.RecvMsg(m)
	if err != nil {
		atomic.AddUint64(&cs.numError, 1)
		return err
	}
	atomic.AddUint64(&cs.numReceived, 1)
	return nil
}

func (s *grpcServer) Start(ctx context.Context, ready chan<- struct{}) error {
	defer func() {
		if ready != nil {
			close(ready)
		}
	}()

	s.logger.Trace().Msg("startup requested")

	// run the listener on the background context, we'll separately monitor ctx to
	// stop the server
	l, err := s.listenConfig.Listen(context.Background(), "tcp", fmt.Sprintf(":%d", s.realPort))
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to open gRPC listen socket")

		return fmt.Errorf("Failed to open listen port for grpc server: %w", err)
	}

	// ask the server to stop when the context is canceled
	go func() {
		<-ctx.Done()
		ss := s.server
		if ss != nil {
			s.logger.Trace().Msg("Stopping gRPC server")
			s.server.GracefulStop()
		}
		// else someone beat us to it
	}()

	s.logger.Info().Int("port", s.realPort).Msg("gRPC server listening")
	close(ready)
	ready = nil
	err = s.server.Serve(l)
	if errors.Is(err, grpc.ErrServerStopped) {
		// this isn't an error
		err = nil
	}
	if err != nil {
		s.logger.Error().Err(err).Msg("Service failed")
	} else {
		s.logger.Trace().Msg("Service stopped")
	}
	return err
}

func (s *grpcServer) Cleanup(context.Context) error {
	if s.server != nil {
		// just in case, do we really need this?
		s.server.Stop()
		s.server = nil
	}

	return nil
}

func NewGrpcService(
	port, offset int,
	opts []grpc.ServerOption,
	faults *faults.Set,
	initializers ...GrpcInitializer,
) *grpcServer {
	return &grpcServer{
		defaultPort:  port,
		offset:       offset,
		opts:         opts,
		faults:       faults,
		initializers: initializers,
	}
}
