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

package server

import (
	"fmt"
	_ "net/http/pprof"
	"os"

	"github.com/Depado/ginprom"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/location"
	"github.com/gin-contrib/logger"
	"github.com/gin-gonic/gin"

	"go.6river.tech/mmmbbb/logging"

	// gin-gonic's cors module is a mess
	cors "github.com/rs/cors/wrapper/gin"
	"github.com/rs/zerolog"
)

func NewEngine() *gin.Engine {
	if os.Getenv("GIN_MODE") == "" {
		// TODO: don't use NODE_ENV for things that aren't nodejs
		switch os.Getenv("NODE_ENV") {
		case "production":
			gin.SetMode(gin.ReleaseMode)
		case "development", "":
			gin.SetMode(gin.DebugMode)
		case "test", "acceptance":
			// TODO: actually probably want debug mode here too?
			gin.SetMode(gin.TestMode)
		default:
			panic(fmt.Errorf("unrecognized NODE_ENV value: '%s'", os.Getenv("NODE_ENV")))
		}
	}

	// logging is all going through zerolog, which won't support color
	gin.DisableConsoleColor()

	ginDebugLogger := logging.GetLogger("gin/debug")
	gin.DefaultWriter = ginDebugLogger
	gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, numHandlers int) {
		ginDebugLogger.Debug().
			Str("method", httpMethod).
			Str("path", absolutePath).
			Str("handler", handlerName).
			Int("numHandlers", numHandlers).
			Msg("route added")
	}

	r := gin.New()

	// request contexts should be request bound
	r.ContextWithFallback = true

	// attach custom logging
	requestLogger := logging.GetLogger("gin/request")
	r.Use(logger.SetLogger(
		logger.WithLogger(func(c *gin.Context, l zerolog.Logger) zerolog.Logger {
			return requestLogger.Current()
		}),
		logger.WithUTC(true),
	))

	// Set up prometheus, before recovery so it can see the recovered responses
	p := ginprom.New(
		ginprom.Engine(r),
		ginprom.Path("/metrics"),
	)
	r.Use(p.Instrument())

	// gzip encoding
	r.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithDecompressFn(gzip.DefaultDecompressHandle)))

	// attach recovery middleware using request logger
	// This needs to be AFTER the gzip layer, as the gzip layer has a defer that
	// will result in sending an empty 200 during panic recovery if this is above
	// that layer
	// TODO: we should make a better recovery handler that is better suited to
	// our idioms and to zerolog, including not trying to color things even when
	// console coloring above is disabled
	r.Use(gin.RecoveryWithWriter(requestLogger))

	// basic CORS policy
	// debugging note: unlike loopback(4), this cors extension will only send the Accept-* header(s)
	// when the request includes the Origin header. This is fine.
	r.Use(cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
	}))

	// make info from reverse proxy headers available to handlers
	r.Use(location.Default())

	return r
}
