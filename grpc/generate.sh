#!/bin/bash

set -xeuo pipefail

rm -rf googleapis.tgz grpc-proto.tgz google/ grpc/ *.pb*.go *-types.go

curl -s --location --fail -o googleapis.tgz \
	https://github.com/googleapis/googleapis/archive/ea947de5d1fca3a75723d993815827e835aed7cf.tar.gz
curl -s --location --fail -o grpc-proto.tgz \
	https://github.com/grpc/grpc-proto/archive/master.tar.gz

tar -zxf googleapis.tgz --strip-components=1 googleapis-ea947de5d1fca3a75723d993815827e835aed7cf/google/
tar -zxf grpc-proto.tgz --strip-components=1 grpc-proto-master/grpc/

protoc \
	--go_out=. \
	--go-grpc_out=. \
	--grpc-gateway_out=. \
	--openapiv2_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_opt=paths=source_relative \
	--grpc-gateway_opt paths=source_relative \
	--experimental_allow_proto3_optional \
	google/pubsub/v1/pubsub.proto \
	google/pubsub/v1/schema.proto

mkdir -p pubsubpb
cp -a -v -f google/pubsub/v1/pubsub_grpc.pb.go google/pubsub/v1/pubsub.pb.gw.go pubsubpb/
cp -a -v -f google/pubsub/v1/schema_grpc.pb.go google/pubsub/v1/schema.pb.gw.go pubsubpb/

# we can use upstream types/codegen for the health service, and it has no HTTP
# swagger types

cp -a -v -f google/pubsub/v1/pubsub.swagger.json google/pubsub/v1/schema.swagger.json .

./gen-types.sh pubsubpb cloud.google.com/go/pubsub/apiv1/pubsubpb google/pubsub/v1/pubsub.pb.go pubsubpb/pubsub-types.go
./gen-types.sh pubsubpb cloud.google.com/go/pubsub/apiv1/pubsubpb google/pubsub/v1/schema.pb.go pubsubpb/schema-types.go

rm -rf google/ grpc/ googleapis.tgz grpc-proto.tgz
