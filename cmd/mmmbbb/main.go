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

package main

import (
	"context"
	"expvar"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/getkin/kin-openapi/openapi3"
	ginexpvar "github.com/gin-contrib/expvar"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.uber.org/fx"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"go.6river.tech/mmmbbb/controllers"
	"go.6river.tech/mmmbbb/db"
	"go.6river.tech/mmmbbb/defaults"
	"go.6river.tech/mmmbbb/ent"
	_ "go.6river.tech/mmmbbb/ent/runtime"
	"go.6river.tech/mmmbbb/faults"
	mbgrpc "go.6river.tech/mmmbbb/grpc"
	swaggerui "go.6river.tech/mmmbbb/internal/swagger-ui"
	"go.6river.tech/mmmbbb/logging"
	"go.6river.tech/mmmbbb/middleware"
	"go.6river.tech/mmmbbb/migrate"
	"go.6river.tech/mmmbbb/migrations"
	"go.6river.tech/mmmbbb/oas"
	"go.6river.tech/mmmbbb/server"
	"go.6river.tech/mmmbbb/services"
	_ "go.6river.tech/mmmbbb/services"
	"go.6river.tech/mmmbbb/version"
)

var testModeIgnoreArgs = false

func main() {
	if len(os.Args) > 1 {
		if len(os.Args) == 2 {
			switch os.Args[1] {
			case "--help":
				fmt.Println("mmmbbb does not accept command line arguments")
				return
			case "--version":
				fmt.Printf("This is mmmbbb version %s running on %s/%s\n",
					version.SemrelVersion, runtime.GOOS, runtime.GOARCH)
				return
			}
		}

		if !testModeIgnoreArgs {
			fmt.Fprintf(os.Stderr, "mmmbbb does not accept command line arguments: %v\n", os.Args[1:])
			//nolint:forbidigo
			os.Exit(1)
		}
	}

	if exitCode, err := NewApp().main(); err != nil {
		panic(err)
	} else if exitCode != 0 {
		//nolint:forbidigo // this is the one and only instance of os.Exit we want to have
		os.Exit(exitCode)
	}
}

type app struct {
	port int

	opts []fx.Option

	logger        *logging.Logger
	ginMiddleware []func(*gin.Engine) error
}

func NewApp() *app {
	app := &app{
		port: defaults.Port,
	}
	// TODO: move these to a provider, and hide GetLogger behind DI
	logging.ConfigureDefaultLogging()
	app.logger = logging.GetLogger(version.AppName)

	app.opts = []fx.Option{
		fx.Invoke(func() {
			// report the app version as an expvar
			expvar.NewString("version/" + version.AppName).Set(version.SemrelVersion)
		}),
		fx.Provide(func(l fx.Lifecycle, sd fx.Shutdowner) (context.Context, *errgroup.Group) {
			ctx, cancel := context.WithCancel(context.Background())
			eg, ctx := errgroup.WithContext(ctx)
			l.Append(fx.StartStopHook(
				func() {
					go func() {
						<-ctx.Done()
						err := eg.Wait()
						var opts []fx.ShutdownOption
						if err != nil {
							opts = append(opts, fx.ExitCode(1))
						}
						_ = sd.Shutdown(opts...)
					}()
				},
				func() {
					cancel()
					// this just waits for the eg to end, processing errors from it happens
					// in the goroutine
					_ = eg.Wait()
				},
			))
			return ctx, eg
		}),
		fx.Provide(func(
			ctx context.Context,
			l fx.Lifecycle,
		) (*sql.Driver, error) {
			db.SetDefaultDbName(version.AppName)
			drv, err := app.openDB(ctx, app.logger)
			if drv != nil {
				l.Append(fx.StopHook(func() error {
					err := drv.Close()
					if err != nil {
						app.logger.Error().Err(err).Msg("Failed to cleanup SQL connection")
					}
					return err
				}))
			}
			return drv, err
		}),
		fx.Provide(func(
			ctx context.Context,
			l fx.Lifecycle,
			drv *sql.Driver,
		) (*ent.Client, error) {
			client, err := app.setupDB(ctx, drv)
			return client, err
		}),
		fx.Provide(func() *faults.Set {
			f := faults.NewSet(version.AppName)
			f.MustRegister(prometheus.DefaultRegisterer)
			return f
		}),
		controllers.Module,
		fx.Provide(app.provideGrpc),
		fx.Provide(app.setupGin),
		// TODO: customize signal handlers?
		services.Module(),
	}

	return app
}

