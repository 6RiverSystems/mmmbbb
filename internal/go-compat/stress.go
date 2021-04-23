package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx := context.Background()

	os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8802")
	psc, err := pubsub.NewClient(ctx, "go-compat")
	panicIf(err)
	id := "go-stress-" + uuid.NewString()
	t, err := psc.CreateTopic(ctx, id)
	panicIf(err)
	defer func() { panicIf(t.Delete(ctx)) }()
	s, err := psc.CreateSubscription(ctx, id, pubsub.SubscriptionConfig{
		Topic: t,
		RetryPolicy: &pubsub.RetryPolicy{
			MinimumBackoff: time.Second * 3 / 2,
		},
	})
	panicIf(err)
	defer func() { panicIf(s.Delete(ctx)) }()
	// configure the flow control same as our typical apps
	s.ReceiveSettings.MaxOutstandingMessages = 20

	const numMessages = 5000

	eg, egCtx := errgroup.WithContext(ctx)

	pubs := make(chan *pubsub.PublishResult, numMessages/10)
	start := time.Now()
	eg.Go(func() error {
		defer close(pubs)
		payload := json.RawMessage(`{"hello":"world"}`)
		for i := 0; i < numMessages; i++ {
			select {
			case <-egCtx.Done():
				fmt.Printf("canceled after queueing %d messages\n", i)
				return egCtx.Err()
			case pubs <- t.Publish(egCtx, &pubsub.Message{
				Data: payload,
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
		err := s.Receive(ctx, func(_ context.Context, m *pubsub.Message) {
			n := atomic.AddInt32(&numReceived, 1)
			m.Ack()
			if n >= numMessages {
				fmt.Println("all received")
				cancel()
			} else if n%1000 == 0 {
				fmt.Println("received", n, float64(time.Since(start))/float64(n)/1e6, "ms/msg")
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
