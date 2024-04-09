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
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"

	"go.6river.tech/mmmbbb/internal"
	"go.6river.tech/mmmbbb/logging"
)

type HTTPService struct {
	defaultPort int
	offset      int
	realPort    int
	handler     *gin.Engine
	logger      *logging.Logger
	server      *http.Server
}

func (s *HTTPService) Name() string {
	if s.realPort != 0 && s.realPort != s.defaultPort+s.offset {
		return fmt.Sprintf("gin-http(%d+%d=%d)", s.defaultPort, s.offset, s.realPort)
	} else {
		return fmt.Sprintf("gin-http(%d)", s.defaultPort+s.offset)
	}
}

func (s *HTTPService) Start(ctx context.Context, ready chan<- struct{}) error {
	// split listen from serve so that we can log when the listen socket is ready
	s.logger.Trace().Str("addr", s.server.Addr).Msgf("listening")
	lc := net.ListenConfig{}
	l, err := lc.Listen(ctx, "tcp", s.server.Addr)
	if err != nil {
		return err
	}
	defer l.Close()
	s.logger.Info().
		// FIXME: get app version via registry.Values injection and report it
		// Str("version", version.SemrelVersion).
		Int("port", s.realPort).
		Msgf("Server is ready")
	eg, egCtx := errgroup.WithContext(ctx)
	// wait for the parent context to be canceled, then request a clean shutdown
	eg.Go(func() error {
		<-egCtx.Done()
		// run shutdown on the background context, since `ctx` is already done
		return s.server.Shutdown(context.Background())
	})
	eg.Go(func() error {
		close(ready)
		err = s.server.Serve(l)
		// ignore graceful shutdowns, they're not a real error
		if err == http.ErrServerClosed {
			err = nil
		}
		return err
	})
	return eg.Wait()
}

func (s *HTTPService) Cleanup(ctx context.Context) error {
	if err := s.server.Close(); err != nil {
		s.logger.Err(err).Msg("Server close failed")
		return err
	}

	return nil
}

// NewHTTPService configures an HTTP server service using the given routing
// engine.
//
// It will listen on `defaultPort` normally, unless overridden by the PORT
// environment variable.
func NewHTTPService(r *gin.Engine, defaultPort, offset int) *HTTPService {
	s := &HTTPService{
		defaultPort: defaultPort,
		offset:      offset,
		handler:     r,
		realPort:    internal.ResolvePort(defaultPort, offset),
	}
	s.logger = logging.GetLogger("server/gin-http/" + strconv.Itoa(s.realPort))
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.realPort),
		Handler: s.handler,
		// BaseContext will be set during Start
	}
	return s
}

func HttpServer(c *gin.Context) *http.Server {
	return c.Request.Context().Value(http.ServerContextKey).(*http.Server)
}