func (app *app) openDB(ctx context.Context, logger *logging.Logger) (drv *sql.Driver, err error) {
	// for 6mon friendliness, wait for up to 120 seconds for a db connection
	if err = func() error {
		dbWaitCtx, dbWaitCancel := context.WithDeadline(ctx, time.Now().Add(120*time.Second))
		defer dbWaitCancel()
		return db.WaitForDB(dbWaitCtx)
	}(); err != nil {
		return nil, err
	}

	if drv, err = db.OpenSqlForEnt(); err != nil {
		if drv != nil {
			if err := drv.Close(); err != nil {
				logger.Error().Err(err).Msg("Failed to cleanup SQL connection")
			}
		}
		return nil, err
	}
	return drv, nil
}

func (app *app) setupDB(ctx context.Context, drv *sql.Driver) (client *ent.Client, err error) {
	sqlLogger := logging.GetLogger(version.AppName + "/sql")
	sqlLoggerFunc := func(args ...interface{}) {
		for _, m := range args {
			// currently ent only sends us strings
			sqlLogger.Trace().Msg(m.(string))
		}
	}
	entDebug := strings.Contains(os.Getenv("DEBUG"), "sql")
	opts := []ent.Option{
		ent.Driver(drv),
		ent.Log(sqlLoggerFunc),
	}
	if entDebug {
		opts = append(opts, ent.Debug())
	}

	client = ent.NewClient(opts...)

	app.ginMiddleware = append(app.ginMiddleware, (func(engine *gin.Engine) error {
		engine.Use(middleware.WithEntClient(client, middleware.EntKey(db.GetDefaultDbName())))
		return nil
	}))
	// Setup db prometheus metrics
	prometheus.DefaultRegisterer.MustRegister(collectors.NewDBStatsCollector(drv.DB(), db.GetDefaultDbName()))

	m := &migrate.Migrator{}

	if ownMigrations, err := migrate.LoadFS(migrations.MessageBusMigrations, nil); err != nil {
		return client, err
	} else {
		m.SortAndAppend("", ownMigrations...)
	}

	// TODO: move this to a lifecycle hook?
	if err = db.Up(ctx, client, m); err != nil {
		return client, err
	}

	return client, nil
}

type ginParams struct {
	fx.In
	Ctx         context.Context
	L           fx.Lifecycle
	SD          fx.Shutdowner
	Client      *ent.Client
	Controllers []controllers.Controller `group:"controllers"`
}
type ginResults struct {
	fx.Out
	Engine     *gin.Engine
	HTTPServer *services.HTTPService
}

