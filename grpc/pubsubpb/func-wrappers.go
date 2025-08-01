package pubsubpb

import (
	upstream "cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"google.golang.org/grpc"
)

// hand written "re-exports" of functions

func NewPublisherClient(cc grpc.ClientConnInterface) PublisherClient {
	return upstream.NewPublisherClient(cc)
}

func NewSubscriberClient(cc grpc.ClientConnInterface) SubscriberClient {
	return upstream.NewSubscriberClient(cc)
}

func NewSchemaServiceClient(cc grpc.ClientConnInterface) SchemaServiceClient {
	return upstream.NewSchemaServiceClient(cc)
}

func RegisterPublisherServer(s *grpc.Server, srv PublisherServer) {
	upstream.RegisterPublisherServer(s, srv)
}

func RegisterSubscriberServer(s *grpc.Server, srv SubscriberServer) {
	upstream.RegisterSubscriberServer(s, srv)
}
