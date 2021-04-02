package main

import (
	"context"
	"net/http"

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
	"go.6river.tech/mmmbbb/grpc"
	"go.6river.tech/mmmbbb/migrations"
	"go.6river.tech/mmmbbb/oas"
	"go.6river.tech/mmmbbb/services"
	_ "go.6river.tech/mmmbbb/services"
	"go.6river.tech/mmmbbb/version"

	_ "github.com/jackc/pgx/v4"
	_ "github.com/mattn/go-sqlite3"

	_ "go.6river.tech/mmmbbb/ent/runtime"
)

func main() {
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
		LoadOASSpec: func(ctx context.Context) (*openapi3.Swagger, error) {
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
			e.StaticFS("/oas-grpc", http.FS(grpc.SwaggerFS))
			controllers.RegisterAll(r)
			return nil
		},
		Grpc: &app.AppGrpc{
			PortOffset:     defaults.GRPCOffset,
			Initializer:    services.InitializeGrpcServers,
			GatewayPaths:   []string{"/v1/projects/*grpcPath"},
			OnGatewayStart: services.BindGatewayHandlers,
		},
	}
	app.WithDefaults()
	return app
}
