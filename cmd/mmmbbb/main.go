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
	"fmt"
	"net/http"
	"os"
	"runtime"

	"entgo.io/ent/dialect/sql"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"

	"go.6river.tech/gosix/app"
	_ "go.6river.tech/gosix/controllers"
	entcommon "go.6river.tech/gosix/ent"
	"go.6river.tech/gosix/migrate"
	"go.6river.tech/gosix/registry"
	swaggerui "go.6river.tech/gosix/swagger-ui"
	"go.6river.tech/mmmbbb/controllers"
	_ "go.6river.tech/mmmbbb/controllers"
	"go.6river.tech/mmmbbb/defaults"
	"go.6river.tech/mmmbbb/ent"
	mbgrpc "go.6river.tech/mmmbbb/grpc"
	"go.6river.tech/mmmbbb/migrations"
	"go.6river.tech/mmmbbb/oas"
	"go.6river.tech/mmmbbb/services"
	_ "go.6river.tech/mmmbbb/services"
	"go.6river.tech/mmmbbb/version"

	_ "go.6river.tech/mmmbbb/ent/runtime"
)

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

		fmt.Fprintln(os.Stderr, "mmmbbb does not accept command line arguments")
		os.Exit(1)
	}

	if err := NewApp().Main(); err != nil {
		panic(err)
	}
}

func NewApp() *app.App {
	app := &app.App{
		Name:    version.AppName,
		Version: version.SemrelVersion,
		Port:    defaults.Port,
		InitDbMigration: func(ctx context.Context, m *migrate.Migrator) error {
			if ownMigrations, err := migrate.LoadFS(migrations.MessageBusMigrations, nil); err != nil {
				return err
			} else {
				m.SortAndAppend("", ownMigrations...)
			}
			return nil
		},
		InitEnt: func(ctx context.Context, drv *sql.Driver, logger func(args ...interface{}), debug bool) (entcommon.EntClient, error) {
			opts := []ent.Option{
				ent.Driver(drv),
				ent.Log(logger),
			}
			if debug {
				opts = append(opts, ent.Debug())
			}
			return ent.NewClient(opts...), nil
		},
		LoadOASSpec: func(ctx context.Context) (*openapi3.T, error) {
			return oas.LoadSpec()
		},
		OASFS: http.FS(oas.OpenAPIFS),
		SwaggerUIConfigHandler: swaggerui.CustomConfigHandler(func(config map[string]interface{}) map[string]interface{} {
			config["urls"] = []map[string]interface{}{
				{"url": config["url"], "name": version.AppName},
				{"url": "../oas-grpc/pubsub.swagger.json", "name": "grpc:pubsub"},
				{"url": "../oas-grpc/schema.swagger.json", "name": "grpc:schema"},
			}
			delete(config, "url")
			return config
		}),
		RegisterServices: func(_ context.Context, r *registry.Registry, _ registry.MutableValues) error {
			services.RegisterDefaultServices(r)
			return nil
		},
		CustomizeRoutes: func(_ context.Context, e *gin.Engine, r *registry.Registry, _ registry.MutableValues) error {
			e.StaticFS("/oas-grpc", http.FS(mbgrpc.SwaggerFS))
			controllers.RegisterAll(r)
			return nil
		},
		Grpc: &app.AppGrpc{
			PortOffset:     defaults.GRPCOffset,
			Initializer:    services.InitializeGrpcServers,
			GatewayPaths:   []string{"/v1/projects/*grpcPath", "/healthz"},
			OnGatewayStart: services.BindGatewayHandlers,
		},
	}
	app.WithDefaults()
	return app
}
