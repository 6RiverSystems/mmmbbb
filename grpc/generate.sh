#!/bin/bash
# Copyright (c) 2024 6 River Systems
#
# Permission is hereby granted, free of charge, to any person obtaining a copy of
# this software and associated documentation files (the "Software"), to deal in
# the Software without restriction, including without limitation the rights to
# use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
# the Software, and to permit persons to whom the Software is furnished to do so,
# subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
# FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
# COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
# IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
# CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.


set -xeuo pipefail

td=$(mktemp -d)
cleanup() {
	rm -rf "$td"
}
trap cleanup EXIT

rm -vf pubsubpb/*{.pb.go,.pb.gw.go,-types.go} *.swagger.json

pushd "$td"
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

popd # back to source dir

mkdir -p pubsubpb
cp -avf \
	"${td}/google/pubsub/v1/pubsub_grpc.pb.go" \
	"${td}/google/pubsub/v1/pubsub.pb.gw.go" \
	"${td}/google/pubsub/v1/schema_grpc.pb.go" \
	"${td}/google/pubsub/v1/schema.pb.gw.go" \
	pubsubpb/

# we can use upstream types/codegen for the health service, and it has no HTTP
# swagger types

cp -avf "${td}/google/pubsub/v1/pubsub.swagger.json" "${td}/google/pubsub/v1/schema.swagger.json" .

./gen-types.sh pubsubpb cloud.google.com/go/pubsub/v2/apiv1/pubsubpb "${td}/google/pubsub/v1/pubsub.pb.go" pubsubpb/pubsub-types.go
./gen-types.sh pubsubpb cloud.google.com/go/pubsub/v2/apiv1/pubsubpb "${td}/google/pubsub/v1/schema.pb.go" pubsubpb/schema-types.go
