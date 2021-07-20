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
	"math/rand"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx := context.Background()

	projectID := os.Getenv("PUBSUB_GCLOUD_PROJECT_ID")
	if projectID == "" {
		os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8802")
		projectID = "go-compat-deadletter"
	}
	psc, err := pubsub.NewClient(ctx, projectID)
	panicIf(err)
	uniq := uuid.NewString()
	primaryID := "go-deadletter-primary-" + uniq
	deadletterID := "go-deadletter-dead-" + uniq
	primaryTopic, err := psc.CreateTopic(ctx, primaryID)
	panicIf(err)
	defer func() { panicIf(primaryTopic.Delete(ctx)) }()
	deadletterTopic, err := psc.CreateTopic(ctx, deadletterID)
	panicIf(err)
	defer func() { panicIf(deadletterTopic.Delete(ctx)) }()

	primarySub, err := psc.CreateSubscription(ctx, primaryID, pubsub.SubscriptionConfig{
		Topic: primaryTopic,
		RetryPolicy: &pubsub.RetryPolicy{
			MinimumBackoff: time.Second * 3 / 2,
		},
		DeadLetterPolicy: &pubsub.DeadLetterPolicy{
			DeadLetterTopic: deadletterTopic.String(),
			// 5 is the minimum google allows, mmmbbb permits as low as 1
			MaxDeliveryAttempts: 5,
		},
	})
	panicIf(err)
	defer func() { panicIf(primarySub.Delete(ctx)) }()
	deadletterSub, err := psc.CreateSubscription(ctx, deadletterID, pubsub.SubscriptionConfig{
		Topic: deadletterTopic,
	})
	panicIf(err)
	defer func() { panicIf(deadletterSub.Delete(ctx)) }()
	primarySub.ReceiveSettings.MaxOutstandingMessages = 20
	deadletterSub.ReceiveSettings.MaxOutstandingMessages = 20

	const numMessages = 1000

	eg, egCtx := errgroup.WithContext(ctx)

	pubs := make(chan *pubsub.PublishResult, numMessages/10)
	start := time.Now()

	// tracking
	todoSet := make(map[int]bool, numMessages)
	todoMu := &sync.Mutex{}
	todo := func(i int) {
		todoMu.Lock()
		todoSet[i] = true
		todoMu.Unlock()
	}
	done := func(i int) {
		todoMu.Lock()
		todo, ok := todoSet[i]
		if !ok {
			panic("Completed item not in todo set")
		} else if !todo {
			panic("Completed item more than once")
		}
		todoSet[i] = false
		todoMu.Unlock()
	}
	allDone := func() bool {
		todoMu.Lock()
		defer todoMu.Unlock()
		if len(todoSet) < numMessages {
			return false
		}
		for _, todo := range todoSet {
			if todo {
				return false
			}
		}
		return true
	}

	// publishing
	eg.Go(func() error {
		defer close(pubs)
		payload := json.RawMessage(`{"hello":"world"}`)
		for i := 0; i < numMessages; i++ {
			todo(i)
			select {
			case <-egCtx.Done():
				fmt.Printf("canceled after queueing %d messages\n", i)
				return egCtx.Err()
			case pubs <- primaryTopic.Publish(egCtx, &pubsub.Message{
				Data: payload,
				Attributes: map[string]string{
					"i": strconv.Itoa(i),
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
			} else if numSent%(numMessages/10) == 0 {
				fmt.Println("sent", numSent)
			}
		}
		return nil
	})

	// primary subscriber
	const nackRate = 0.5
	var numAckedPrimary, numNackedPrimary int32
	var cancelPrimary, cancelDeadletter context.CancelFunc
	eg.Go(func() error {
		ctx, cancel := context.WithCancel(egCtx)
		defer cancel()
		cancelPrimary = cancel
		err := primarySub.Receive(ctx, func(_ context.Context, m *pubsub.Message) {
			if i, err := strconv.Atoi(m.Attributes["i"]); err != nil {
				fmt.Println("Bad Attributes.i:", err)
			} else if rand.Float64() < nackRate {
				m.Nack()
				n := atomic.AddInt32(&numNackedPrimary, 1)
				if n%(numMessages/10) == 0 {
					// timing data here isn't meaningful due to nacks
					fmt.Println("nacked primary", n, float64(time.Since(start))/float64(n)/1e6, "ms/msg")
				}
			} else {
				m.Ack()
				n := atomic.AddInt32(&numAckedPrimary, 1)
				done(i)
				if allDone() {
					fmt.Println("all received")
					cancel()
					if cancelDeadletter != nil {
						cancelDeadletter()
					}
				} else if n%(numMessages/10) == 0 {
					// timing data here isn't meaningful due to nacks
					fmt.Println("received primary", n, float64(time.Since(start))/float64(n)/1e6, "ms/msg")
				}
			}
		})
		if err != nil {
			fmt.Println("receive error:", err)
		}
		return err
	})

	// deadletter subscriber
	var numAckedDeadLetter int32
	eg.Go(func() error {
		ctx, cancel := context.WithCancel(egCtx)
		defer cancel()
		cancelDeadletter = cancel
		err := deadletterSub.Receive(ctx, func(_ context.Context, m *pubsub.Message) {
			if i, err := strconv.Atoi(m.Attributes["i"]); err != nil {
				fmt.Println("Bad Attributes.i:", err)
			} else {
				m.Ack()
				n := atomic.AddInt32(&numAckedDeadLetter, 1)
				done(i)
				if allDone() {
					fmt.Println("all received")
					cancel()
					if cancelPrimary != nil {
						cancelPrimary()
					}
				} else if n%(numMessages/10) == 0 {
					// timing data here isn't meaningful due to nacks
					fmt.Println("received deadletter", n, float64(time.Since(start))/float64(n)/1e6, "ms/msg")
				}
			}
		})
		if err != nil {
			fmt.Println("receive error:", err)
		}
		return err
	})

	// ...
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
