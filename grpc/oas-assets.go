package grpc

import "embed"

//go:embed pubsub.swagger.json schema.swagger.json health.swagger.json
var SwaggerFS embed.FS
