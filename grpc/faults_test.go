package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	"go.6river.tech/mmmbbb/grpc/pubsub"
)

func unaryNoOp(context.Context, interface{}) (interface{}, error) {
	return nil, nil
}

func Benchmark_UnaryFaultInjection_empty_miss(b *testing.B) {
	ctx := context.Background()
	req := &pubsub.PullRequest{
		Subscription: "projects/development/subscriptions/foobarbazbat",
	}
	info := &grpc.UnaryServerInfo{
		FullMethod: "/google.pubsub.v1.Subscriber/Pull",
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if ret, err := UnaryFaultInjection(ctx, req, info, unaryNoOp); err != nil {
			b.Fatal("check of empty set returned error")
		} else if ret != nil {
			b.Fatal("impossible return from unary no-op")
		}
	}
}
