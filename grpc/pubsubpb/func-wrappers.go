// Copyright (c) 2025 6 River Systems
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