func (app *app) setupGin(params ginParams) (ginResults, error) {
	var res ginResults
	res.Engine = server.NewEngine()
	for _, m := range app.ginMiddleware {
		if err := m(res.Engine); err != nil {
			return res, err
		}
	}

	// Enable `format: uuid` validation
	openapi3.DefineStringFormat("uuid", openapi3.FormatOfStringForUUIDOfRFC4122)

	if spec, err := oas.LoadSpec(); err != nil {
		return res, err
	} else if spec != nil {
		res.Engine.Use(middleware.WithOASValidation(
			spec,
			true,
			middleware.AllowUndefinedRoutes(
				middleware.DefaultOASErrorHandler,
			),
			nil,
		))
	}

	// TODO: wildcard route doesn't permit this to overlap with the `/oas` "fs"
	// TODO: this won't work properly behind a path-modifying reverse proxy as we
	// don't have any `servers` entries so it will guess the wrong base
	res.Engine.StaticFS("/oas-ui", http.FS(swaggerui.FS))
	configHandler := swaggerui.CustomConfigHandler(func(config map[string]interface{}) map[string]interface{} {
		config["urls"] = []map[string]interface{}{
			{"url": config["url"], "name": version.AppName},
			{"url": "../oas-grpc/pubsub.swagger.json", "name": "grpc:pubsub"},
			{"url": "../oas-grpc/schema.swagger.json", "name": "grpc:schema"},
		}
		delete(config, "url")
		return config
	})
	res.Engine.GET(swaggerui.ConfigLoadingPath, gin.WrapF(configHandler))
	// NOTE: this will serve yaml as text/plain. YAML doesn't have a standardized
	// mime type, so that's OK for now
	res.Engine.StaticFS("/oas", http.FS(oas.OpenAPIFS))

	// add standard debug routes
	res.Engine.GET("/debug/vars", ginexpvar.Handler())
	// use a wildcard route and defert to the default servemux, so that
	// we don't have to replicate the Index wildcard behavior ourselves
	res.Engine.Any("/debug/pprof/*profile", gin.WrapH(http.DefaultServeMux))
	res.Engine.StaticFS("/oas-grpc", http.FS(mbgrpc.SwaggerFS))

	for _, c := range params.Controllers {
		if err := c.Register(res.Engine); err != nil {
			return res, err
		}
	}

	res.HTTPServer = services.NewHTTPService(res.Engine, app.port, 0)
	params.L.Append(fx.StartStopHook(
		func(startCtx context.Context) error {
			ready := make(chan struct{})
			go func() {
				if err := res.HTTPServer.Start(params.Ctx, ready); err != nil {
					_ = params.SD.Shutdown(fx.ExitCode(1))
				}
			}()
			select {
			case <-startCtx.Done():
				return startCtx.Err()
			case <-ready:
				return nil
			}
		},
		func(stopCtx context.Context) error {
			return res.HTTPServer.Cleanup(stopCtx)
		},
	))

	return res, nil
}

type grpcParams struct {
	fx.In
	Ctx     context.Context
	EG      *errgroup.Group
	L       fx.Lifecycle
	Client  *ent.Client
	Engine  *gin.Engine
	Faults  *faults.Set
	Readies []services.ReadyCheck `group:"ready"`
}
type grpcResults struct {
	fx.Out
	GrpcService services.Service `group:"services"`
	// GrpcReady      services.ReadyCheck `group:"ready"`
	GatewayService services.Service `group:"services"`
	// GatewayReady   services.ReadyCheck `group:"ready"`
}

func (app *app) provideGrpc(params grpcParams) (grpcResults, error) {
	var results grpcResults
	results.GrpcService = mbgrpc.NewGrpcService(
		app.port, defaults.GRPCOffset,
		nil,
		params.Faults,
		func(_ context.Context, server *grpc.Server, client *ent.Client) error {
			return services.InitializeGrpcServers(server, client, params.Readies)
		},
	)
	if gsr, err := services.WrapService(params.Ctx, params.EG, results.GrpcService, params.L, params.Client); err != nil {
		return results, err
	} else {
		// results.GatewayReady = gsr.Ready
		_ = gsr.Ready
	}

	results.GatewayService = mbgrpc.NewGatewayService(
		version.AppName,
		app.port, defaults.GRPCOffset,
		params.Engine,
		[]string{"/v1/projects/*grpcPath", "/healthz"},
		services.BindGatewayHandlers,
	)
	if gsr, err := services.WrapService(
		params.Ctx, params.EG, results.GatewayService, params.L, params.Client,
	); err != nil {
		return results, err
	} else {
		// results.GatewayReady = gsr.Ready
		_ = gsr.Ready
	}

	return results, nil
}

func (a *app) main() (int, error) {
	app := fx.New(a.opts...)

	// adapted from fx.App.Run()

	startCtx, cancel := context.WithTimeout(context.Background(), app.StartTimeout())
	defer cancel()

	if err := app.Start(startCtx); err != nil {
		return 1, err
	}

	sig := <-app.Wait()
	exitCode := sig.ExitCode
	// app.log().LogEvent(&fxevent.Stopping{Signal: sig.Signal})

	stopCtx, cancel := context.WithTimeout(context.Background(), app.StopTimeout())
	defer cancel()

	if err := app.Stop(stopCtx); err != nil {
		return 1, err
	}

	return exitCode, nil
}
