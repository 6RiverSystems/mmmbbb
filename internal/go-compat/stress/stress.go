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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.6river.tech/mmmbbb/grpc/pubsubpb"
	"go.6river.tech/mmmbbb/internal"
)

func main() {
	ctx := context.Background()

	orderSplit := 0
	reqOrderSplit := os.Getenv("ORDERED_SPLIT")
	if reqOrderSplit != "" {
		var err error
		orderSplit, err = strconv.Atoi(reqOrderSplit)
		panicIf(err)
	}

	var pscOpts []option.ClientOption
	if os.Getenv("CONCURRENT_EMULATORS") != "" {
		// using the emulator forces the library to use just one connection, which
		// sucks for stress testing
		pscOpts = append(pscOpts,
			option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
			option.WithTelemetryDisabled(),
			option.WithEndpoint("localhost:8802"),
			option.WithoutAuthentication(),
		)
	} else {
		os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8802")
	}
	const projectID = "go-compat-stress"
	psc, err := pubsub.NewClient(ctx, projectID, pscOpts...)
	panicIf(err)
	_id := "go-stress-" + uuid.NewString()
	tac := psc.TopicAdminClient
	tm, err := tac.CreateTopic(ctx, &pubsubpb.Topic{Name: internal.PSTopicName(projectID, _id)})
	panicIf(err)
	defer func() { panicIf(tac.DeleteTopic(ctx, &pubsubpb.DeleteTopicRequest{Topic: tm.Name})) }()
	t := psc.Publisher(tm.Name)
	t.EnableMessageOrdering = orderSplit != 0
	sac := psc.SubscriptionAdminClient
	sm, err := sac.CreateSubscription(ctx, &pubsubpb.Subscription{
		Name:                  internal.PSSubName(projectID, _id),
		Topic:                 tm.Name,
		EnableMessageOrdering: orderSplit != 0,
		RetryPolicy: &pubsubpb.RetryPolicy{
			MinimumBackoff: durationpb.New(time.Second * 3 / 2),
		},
	})
	panicIf(err)
	defer func() {
		panicIf(sac.DeleteSubscription(ctx, &pubsubpb.DeleteSubscriptionRequest{Subscription: sm.Name}))
	}()
	s := psc.Subscriber(sm.Name)
	// configure the flow control same as our typical apps
	s.ReceiveSettings.MaxOutstandingMessages = 20
	// scaling up from the default of 10 can make it go faster if the emulator can
	// keep up, but you need to balance cpu usage by postgres, the server, and
	// this client.
	// s.ReceiveSettings.NumGoroutines = 10

	const numMessages = 5000

	eg, egCtx := errgroup.WithContext(ctx)

	pubs := make(chan *pubsub.PublishResult, numMessages/10)
	start := time.Now()
	eg.Go(func() error {
		defer close(pubs)
		payload := json.RawMessage(`{"hello":"world"}`)
		for i := range numMessages {
			orderKey := ""
			if orderSplit != 0 {
				orderKey = strconv.Itoa(i % orderSplit)
			}
			select {
			case <-egCtx.Done():
				fmt.Printf("canceled after queueing %d messages\n", i)
				return egCtx.Err()
			case pubs <- t.Publish(egCtx, &pubsub.Message{
				Data:        payload,
				OrderingKey: orderKey,
				Attributes: map[string]string{
					// send i+1 so that we can treat 0 as nothing received in the
					// subscriber checks
					"i": strconv.Itoa(i + 1),
				},
			}):
			}
		}
		return nil
	})
	eg.Go(func() error {
		numSent := 0
		for p := range pubs {
			_, err := p.Get(egCtx)
			if err != nil {
				fmt.Println("error publishing:", err)
				return err
			}
			numSent++
			if numSent >= numMessages {
				fmt.Println("all sent")
			} else if numSent%1000 == 0 {
				fmt.Println("sent", numSent)
			}
		}
		return nil
	})
	eg.Go(func() error {
		numReceived := int32(0)
		ctx, cancel := context.WithCancel(egCtx)
		defer cancel()
		lastReceived := make([]int, orderSplit)
		lastReceivedMu := make([]sync.Mutex, orderSplit)
		err := s.Receive(ctx, func(_ context.Context, m *pubsub.Message) {
			n := atomic.AddInt32(&numReceived, 1)
			m.Ack()
			if n >= numMessages {
				fmt.Println("all received")
				cancel()
			} else if n%1000 == 0 {
				fmt.Println("received", n, float64(time.Since(start))/float64(n)/1e6, "ms/msg")
			}
			if orderSplit != 0 {
				if k, err := strconv.Atoi(m.OrderingKey); err != nil {
					fmt.Println("Bad OrderingKey:", err)
				} else if i, err := strconv.Atoi(m.Attributes["i"]); err != nil {
					fmt.Println("Bad Attributes.i:", err)
				} else {
					lastReceivedMu[k].Lock()
					defer lastReceivedMu[k].Unlock()
					if i <= lastReceived[k] {
						fmt.Println("Out of Order:", k, lastReceived[k], i)
					} else {
						lastReceived[k] = i
					}
				}
			}
		})
		if err != nil {
			fmt.Println("receive error:", err)
		}
		return err
	})
	panicIf(eg.Wait())
	duration := time.Since(start)
	fmt.Println("took", duration)
	fmt.Println(float64(duration)/numMessages/1e6, "ms/msg")
}

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}
