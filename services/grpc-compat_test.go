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

	"cloud.google.com/go/pubsub/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.6river.tech/mmmbbb/actions"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/ent/topic"
	"go.6river.tech/mmmbbb/grpc/pubsubpb"
	"go.6river.tech/mmmbbb/internal"
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
		topics  []*pubsub.Publisher
		subs    []*pubsub.Subscriber
		servers []*httptest.Server
		errs    []error
		waiters []chan struct{}
		mu      sync.Mutex
	}
	type testStep = func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client)

	createTopic := func(
		ctx context.Context,
		psc *pubsub.Client,
		id string,
	) (*pubsub.Publisher, error) {
		if tm, err := psc.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
			Name: internal.PSTopicName(psc.Project(), id),
		}); err != nil {
			return nil, err
		} else {
			return psc.Publisher(tm.Name), nil
		}
	}
	createSub := func(
		ctx context.Context,
		psc *pubsub.Client,
		topic *pubsub.Publisher,
		id string,
	) (*pubsub.Subscriber, error) {
		if sm, err := psc.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
			Name:  internal.PSSubName(psc.Project(), id),
			Topic: topic.String(), // returns Name
		}); err != nil {
			return nil, err
		} else {
			return psc.Subscriber(sm.Name), nil
		}
	}

	tests := []*test{
		{
			name: "concurrent topic create",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				tt.topics = make([]*pubsub.Publisher, 2)
				tt.errs = make([]error, 2)
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					tt.topics[0], tt.errs[0] = createTopic(ctx, psc, safeID(t))
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
					tt.topics[1], tt.errs[1] = createTopic(ctx, psc, safeID(t))
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
				tt.topics = make([]*pubsub.Publisher, 1)
				var err error
				tt.topics[0], err = createTopic(ctx, psc, safeID(t))
				require.NoError(t, err)
				tt.errs = make([]error, 2)
				tt.subs = make([]*pubsub.Subscriber, 2)
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					tt.subs[0], tt.errs[0] = createSub(ctx, psc, tt.topics[0], safeID(t))
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
					tt.subs[1], tt.errs[1] = createSub(ctx, psc, tt.topics[0], safeID(t))
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
				tt.topics = make([]*pubsub.Publisher, numTopics)
				for i := range tt.topics {
					var err error
					tt.topics[i], err = createTopic(ctx, psc, safeID(t)+"_"+strconv.Itoa(i))
					require.NoError(t, err)
				}
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					expected := make([]string, len(tt.topics))
					for i, t := range tt.topics {
						expected[i] = t.String() // Name
					}
					ti := psc.TopicAdminClient.ListTopics(ctx, &pubsubpb.ListTopicsRequest{
						Project: "projects/" + psc.Project(),
					})
					var actual []string
					for topic, err := ti.Next(); ; topic, err = ti.Next() {
						if err == iterator.Done {
							assert.Nil(t, topic)
							break
						}
						assert.NoError(t, err)
						require.NotNil(t, topic)
						actual = append(actual, topic.Name)
					}
					assert.ElementsMatch(t, expected, actual)
				},
			},
		},
		{
			name: "list subs on one topic",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				const numSubs = 5
				tt.topics = make([]*pubsub.Publisher, 1)
				tt.subs = make([]*pubsub.Subscriber, numSubs)
				var err error
				tt.topics[0], err = createTopic(ctx, psc, safeID(t))
				require.NoError(t, err)
				for i := range tt.subs {
					tt.subs[i], err = createSub(ctx, psc, tt.topics[0], safeID(t)+"_"+strconv.Itoa(i))
					require.NoError(t, err)
				}
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					expected := make([]string, len(tt.subs))
					for i, s := range tt.subs {
						expected[i] = s.String() // Name
					}
					si := psc.TopicAdminClient.ListTopicSubscriptions(
						ctx,
						&pubsubpb.ListTopicSubscriptionsRequest{
							Topic: tt.topics[0].String(), // Name
						},
					)
					var actual []string
					for sub, err := si.Next(); ; sub, err = si.Next() {
						if err == iterator.Done {
							assert.Empty(t, sub)
							break
						}
						assert.NoError(t, err)
						require.NotNil(t, sub)
						actual = append(actual, sub)
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
				tt.topics = make([]*pubsub.Publisher, numTopics)
				tt.subs = make([]*pubsub.Subscriber, numSubs)
				var err error
				for i := range tt.topics {
					var err error
					tt.topics[i], err = createTopic(ctx, psc, safeID(t)+"_"+strconv.Itoa(i))
					require.NoError(t, err)
				}
				for i := range tt.subs {
					tt.subs[i], err = createSub(ctx, psc, tt.topics[i%numTopics], safeID(t)+"_"+strconv.Itoa(i))
					require.NoError(t, err)
				}
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					expected := make([]string, len(tt.subs))
					for i, s := range tt.subs {
						expected[i] = s.String() // Name
					}
					si := psc.SubscriptionAdminClient.ListSubscriptions(ctx, &pubsubpb.ListSubscriptionsRequest{
						Project: "projects/" + psc.Project(),
					})
					var actual []string
					for sub, err := si.Next(); ; sub, err = si.Next() {
						if err == iterator.Done {
							assert.Nil(t, sub)
							break
						}
						assert.NoError(t, err)
						require.NotNil(t, sub)
						actual = append(actual, sub.Name)
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
					tac := psc.TopicAdminClient
					tName := internal.PSTopicName(psc.Project(), safeID(t))
					_, err := tac.GetTopic(ctx, &pubsubpb.GetTopicRequest{Topic: tName})
					require.Equal(t, codes.NotFound, status.Code(err), "topic should not exist: %#v", err)

					_, err = createTopic(ctx, psc, safeID(t))
					require.NoError(t, err)

					cfg, err := tac.GetTopic(ctx, &pubsubpb.GetTopicRequest{Topic: tName})
					require.NoError(t, err)
					assert.Empty(t, cfg.KmsKeyName)
					assert.Empty(t, cfg.Labels)
					assert.Empty(t, cfg.MessageStoragePolicy.GetAllowedPersistenceRegions())

					cfg, err = tac.UpdateTopic(ctx, &pubsubpb.UpdateTopicRequest{
						Topic: &pubsubpb.Topic{
							Name: tName,
							Labels: map[string]string{
								"forTest": t.Name(),
							},
						},
						UpdateMask: &fieldmaskpb.FieldMask{
							Paths: []string{"labels"},
						},
					})
					require.NoError(t, err)
					assert.Empty(t, cfg.KmsKeyName)
					assert.Equal(t, map[string]string{"forTest": t.Name()}, cfg.Labels)
					assert.Empty(t, cfg.MessageStoragePolicy.GetAllowedPersistenceRegions())

					_, err = tac.GetTopic(ctx, &pubsubpb.GetTopicRequest{Topic: tName})
					require.NoError(t, err)

					err = tac.DeleteTopic(ctx, &pubsubpb.DeleteTopicRequest{Topic: tName})
					require.NoError(t, err)

					_, err = tac.GetTopic(ctx, &pubsubpb.GetTopicRequest{Topic: tName})
					require.Equal(t, codes.NotFound, status.Code(err), "topic should not exist after delete: %#v", err)
				},
			},
		},
		{
			// TODO: this isn't a very "unity" test
			name: "subscription lifecycle",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				topic, err := createTopic(ctx, psc, safeID(t))
				require.NoError(t, err)
				tt.topics = []*pubsub.Publisher{topic}
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					sac := psc.SubscriptionAdminClient
					sName := internal.PSSubName(psc.Project(), safeID(t))
					_, err := sac.GetSubscription(ctx, &pubsubpb.GetSubscriptionRequest{Subscription: sName})
					require.Equal(t, codes.NotFound, status.Code(err), "subscription should not exist: %#v", err)

					sub, err := createSub(ctx, psc, tt.topics[0], safeID(t))
					require.NoError(t, err)

					cfg, err := sac.GetSubscription(ctx, &pubsubpb.GetSubscriptionRequest{Subscription: sName})
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
					assert.Equal(t, cfg.Topic, tt.topics[0].String())

					cfg, err = sac.UpdateSubscription(ctx, &pubsubpb.UpdateSubscriptionRequest{
						Subscription: &pubsubpb.Subscription{
							Name: sName,
							ExpirationPolicy: &pubsubpb.ExpirationPolicy{
								Ttl: durationpb.New(42 * time.Hour),
							},
							Labels: map[string]string{
								"forTest": t.Name(),
							},
							// can't change EnableMessageOrdering on the fly with the google
							// client, even though our code supports it
							MessageRetentionDuration: durationpb.New(time.Hour),
							RetryPolicy: &pubsubpb.RetryPolicy{
								MinimumBackoff: durationpb.New(4200 * time.Millisecond),
								MaximumBackoff: durationpb.New(42420 * time.Millisecond),
							},
						},
						UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{
							"expiration_policy",
							"labels",
							"message_retention_duration",
							"retry_policy",
						}},
					})
					require.NoError(t, err)
					// cfg.AckDeadline // we don't allow setting this
					assert.Nil(t, cfg.DeadLetterPolicy)
					assert.False(t, cfg.Detached)
					assert.False(t, cfg.EnableMessageOrdering)
					assert.Equal(t, 42*time.Hour, cfg.ExpirationPolicy.GetTtl().AsDuration())
					assert.Empty(t, cfg.Filter)
					assert.Equal(t, map[string]string{"forTest": t.Name()}, cfg.Labels)
					assert.Zero(t, cfg.PushConfig)
					assert.False(t, cfg.RetainAckedMessages)
					assert.Equal(t, durationpb.New(time.Hour), cfg.MessageRetentionDuration)
					assert.Equal(t, &pubsubpb.RetryPolicy{
						MinimumBackoff: durationpb.New(4200 * time.Millisecond),
						MaximumBackoff: durationpb.New(42420 * time.Millisecond),
					}, cfg.RetryPolicy)
					assert.Equal(t, cfg.Topic, tt.topics[0].String())

					_, err = sac.GetSubscription(ctx, &pubsubpb.GetSubscriptionRequest{Subscription: sub.String()})
					require.NoError(t, err)

					err = sac.DeleteSubscription(ctx, &pubsubpb.DeleteSubscriptionRequest{Subscription: sub.String()})
					require.NoError(t, err)

					_, err = sac.GetSubscription(ctx, &pubsubpb.GetSubscriptionRequest{Subscription: sub.String()})
					require.Equal(t, codes.NotFound, status.Code(err), "subscription should not exist after delete: %#v", err)
				},
			},
		},
	}
	const pullTimeScale = 100 * time.Millisecond
	tests = append(tests, []*test{
		{
			name: "streaming pull",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				topic, err := createTopic(ctx, psc, safeID(t))
				require.NoError(t, err)
				subM, err := psc.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
					Name:  internal.PSSubName(psc.Project(), safeID(t)),
					Topic: topic.String(), // Name
					RetryPolicy: &pubsubpb.RetryPolicy{
						MinimumBackoff: durationpb.New(pullTimeScale * 5 / 4),
					},
				})
				require.NoError(t, err)
				sub := psc.Subscriber(subM.Name)
				// avoid wasting a lot of time spinning up extra gRPC connections
				sub.ReceiveSettings.NumGoroutines = 1
				tt.topics = []*pubsub.Publisher{topic}
				tt.subs = []*pubsub.Subscriber{sub}
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
					assert.NoError(
						t,
						tt.subs[0].Receive(rCtx, func(ctx context.Context, m *pubsub.Message) {
							counter := atomic.AddInt32(&i, 1)
							// t.Logf("received %q as %q at %d", string(m.Data), m.ID, counter)
							expected := counter
							if expected > 2 {
								expected = 2
							}
							assert.Equal(t, strconv.Itoa(int(expected)), string(m.Data),
								"should get expected message (in order)")
							switch counter {
							case 1:
								// go fast
								defer m.Ack()
							case 2:
								// delay and nack
								defer m.Nack()
								time.Sleep(pullTimeScale)
							case 3:
								// delay and ack and done
								defer m.Ack()
								time.Sleep(pullTimeScale * 3 / 2)
								cancel()
							default:
								assert.LessOrEqual(t, expected, int32(3))
							}
							// t.Logf("done with %q at %d", m.ID, counter)
						}),
					)
				},
			},
		},
		// TODO: synchronous pull is not supported in the v2 client, to test this
		// code path we need to use the "raw" client
		/*
			{
				name: "synchronous pull",
				before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					topic, err := createTopic(ctx, psc, safeID(t))
					require.NoError(t, err)
					subM, err := psc.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
						Name:  internal.PSSubName(psc.Project(), safeID(t)),
						Topic: topic.String(), // Name
						RetryPolicy: &pubsubpb.RetryPolicy{
							MinimumBackoff: durationpb.New(pullTimeScale * 5 / 4),
						},
					})
					require.NoError(t, err)
					sub := psc.Subscriber(subM.Name)
					// this should make it use the non-streaming pull endpoint
					sub.ReceiveSettings.Synchronous = true
					// avoid wasting a lot of time spinning up extra gRPC connections
					sub.ReceiveSettings.NumGoroutines = 1
					tt.topics = []*pubsub.Publisher{topic}
					tt.subs = []*pubsub.Subscriber{sub}
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
							switch counter {
							case 1:
								// go fast
								defer m.Ack()
							case 2:
								// delay and nack
								defer m.Nack()
								time.Sleep(pullTimeScale)
							case 3:
								// delay and ack and done
								defer m.Ack()
								time.Sleep(pullTimeScale * 3 / 2)
								cancel()
							default:
								assert.LessOrEqual(t, expected, int32(3))
							}
						})
						assert.NoError(t, err)
					},
				},
			},
		*/
	}...)
	tests = append(tests, []*test{
		{
			name: "subscription http push",
			before: func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
				srv := httptest.NewUnstartedServer(
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						// test step will replace this with the real listener before starting the server
						panic("should never be called")
					}),
				)
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

				topic, err := createTopic(ctx, psc, safeID(t))
				require.NoError(t, err)
				subM, err := psc.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
					Name:  internal.PSSubName(psc.Project(), safeID(t)),
					Topic: topic.String(), // Name
					RetryPolicy: &pubsubpb.RetryPolicy{
						MinimumBackoff: durationpb.New(51 * time.Millisecond),
					},
					PushConfig: &pubsubpb.PushConfig{
						PushEndpoint: serverURL,
					},
				})
				require.NoError(t, err)
				tt.topics = []*pubsub.Publisher{topic}
				tt.subs = []*pubsub.Subscriber{psc.Subscriber(subM.Name)}
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
					tt.servers[0].Config.Handler = http.HandlerFunc(
						func(w http.ResponseWriter, r *http.Request) {
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
							switch counter {
							case 1:
								// go fast
								w.WriteHeader(http.StatusNoContent)
							case 2:
								// delay and nack
								time.Sleep(50 * time.Millisecond)
								w.WriteHeader(http.StatusInternalServerError)
							case 3:
								// delay and ack and done
								time.Sleep(150 * time.Millisecond)
								w.WriteHeader(http.StatusNoContent)
								cancel()
							default:
								assert.LessOrEqual(t, expected, int32(3))
								// above will always fail
								w.WriteHeader(http.StatusInternalServerError)
							}
						},
					)
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
				topic, err := createTopic(ctx, psc, safeID(t))
				require.NoError(t, err)
				topic.EnableMessageOrdering = true
				tt.topics = []*pubsub.Publisher{topic}
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					subM, err := psc.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
						Name:                  internal.PSSubName(psc.Project(), safeID(t)),
						Topic:                 tt.topics[0].String(), // Name
						Filter:                "attributes:deliver",
						EnableMessageOrdering: true,
					})
					require.NoError(t, err)
					sub := psc.Subscriber(subM.Name)
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
				topic, err := createTopic(ctx, psc, safeID(t))
				require.NoError(t, err)
				sub, err := createSub(ctx, psc, topic, safeID(t))
				require.NoError(t, err)
				// avoid wasting a lot of time spinning up extra gRPC connections
				sub.ReceiveSettings.NumGoroutines = 1
				tt.topics = []*pubsub.Publisher{topic}
				tt.subs = []*pubsub.Subscriber{sub}

				// send some messages that we're gonna purge
				for i := range 10 {
					_, err := topic.Publish(ctx, &pubsub.Message{
						Data: json.RawMessage(strconv.Itoa(i)),
					}).Get(ctx)
					assert.NoError(t, err)
				}
			},
			steps: []testStep{
				func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, psc *pubsub.Client) {
					// seek the sub to now to purge everything we published
					_, err := psc.SubscriptionAdminClient.Seek(ctx, &pubsubpb.SeekRequest{
						Subscription: tt.subs[0].String(), // Name
						Target: &pubsubpb.SeekRequest_Time{
							Time: timestamppb.New(time.Now()),
						},
					})
					require.NoError(t, err)

					// do a synchronous pull and verify there are no messages
					timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
					defer cancel()
					err = tt.subs[0].Receive(
						timeoutCtx,
						func(ctx context.Context, m *pubsub.Message) {
							assert.Fail(t, "should not have received a message")
							m.Ack()
							cancel()
						},
					)
					assert.NoError(t, err)
				},
			},
		},
	}...)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			if tt.before != nil {
				tt.before(t, ctx, client, tt, psClient)
			}
			teg, tegCtx := errgroup.WithContext(ctx)
			require.NotEmpty(t, tt.steps)
			for _, s := range tt.steps {
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

func benchThroughput(
	b *testing.B,
	numTopics, subsPerTopic, maxMessages int,
	payload json.RawMessage,
) {
	_, psClient := initGrpcTest(b)
	ctx := b.Context()
	tac, sac := psClient.TopicAdminClient, psClient.SubscriptionAdminClient

	topics := make([]*pubsub.Publisher, numTopics)
	pubs := make([][]*pubsub.PublishResult, numTopics)
	subs := make([]*pubsub.Subscriber, numTopics*subsPerTopic)
	for i := range numTopics {
		topic, err := tac.CreateTopic(ctx, &pubsubpb.Topic{
			Name: internal.PSTopicName(psClient.Project(), safeID(b)+"_"+strconv.Itoa(i)),
		})
		require.NoError(b, err)
		topics[i] = psClient.Publisher(topic.Name)
		pubs[i] = make([]*pubsub.PublishResult, b.N)
	}
	for i := range numTopics * subsPerTopic {
		sub, err := sac.CreateSubscription(ctx, &pubsubpb.Subscription{
			Name:  internal.PSSubName(psClient.Project(), safeID(b)+"_"+strconv.Itoa(i)),
			Topic: topics[i%numTopics].String(), // Name
		})
		require.NoError(b, err)
		subs[i] = psClient.Subscriber(sub.Name)
		// most of our apps constrain this from its default of 1000 down to 20, or
		// even less. note that this has a _huge_ impact on per-sub throughput!
		subs[i].ReceiveSettings.MaxOutstandingMessages = maxMessages
	}

	eg, egCtx := errgroup.WithContext(b.Context())
	b.ResetTimer()
	start := time.Now()
	var pubDuration time.Duration
	for i, topic := range topics {
		eg.Go(func() error {
			for j := range b.N {
				pubs[i][j] = topic.Publish(egCtx, &pubsub.Message{
					Data:       payload,
					Attributes: map[string]string{"n": strconv.Itoa(j)},
				})
			}
			for j := range b.N {
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
