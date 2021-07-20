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
	"errors"
	"strings"

	"google.golang.org/grpc"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	entcommon "go.6river.tech/gosix/ent"
	"go.6river.tech/gosix/registry"

	"go.6river.tech/mmmbbb/actions"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/grpc/health"
	"go.6river.tech/mmmbbb/grpc/pubsub"
)

func projectTopicPrefix(project string) string {
	return project + "/topics/"
}
func projectSubscriptionPrefix(project string) string {
	return project + "/subscriptions/"
}

func isValidTopicName(name string) bool {
	// match projects/.../topics/...
	segments := strings.Split(name, "/")
	return len(segments) == 4 &&
		segments[0] == "projects" &&
		segments[1] != "" &&
		segments[2] == "topics" &&
		segments[3] != ""
}

func isValidSubscriptionName(name string) bool {
	// match projects/.../subscriptions/...
	segments := strings.Split(name, "/")
	return len(segments) == 4 &&
		segments[0] == "projects" &&
		segments[1] != "" &&
		segments[2] == "subscriptions" &&
		segments[3] != ""
}

func InitializeGrpcServers(_ context.Context, server *grpc.Server, services *registry.Registry, client_ entcommon.EntClient) error {
	client := client_.(*ent.Client)
	pubsub.RegisterPublisherServer(server, &publisherServer{client: client})
	pubsub.RegisterSubscriberServer(server, &subscriberServer{client: client})
	health.RegisterHealthServer(server, &healthServer{client: client, services: services})
	return nil
}

func BindGatewayHandlers(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
	if err := pubsub.RegisterSubscriberHandler(ctx, mux, conn); err != nil {
		return err
	}
	if err := pubsub.RegisterPublisherHandler(ctx, mux, conn); err != nil {
		return err
	}
	return nil
}

func isNotFound(err error) bool {
	return ent.IsNotFound(err) || errors.Is(err, actions.ErrNotFound)
}
