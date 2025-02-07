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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.6river.tech/mmmbbb/actions"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/ent/topic"
	"go.6river.tech/mmmbbb/internal/testutil"
)

// TestGrpcCompat is an acceptance-style test for checking the gRPC
// implementation plays nice with Google's client
func TestGrpcCompat(t *testing.T) {
	client, psClient := initGrpcTest(t)

	type test struct {
		name   string
		before func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client)
		// steps are run _concurrently_
		steps []func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client)
		after func(t *testing.T, ctx context.Context, client *ent.Client, tt *test)
		// tests can fill these in to share state among before/step/concurrent/after
		topics  []*pubsub.Topic
		subs    []*pubsub.Subscription
		servers []*httptest.Server
		errs    []error
		waiters []chan struct{}
		mu      sync.Mutex
	}
	type testStep = func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client)

	tests := []*test{
		{
			name: "concurrent topic create",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				tt.topics = make([]*pubsub.Topic, 2)
				tt.errs = make([]error, 2)
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					tt.topics[0], tt.errs[0] = psc.CreateTopic(ctx, safeName(t))
					if tt.errs[0] != nil {
						assert.Equal(
							t,
							codes.AlreadyExists.String(),
							status.Code(tt.errs[0]).String(),
							"should be code AlreadyExists: %#v",
							tt.errs[0],
						)
					}
				},
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					tt.topics[1], tt.errs[1] = psc.CreateTopic(ctx, safeName(t))
					if tt.errs[1] != nil {
						assert.Equal(
							t,
							codes.AlreadyExists.String(),
							status.Code(tt.errs[1]).String(),
							"should be code AlreadyExists: %#v",
							tt.errs[1],
						)
					}
				},
			},
			after: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				successes, fails := 0, 0
				for i, err := range tt.errs {
					if err != nil && tt.topics[i] == nil {
						fails++
					} else if err == nil && tt.topics[i] != nil {
						successes++
					} else {
						assert.Fail(t, "hybrid success fail", "call %d returned both topic and error", i)
					}
				}
				assert.Equal(t, 1, successes)
				assert.Equal(t, 1, fails)
			},
		},
		{
			name: "concurrent sub create",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				tt.topics = make([]*pubsub.Topic, 1)
				var err error
				tt.topics[0], err = psc.CreateTopic(ctx, safeName(t))
				require.NoError(t, err)
				tt.errs = make([]error, 2)
				tt.subs = make([]*pubsub.Subscription, 2)
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					tt.subs[0], tt.errs[0] = psc.CreateSubscription(
						ctx,
						safeName(t),
						pubsub.SubscriptionConfig{Topic: tt.topics[0]},
					)
					if tt.errs[0] != nil {
						assert.Equal(
							t,
							codes.AlreadyExists.String(),
							status.Code(tt.errs[0]).String(),
							"should be code AlreadyExists: %#v",
							tt.errs[0],
						)
					}
				},
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					tt.subs[1], tt.errs[1] = psc.CreateSubscription(
						ctx,
						safeName(t),
						pubsub.SubscriptionConfig{Topic: tt.topics[0]},
					)
					if tt.errs[1] != nil {
						assert.Equal(
							t,
							codes.AlreadyExists.String(),
							status.Code(tt.errs[1]).String(),
							"should be code AlreadyExists: %#v",
							tt.errs[1],
						)
					}
				},
			},
			after: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				successes, fails := 0, 0
				for i, err := range tt.errs {
					if err != nil && tt.subs[i] == nil {
						fails++
					} else if err == nil && tt.subs[i] != nil {
						successes++
					} else {
						assert.Fail(t, "hybrid success fail", "call %d returned both topic and error", i)
					}
				}
				assert.Equal(t, 1, successes)
				assert.Equal(t, 1, fails)
			},
		},
		{
			name: "list topics",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				// need to purge topics from prior tests
				assert.NoError(t, client.Topic.Update().
					Where(topic.DeletedAtIsNil()).
					SetDeletedAt(time.Now()).
					ClearLive().
					Exec(ctx))
				const numTopics = 5
				tt.topics = make([]*pubsub.Topic, numTopics)
				for i := range tt.topics {
					var err error
					tt.topics[i], err = psc.CreateTopic(ctx, safeName(t)+"_"+strconv.Itoa(i))
					require.NoError(t, err)
				}
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					expected := make([]string, len(tt.topics))
					for i, t := range tt.topics {
						expected[i] = t.ID()
					}
					ti := psc.Topics(ctx)
					var actual []string
					for topic, err := ti.Next(); ; topic, err = ti.Next() {
						if err == iterator.Done {
							assert.Nil(t, topic)
							break
						}
						assert.NoError(t, err)
						require.NotNil(t, topic)
						actual = append(actual, topic.ID())
					}
					assert.ElementsMatch(t, expected, actual)
				},
			},
		},
		{
			name: "list subs on one topic",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				const numSubs = 5
				tt.topics = make([]*pubsub.Topic, 1)
				tt.subs = make([]*pubsub.Subscription, numSubs)
				var err error
				tt.topics[0], err = psc.CreateTopic(ctx, safeName(t))
				require.NoError(t, err)
				for i := range tt.subs {
					tt.subs[i], err = psc.CreateSubscription(
						ctx,
						safeName(t)+"_"+strconv.Itoa(i),
						pubsub.SubscriptionConfig{Topic: tt.topics[0]},
					)
					require.NoError(t, err)
				}
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					expected := make([]string, len(tt.subs))
					for i, s := range tt.subs {
						expected[i] = s.ID()
					}
					si := tt.topics[0].Subscriptions(ctx)
					var actual []string
					for sub, err := si.Next(); ; sub, err = si.Next() {
						if err == iterator.Done {
							assert.Nil(t, sub)
							break
						}
						assert.NoError(t, err)
						require.NotNil(t, sub)
						actual = append(actual, sub.ID())
					}
					assert.ElementsMatch(t, expected, actual)
				},
			},
		},
		{
			name: "list subs on all topics",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				// need to purge topics and subs from prior tests
				assert.NoError(t, client.Topic.Update().
					Where(topic.DeletedAtIsNil()).
					SetDeletedAt(time.Now()).
					ClearLive().
					Exec(ctx))
				assert.NoError(t, client.Subscription.Update().
					Where(subscription.DeletedAtIsNil()).
					SetDeletedAt(time.Now()).
					ClearLive().
					Exec(ctx))
				const numTopics = 2
				const numSubs = 6
				tt.topics = make([]*pubsub.Topic, numTopics)
				tt.subs = make([]*pubsub.Subscription, numSubs)
				var err error
				for i := range tt.topics {
					var err error
					tt.topics[i], err = psc.CreateTopic(ctx, safeName(t)+"_"+strconv.Itoa(i))
					require.NoError(t, err)
				}
				for i := range tt.subs {
					tt.subs[i], err = psc.CreateSubscription(
						ctx,
						safeName(t)+"_"+strconv.Itoa(i),
						pubsub.SubscriptionConfig{Topic: tt.topics[i%numTopics]},
					)
					require.NoError(t, err)
				}
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					expected := make([]string, len(tt.subs))
					for i, s := range tt.subs {
						expected[i] = s.ID()
					}
					si := psc.Subscriptions(ctx)
					var actual []string
					for sub, err := si.Next(); ; sub, err = si.Next() {
						if err == iterator.Done {
							assert.Nil(t, sub)
							break
						}
						assert.NoError(t, err)
						require.NotNil(t, sub)
						actual = append(actual, sub.ID())
					}
					assert.ElementsMatch(t, expected, actual)
				},
			},
		},
		{
			// TODO: this isn't a very "unity" test
			name: "topic lifecycle",
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					// Exists maps to GetTopic
					exists, err := psc.Topic(safeName(t)).Exists(ctx)
					require.NoError(t, err)
					require.False(t, exists)

					topic, err := psc.CreateTopic(ctx, safeName(t))
					require.NoError(t, err)

					cfg, err := topic.Config(ctx)
					require.NoError(t, err)
					assert.Empty(t, cfg.KMSKeyName)
					assert.Empty(t, cfg.Labels)
					assert.Empty(t, cfg.MessageStoragePolicy.AllowedPersistenceRegions)

					cfg, err = topic.Update(ctx, pubsub.TopicConfigToUpdate{
						Labels: map[string]string{
							"forTest": t.Name(),
						},
					})
					require.NoError(t, err)
					assert.Empty(t, cfg.KMSKeyName)
					assert.Equal(t, map[string]string{"forTest": t.Name()}, cfg.Labels)
					assert.Empty(t, cfg.MessageStoragePolicy.AllowedPersistenceRegions)

					exists, err = topic.Exists(ctx)
					require.NoError(t, err)
					require.True(t, exists)

					err = topic.Delete(ctx)
					require.NoError(t, err)

					exists, err = topic.Exists(ctx)
					require.NoError(t, err)
					require.False(t, exists)
				},
			},
		},
		{
			// TODO: this isn't a very "unity" test
			name: "subscription lifecycle",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				topic, err := psc.CreateTopic(ctx, safeName(t))
				require.NoError(t, err)
				tt.topics = []*pubsub.Topic{topic}
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					// Exists maps to GetTopic
					exists, err := psc.Subscription(safeName(t)).Exists(ctx)
					require.NoError(t, err)
					require.False(t, exists)

					sub, err := psc.CreateSubscription(ctx, safeName(t), pubsub.SubscriptionConfig{Topic: tt.topics[0]})
					require.NoError(t, err)

					cfg, err := sub.Config(ctx)
					require.NoError(t, err)
					// cfg.AckDeadline // is some default value here
					assert.Nil(t, cfg.DeadLetterPolicy)
					assert.False(t, cfg.Detached)
					assert.False(t, cfg.EnableMessageOrdering)
					// cfg.ExpirationPolicy // is some default value here
					assert.Empty(t, cfg.Filter)
					assert.Empty(t, cfg.Labels)
					assert.Zero(t, cfg.PushConfig)
					assert.False(t, cfg.RetainAckedMessages)
					// cfg.RetentionDuration // is some default value here
					assert.Nil(t, cfg.RetryPolicy)
					assert.Equal(t, cfg.Topic.ID(), tt.topics[0].ID())

					cfg, err = sub.Update(ctx, pubsub.SubscriptionConfigToUpdate{
						ExpirationPolicy: 42 * time.Hour,
						Labels: map[string]string{
							"forTest": t.Name(),
						},
						// can't change EnableMessageOrdering on the fly with the google
						// client, even though our code supports it
						RetentionDuration: time.Hour,
						RetryPolicy: &pubsub.RetryPolicy{
							MinimumBackoff: 4200 * time.Millisecond,
							MaximumBackoff: 42420 * time.Millisecond,
						},
					})
					require.NoError(t, err)
					// cfg.AckDeadline // we don't allow setting this
					assert.Nil(t, cfg.DeadLetterPolicy)
					assert.False(t, cfg.Detached)
					assert.False(t, cfg.EnableMessageOrdering)
					assert.Equal(t, 42*time.Hour, cfg.ExpirationPolicy)
					assert.Empty(t, cfg.Filter)
					assert.Equal(t, map[string]string{"forTest": t.Name()}, cfg.Labels)
					assert.Zero(t, cfg.PushConfig)
					assert.False(t, cfg.RetainAckedMessages)
					assert.Equal(t, time.Hour, cfg.RetentionDuration)
					assert.Equal(t, &pubsub.RetryPolicy{
						MinimumBackoff: 4200 * time.Millisecond,
						MaximumBackoff: 42420 * time.Millisecond,
					}, cfg.RetryPolicy)
					assert.Equal(t, cfg.Topic.ID(), tt.topics[0].ID())

					exists, err = sub.Exists(ctx)
					require.NoError(t, err)
					require.True(t, exists)

					err = sub.Delete(ctx)
					require.NoError(t, err)

					exists, err = sub.Exists(ctx)
					require.NoError(t, err)
					require.False(t, exists)
				},
			},
		},
	}
	const pullTimeScale = 100 * time.Millisecond
	tests = append(tests, []*test{
		{
			name: "streaming pull",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				topic, err := psc.CreateTopic(ctx, safeName(t))
				require.NoError(t, err)
				sub, err := psc.CreateSubscription(ctx, safeName(t), pubsub.SubscriptionConfig{
					Topic: topic,
					RetryPolicy: &pubsub.RetryPolicy{
						MinimumBackoff: pullTimeScale * 5 / 4,
					},
				})
				require.NoError(t, err)
				// avoid wasting a lot of time spinning up extra gRPC connections
				sub.ReceiveSettings.NumGoroutines = 1
				tt.topics = []*pubsub.Topic{topic}
				tt.subs = []*pubsub.Subscription{sub}
			},
			steps: []testStep{
				// sender
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					id, err := tt.topics[0].Publish(ctx, &pubsub.Message{
						// bare numbers are valid json values
						Data: json.RawMessage(`1`),
					}).Get(ctx)
					assert.NoError(t, err)
					assert.NotEmpty(t, id)
					// t.Logf("published 1 as %q", id)
					// pause so the subscriber gets two separate "pulls"
					time.Sleep(pullTimeScale * 3 / 2)
					id, err = tt.topics[0].Publish(ctx, &pubsub.Message{
						Data: json.RawMessage(`2`),
					}).Get(ctx)
					assert.NoError(t, err)
					assert.NotEmpty(t, id)
					// t.Logf("published 2 as %q", id)
				},
				// receiver
				// TODO: this is slow because of sleeps needed to trigger modack behavior
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					i := int32(0)
					rCtx, cancel := context.WithCancel(ctx)
					assert.NoError(t, tt.subs[0].Receive(rCtx, func(ctx context.Context, m *pubsub.Message) {
						counter := atomic.AddInt32(&i, 1)
						// t.Logf("received %q as %q at %d", string(m.Data), m.ID, counter)
						expected := counter
						if expected > 2 {
							expected = 2
						}
						assert.Equal(t, strconv.Itoa(int(expected)), string(m.Data),
							"should get expected message (in order)")
						if counter == 1 {
							// go fast
							defer m.Ack()
						} else if counter == 2 {
							// delay and nack
							defer m.Nack()
							time.Sleep(pullTimeScale)
						} else if counter == 3 {
							// delay and ack and done
							defer m.Ack()
							time.Sleep(pullTimeScale * 3 / 2)
							cancel()
						} else {
							assert.LessOrEqual(t, expected, int32(3))
						}
						// t.Logf("done with %q at %d", m.ID, counter)
					}))
				},
			},
		},
		{
			name: "synchronous pull",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				topic, err := psc.CreateTopic(ctx, safeName(t))
				require.NoError(t, err)
				sub, err := psc.CreateSubscription(ctx, safeName(t), pubsub.SubscriptionConfig{
					Topic: topic,
					RetryPolicy: &pubsub.RetryPolicy{
						MinimumBackoff: pullTimeScale * 5 / 4,
					},
				})
				require.NoError(t, err)
				// this should make it use the non-streaming pull endpoint
				sub.ReceiveSettings.Synchronous = true
				// avoid wasting a lot of time spinning up extra gRPC connections
				sub.ReceiveSettings.NumGoroutines = 1
				tt.topics = []*pubsub.Topic{topic}
				tt.subs = []*pubsub.Subscription{sub}
			},
			steps: []testStep{
				// sender
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					id, err := tt.topics[0].Publish(ctx, &pubsub.Message{
						// bare numbers are valid json values
						Data: json.RawMessage(`1`),
					}).Get(ctx)
					assert.NoError(t, err)
					assert.NotEmpty(t, id)
					// pause so the subscriber gets two separate "pulls"
					time.Sleep(pullTimeScale * 3 / 2)
					id, err = tt.topics[0].Publish(ctx, &pubsub.Message{
						Data: json.RawMessage(`2`),
					}).Get(ctx)
					assert.NoError(t, err)
					assert.NotEmpty(t, id)
				},
				// receiver
				// TODO: this is slow because of sleeps needed to trigger modack behavior
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					i := int32(0)
					rCtx, cancel := context.WithCancel(ctx)
					err := tt.subs[0].Receive(rCtx, func(ctx context.Context, m *pubsub.Message) {
						counter := atomic.AddInt32(&i, 1)
						expected := counter
						if expected > 2 {
							expected = 2
						}
						assert.Equal(t, ([]byte)(strconv.Itoa(int(expected))), m.Data)
						if counter == 1 {
							// go fast
							defer m.Ack()
						} else if counter == 2 {
							// delay and nack
							defer m.Nack()
							time.Sleep(pullTimeScale)
						} else if counter == 3 {
							// delay and ack and done
							defer m.Ack()
							time.Sleep(pullTimeScale * 3 / 2)
							cancel()
						} else {
							assert.LessOrEqual(t, expected, int32(3))
						}
					})
					assert.NoError(t, err)
				},
			},
		},
	}...)
	tests = append(tests, []*test{
		{
			name: "subscription http push",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// test step will replace this with the real listener before starting the server
					panic("should never be called")
				}))
				// s.URL won't be populated until we start the server, but we can find
				// it out from the Listener, which is already created
				serverURL := "http://" + srv.Listener.Addr().String()
				tt.servers = append(tt.servers, srv)
				t.Cleanup(srv.Close)

				pusher := &httpPusher{}
				err := pusher.Initialize(ctx, client)
				require.NoError(t, err)
				pusherReady := make(chan struct{})
				// waiter 0 is pusher end request, waiter 1 is pusher done notify
				tt.waiters = append(tt.waiters, make(chan struct{}), make(chan struct{}))
				// TODO: this needs to be part of the errgroup
				go func() {
					defer close(tt.waiters[1])
					ctx, cancel := context.WithCancel(ctx)
					defer cancel()
					// cancel on request
					go func() {
						defer cancel()
						<-tt.waiters[0]
					}()
					if err := pusher.Start(ctx, pusherReady); err != nil {
						tt.errs = append(tt.errs, err)
					}
				}()
				<-pusherReady
				t.Cleanup(func() {
					tt.mu.Lock()
					defer tt.mu.Unlock()
					_ = pusher.Cleanup(ctx)
				})

				topic, err := psc.CreateTopic(ctx, safeName(t))
				require.NoError(t, err)
				sub, err := psc.CreateSubscription(ctx, safeName(t), pubsub.SubscriptionConfig{
					Topic: topic,
					RetryPolicy: &pubsub.RetryPolicy{
						MinimumBackoff: 51 * time.Millisecond,
					},
					PushConfig: pubsub.PushConfig{
						Endpoint: serverURL,
					},
				})
				require.NoError(t, err)
				tt.topics = []*pubsub.Topic{topic}
				tt.subs = []*pubsub.Subscription{sub}
			},
			steps: []testStep{
				// sender
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					id, err := tt.topics[0].Publish(ctx, &pubsub.Message{
						// bare numbers are valid json values
						Data: json.RawMessage(`1`),
					}).Get(ctx)
					assert.NoError(t, err)
					assert.NotEmpty(t, id)
					// pause so the subscriber gets two separate "pulls"
					time.Sleep(100 * time.Millisecond)
					id, err = tt.topics[0].Publish(ctx, &pubsub.Message{
						Data: json.RawMessage(`2`),
					}).Get(ctx)
					assert.NoError(t, err)
					assert.NotEmpty(t, id)
				},
				// receiver
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					i := int32(0)
					rCtx, cancel := context.WithCancel(ctx)
					tt.servers[0].Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						defer r.Body.Close()
						counter := atomic.AddInt32(&i, 1)
						expected := counter
						if expected > 2 {
							expected = 2
						}
						var m actions.PushRequest
						dec := json.NewDecoder(r.Body)
						assert.NoError(t, dec.Decode(&m))
						decoded, err := base64.StdEncoding.DecodeString(m.Message.Data)
						assert.NoError(t, err, "message data should be base64 decodable")
						assert.Equal(t, strconv.Itoa(int(expected)), string(decoded))
						if counter == 1 {
							// go fast
							w.WriteHeader(http.StatusNoContent)
						} else if counter == 2 {
							// delay and nack
							time.Sleep(50 * time.Millisecond)
							w.WriteHeader(http.StatusInternalServerError)
						} else if counter == 3 {
							// delay and ack and done
							time.Sleep(150 * time.Millisecond)
							w.WriteHeader(http.StatusNoContent)
							cancel()
						} else {
							assert.LessOrEqual(t, expected, int32(3))
							// above will always fail
							w.WriteHeader(http.StatusInternalServerError)
						}
					})
					tt.servers[0].Start()
					<-rCtx.Done()
				},
			},
			after: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				// request pusher stop
				close(tt.waiters[0])
				// wait for pusher to end
				<-tt.waiters[1]
				tt.servers[0].Close()
			},
		},
		{
			name: "subscription filter",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				topic, err := psc.CreateTopic(ctx, safeName(t))
				require.NoError(t, err)
				topic.EnableMessageOrdering = true
				tt.topics = []*pubsub.Topic{topic}
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					sub, err := psc.CreateSubscription(ctx, safeName(t), pubsub.SubscriptionConfig{
						Topic:                 tt.topics[0],
						Filter:                "attributes:deliver",
						EnableMessageOrdering: true,
					})
					require.NoError(t, err)
					tt.topics[0].Publish(ctx, &pubsub.Message{
						Data:        json.RawMessage(`1`),
						OrderingKey: "x",
						Attributes:  map[string]string{"deliver": ""},
					})
					tt.topics[0].Publish(ctx, &pubsub.Message{
						Data:        json.RawMessage(`2`),
						OrderingKey: "x",
						// Attributes: map[string]string{"skip":""},
					})
					tt.topics[0].Publish(ctx, &pubsub.Message{
						Data:        json.RawMessage(`3`),
						OrderingKey: "x",
						Attributes:  map[string]string{"deliver": ""},
					})

					i := int32(0)
					rCtx, cancel := context.WithCancel(ctx)
					err = sub.Receive(rCtx, func(ctx context.Context, m *pubsub.Message) {
						counter := atomic.AddInt32(&i, 1)
						var expected string
						// don't expect to get message 2
						if counter == 1 {
							expected = "1"
						} else {
							assert.Equal(t, int32(2), counter)
							expected = "3"
						}
						assert.Equal(t, ([]byte)(expected), m.Data)
						defer m.Ack()
						if counter >= 2 {
							cancel()
						}
					})
					assert.NoError(t, err)
				},
			},
		},
		{
			name: "sub purge via seek",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				topic, err := psc.CreateTopic(ctx, safeName(t))
				require.NoError(t, err)
				sub, err := psc.CreateSubscription(ctx, safeName(t), pubsub.SubscriptionConfig{Topic: topic})
				require.NoError(t, err)
				// this should make it use the non-streaming pull endpoint
				sub.ReceiveSettings.Synchronous = true
				// avoid wasting a lot of time spinning up extra gRPC connections
				sub.ReceiveSettings.NumGoroutines = 1
				tt.topics = []*pubsub.Topic{topic}
				tt.subs = []*pubsub.Subscription{sub}

				// send some messages that we're gonna purge
				for i := 0; i < 10; i++ {
					_, err := topic.Publish(ctx, &pubsub.Message{
						Data: json.RawMessage(strconv.Itoa(i)),
					}).Get(ctx)
					assert.NoError(t, err)
				}
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					// seek the sub to now to purge everything we published
					err := tt.subs[0].SeekToTime(ctx, time.Now())
					assert.NoError(t, err)

					// do a synchronous pull and verify there are no messages
					timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
					defer cancel()
					err = tt.subs[0].Receive(timeoutCtx, func(ctx context.Context, m *pubsub.Message) {
						assert.Fail(t, "should not have received a message")
						m.Ack()
						cancel()
					})
					assert.NoError(t, err)
				},
			},
		},
	}...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testutil.Context(t)
			if tt.before != nil {
				tt.before(t, ctx, client, tt, psClient)
			}
			teg, tegCtx := errgroup.WithContext(ctx)
			require.NotEmpty(t, tt.steps)
			for _, s := range tt.steps {
				s := s
				require.NotNil(t, s)
				teg.Go(func() error {
					s(t, tegCtx, client, tt, psClient)
					return nil
				})
			}
			assert.NoError(t, teg.Wait())
			if tt.after != nil {
				tt.after(t, ctx, client, tt)
			}
		})
	}
}

