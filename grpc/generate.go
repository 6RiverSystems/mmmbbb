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

// this isn't part of the actual code build
// +build generate

package grpc

import (
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)

//go:generate sh -c "rm -rf googleapis.tgz grpc-proto.tgz google/ grpc/ *.pb*.go *-types.go"

//go:generate curl -s --location --fail -o googleapis.tgz https://github.com/googleapis/googleapis/archive/master.tar.gz
//go:generate curl -s --location --fail -o grpc-proto.tgz https://github.com/grpc/grpc-proto/archive/master.tar.gz

//go:generate tar -zxf googleapis.tgz --strip-components=1 googleapis-master/google/
//go:generate tar -zxf grpc-proto.tgz --strip-components=1 grpc-proto-master/grpc/

//go:generate protoc --go_out=. --go-grpc_out=. --grpc-gateway_out=. --openapiv2_out=. --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative --grpc-gateway_opt paths=source_relative google/pubsub/v1/pubsub.proto google/pubsub/v1/schema.proto grpc/health/v1/health.proto

//go:generate mkdir -p pubsub health
//go:generate cp -a -v -f google/pubsub/v1/pubsub_grpc.pb.go google/pubsub/v1/pubsub.pb.gw.go pubsub/
//go:generate cp -a -v -f google/pubsub/v1/schema_grpc.pb.go google/pubsub/v1/schema.pb.gw.go pubsub/

// health has no http gateway defined, and needs to use its own types as there is no "upstream" from which to source them
//go:generate cp -a -v -f grpc/health/v1/health_grpc.pb.go grpc/health/v1/health.pb.go health/
//go:generate cp -a -v -f google/pubsub/v1/pubsub.swagger.json google/pubsub/v1/schema.swagger.json grpc/health/v1/health.swagger.json .
// it also needs its package header fixed
//go:generate sed -i "s/^package grpc_health_v1$/package health/" health/health_grpc.pb.go health/health.pb.go

//go:generate ./gen-types.sh pubsub google.golang.org/genproto/googleapis/pubsub/v1 google/pubsub/v1/pubsub.pb.go pubsub/pubsub-types.go
//go:generate ./gen-types.sh pubsub google.golang.org/genproto/googleapis/pubsub/v1 google/pubsub/v1/schema.pb.go pubsub/schema-types.go

//go:generate rm -rf google/ grpc/ googleapis.tgz grpc-proto.tgz
