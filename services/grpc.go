package services

import (
	"context"
	"strings"

	"google.golang.org/grpc"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	entcommon "go.6river.tech/gosix/ent"
	"go.6river.tech/gosix/registry"

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