func BenchmarkGrpcThroughputSmall(b *testing.B) {
	for _, pg := range []bool{false, true} {
		mode := "SQLite"
		if pg {
			mode = "PostgreSQL"
		}
		for _, topics := range []int{1, 2} {
			for _, subsPerTopic := range []int{1, 2} {
				for _, maxMessages := range []int{20, 100, 1000} {
					name := fmt.Sprintf("%s_%d_%d_%d", mode, topics, subsPerTopic, maxMessages)
					b.Run(name, func(b *testing.B) {
						if pg {
							benchUsePg(b)
						}
						benchThroughput(b, topics, subsPerTopic, maxMessages, json.RawMessage(`{}`))
					})
				}
			}
		}
	}
}

func BenchmarkGrpcThroughputHardSQLite(b *testing.B) {
	benchThroughput(b, 2, 2, 1000, json.RawMessage(`{}`))
}

func BenchmarkGrpcThroughputHardPostgreSQL(b *testing.B) {
	benchUsePg(b)
	benchThroughput(b, 2, 2, 1000, json.RawMessage(`{}`))
}

func benchUsePg(b *testing.B) {
	b.Setenv("NODE_ENV", "acceptance")
}

func benchThroughput(b *testing.B, numTopics, subsPerTopic, maxMessages int, payload json.RawMessage) {
	_, psClient := initGrpcTest(b)
	ctx := testutil.Context(b)

	topics := make([]*pubsub.Topic, numTopics)
	pubs := make([][]*pubsub.PublishResult, numTopics)
	subs := make([]*pubsub.Subscription, numTopics*subsPerTopic)
	for i := 0; i < numTopics; i++ {
		var err error
		topics[i], err = psClient.CreateTopic(ctx, safeName(b)+"_"+strconv.Itoa(i))
		require.NoError(b, err)
		pubs[i] = make([]*pubsub.PublishResult, b.N)
	}
	for i := 0; i < numTopics*subsPerTopic; i++ {
		var err error
		subs[i], err = psClient.CreateSubscription(
			ctx,
			safeName(b)+"_"+strconv.Itoa(i),
			pubsub.SubscriptionConfig{Topic: topics[i%numTopics]},
		)
		require.NoError(b, err)
		// most of our apps constrain this from its default of 1000 down to 20, or
		// even less. note that this has a _huge_ impact on per-sub throughput!
		subs[i].ReceiveSettings.MaxOutstandingMessages = maxMessages
	}

	eg, egCtx := errgroup.WithContext(testutil.Context(b))
	b.ResetTimer()
	start := time.Now()
	var pubDuration time.Duration
	for i, topic := range topics {
		i := i
		topic := topic
		eg.Go(func() error {
			for j := 0; j < b.N; j++ {
				pubs[i][j] = topic.Publish(egCtx, &pubsub.Message{
					Data:       payload,
					Attributes: map[string]string{"n": strconv.Itoa(j)},
				})
			}
			for j := 0; j < b.N; j++ {
				_, err := pubs[i][j].Get(egCtx)
				assert.NoError(b, err, "publish should not fail on topic/message %d/%d", i, j)
			}
			thisPD := time.Since(start)
			for {
				pd := atomic.LoadInt64((*int64)(&pubDuration))
				if int64(thisPD) > pd {
					if atomic.CompareAndSwapInt64((*int64)(&pubDuration), pd, int64(thisPD)) {
						break
					}
				}
			}
			// b.Log("publish done", i, b.N)
			return nil
		})
	}
	var totalExtra int32
	for i, sub := range subs {
		i := i
		sub := sub
		eg.Go(func() error {
			var mu sync.Mutex
			received := make(map[int]bool, b.N)
			extraReceives := 0
			ctx, cancel := context.WithCancel(egCtx)
			defer cancel()
			err := sub.Receive(ctx, func(c context.Context, m *pubsub.Message) {
				m.Ack()
				mu.Lock()
				defer mu.Unlock()
				n, err := strconv.Atoi(m.Attributes["n"])
				if assert.NoError(b, err) {
					// assert.NotContains(b, received, n, "should not double-receive %d/%d on %d", n, b.N, i)
					if received[n] {
						extraReceives++
					}
					received[n] = true
				}
				assert.GreaterOrEqual(b, n, 0)
				assert.Less(b, n, b.N)
				if len(received) >= b.N {
					// got everything, we're done
					go cancel()
				}
			})
			assert.Equal(b, b.N, len(received), "should receive correct number for %d", i)
			if extraReceives > 0 {
				// b.Logf("Got %d extra receives on sub %d for N=%d", extraReceives, i, b.N)
				atomic.AddInt32(&totalExtra, int32(extraReceives))
			}
			// b.Log("receive done", i, b.N, numReceived)
			return err
		})
	}
	assert.NoError(b, eg.Wait())
	b.StopTimer()
	duration := time.Since(start)
	b.ReportMetric(float64(b.N)*float64(numTopics)/pubDuration.Seconds(), "pubs/sec")
	b.ReportMetric(float64(b.N)*float64(numTopics*subsPerTopic)/duration.Seconds(), "msgs/sec")
	b.ReportMetric(float64(totalExtra), "dupes")
	// b.Log("bench done", b.N)

	// let things settle down before we kill off the test
	time.Sleep(25 * time.Millisecond)
}
