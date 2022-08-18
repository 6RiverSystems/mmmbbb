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

package actions

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/enttest"
)

func TestGetSubscriptionMessages_Execute(t *testing.T) {
	type test struct {
		name        string
		before      func(t *testing.T, ctx context.Context, client *ent.Client, tt *test)
		params      GetSubscriptionMessagesParams
		concurrent  func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, notify <-chan struct{})
		assertion   assert.ErrorAssertionFunc
		numExpected int
		after       func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, r *getSubscriptionMessagesResults)
	}
	tests := []test{
		{
			"empty, instantaneous",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				createSubscriptionClient(t, ctx, client, topic, 0)
			},
			GetSubscriptionMessagesParams{
				// Name will be auto-filled
				MaxMessages: 1,
				MaxBytes:    1,
				MaxWait:     time.Nanosecond, // 0 is a "use default", not "instantaneous"
			},
			nil,
			assert.NoError,
			0,
			nil,
		},
		{
			"non-empty, instantaneous",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				sub := createSubscriptionClient(t, ctx, client, topic, 0)
				msg := createMessageClient(t, ctx, client, topic, 0)
				createDeliveryClient(t, ctx, client, sub, msg, 0)
			},
			GetSubscriptionMessagesParams{
				// Name will be auto-filled
				MaxMessages: 1,
				MaxBytes:    1,
				MaxWait:     time.Nanosecond, // 0 is a "use default", not "instantaneous"
			},
			nil,
			assert.NoError,
			1,
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, r *getSubscriptionMessagesResults) {
				if assert.NotEmpty(t, r.deliveries, "should get at least one delivery") {
					d := r.deliveries[0]
					assert.Equal(t, xID(t, &ent.Delivery{}, 0), d.ID)
					// most checking of the delivery is in common code
				}
			},
		},
		{
			"unsatisfied order, instantaneous",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				sub := createSubscriptionClient(t, ctx, client, topic, 0, func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
					return sc.SetOrderedDelivery(true)
				})
				msg1 := createMessageClient(t, ctx, client, topic, 0, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetOrderKey(t.Name())
				})
				del1 := createDeliveryClient(t, ctx, client, sub, msg1, 0, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					// have to set this one as not ready to attempt, to prevent it from
					// being delivered in the request
					return dc.SetAttemptAt(time.Now().Add(time.Hour))
				})
				msg2 := createMessageClient(t, ctx, client, topic, 1, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetOrderKey(t.Name())
				})
				createDeliveryClient(t, ctx, client, sub, msg2, 1, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetNotBeforeID(del1.ID)
				})
			},
			GetSubscriptionMessagesParams{
				// Name will be auto-filled
				MaxMessages: 1,
				MaxBytes:    1,
				MaxWait:     time.Nanosecond, // 0 is a "use default", not "instantaneous"
			},
			nil,
			assert.NoError,
			0,
			nil,
		},
		{
			"satisfied order, instantaneous",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				sub := createSubscriptionClient(t, ctx, client, topic, 0, func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
					return sc.SetOrderedDelivery(true)
				})
				msg1 := createMessageClient(t, ctx, client, topic, 0, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetOrderKey(t.Name())
				})
				del1 := createDeliveryClient(t, ctx, client, sub, msg1, 0, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetAttemptAt(time.Now().Add(time.Hour)).SetCompletedAt(time.Now())
				})
				msg2 := createMessageClient(t, ctx, client, topic, 1, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetOrderKey(t.Name())
				})
				createDeliveryClient(t, ctx, client, sub, msg2, 1, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetNotBeforeID(del1.ID)
				})
			},
			GetSubscriptionMessagesParams{
				// Name will be auto-filled
				MaxMessages: 1,
				MaxBytes:    1,
				MaxWait:     time.Nanosecond, // 0 is a "use default", not "instantaneous"
			},
			nil,
			assert.NoError,
			1,
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, r *getSubscriptionMessagesResults) {
				d := r.deliveries[0]
				assert.Equal(t, xID(t, &ent.Delivery{}, 1), d.ID)
				// most checking of the delivery is in common code
			},
		},
		{
			"expired order, instantaneous",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				sub := createSubscriptionClient(t, ctx, client, topic, 0, func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
					return sc.SetOrderedDelivery(true)
				})
				msg1 := createMessageClient(t, ctx, client, topic, 0, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetOrderKey(t.Name())
				})
				del1 := createDeliveryClient(t, ctx, client, sub, msg1, 0, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetAttemptAt(time.Now().Add(time.Hour)).SetExpiresAt(time.Now())
				})
				msg2 := createMessageClient(t, ctx, client, topic, 1, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetOrderKey(t.Name())
				})
				createDeliveryClient(t, ctx, client, sub, msg2, 1, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetNotBeforeID(del1.ID)
				})
			},
			GetSubscriptionMessagesParams{
				// Name will be auto-filled
				MaxMessages: 1,
				MaxBytes:    1,
				MaxWait:     time.Nanosecond, // 0 is a "use default", not "instantaneous"
			},
			nil,
			assert.NoError,
			1,
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, r *getSubscriptionMessagesResults) {
				d := r.deliveries[0]
				assert.Equal(t, xID(t, &ent.Delivery{}, 1), d.ID)
				// most checking of the delivery is in common code
			},
		},
		{
			"message limit, instantaneous",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				sub := createSubscriptionClient(t, ctx, client, topic, 0)
				msg1 := createMessageClient(t, ctx, client, topic, 0)
				createDeliveryClient(t, ctx, client, sub, msg1, 0)
				msg2 := createMessageClient(t, ctx, client, topic, 1)
				createDeliveryClient(t, ctx, client, sub, msg2, 1)
			},
			GetSubscriptionMessagesParams{
				// Name will be auto-filled
				MaxMessages: 1,
				MaxBytes:    1_000_000,
				MaxWait:     time.Nanosecond, // 0 is a "use default", not "instantaneous"
			},
			nil,
			assert.NoError,
			1,
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, r *getSubscriptionMessagesResults) {
				d := r.deliveries[0]
				// receiving either message is valid
				assert.Contains(t, []uuid.UUID{
					xID(t, &ent.Delivery{}, 0),
					xID(t, &ent.Delivery{}, 1),
				}, d.ID)
				// most checking of the delivery is in common code
			},
		},
		{
			"byte limit, instantaneous",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				sub := createSubscriptionClient(t, ctx, client, topic, 0)
				msg1 := createMessageClient(t, ctx, client, topic, 0, func(mc *ent.MessageCreate) *ent.MessageCreate {
					// length 5
					return mc.SetPayload(json.RawMessage(`"123"`))
				})
				createDeliveryClient(t, ctx, client, sub, msg1, 0)
				msg2 := createMessageClient(t, ctx, client, topic, 1, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetPayload(json.RawMessage(`"123"`))
				})
				createDeliveryClient(t, ctx, client, sub, msg2, 1)
			},
			GetSubscriptionMessagesParams{
				// Name will be auto-filled
				MaxMessages: 100,
				// one byte less than what is required to fit the second message
				MaxBytes: 9,
				MaxWait:  time.Nanosecond, // 0 is a "use default", not "instantaneous"
			},
			nil,
			assert.NoError,
			1,
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, r *getSubscriptionMessagesResults) {
				d := r.deliveries[0]
				// receiving either message is valid
				assert.Contains(t, []uuid.UUID{
					xID(t, &ent.Delivery{}, 0),
					xID(t, &ent.Delivery{}, 1),
				}, d.ID)
				// most checking of the delivery is in common code
			},
		},
		{
			"strict byte limit, instantaneous",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				sub := createSubscriptionClient(t, ctx, client, topic, 0)
				msg1 := createMessageClient(t, ctx, client, topic, 0, func(mc *ent.MessageCreate) *ent.MessageCreate {
					// length 5
					return mc.SetPayload(json.RawMessage(`"123"`))
				})
				createDeliveryClient(t, ctx, client, sub, msg1, 0)
				msg2 := createMessageClient(t, ctx, client, topic, 1, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetPayload(json.RawMessage(`"123"`))
				})
				createDeliveryClient(t, ctx, client, sub, msg2, 1)
			},
			GetSubscriptionMessagesParams{
				// Name will be auto-filled
				MaxMessages: 100,
				// one byte less than what is required to fit any message
				MaxBytes:       4,
				MaxBytesStrict: true,
				MaxWait:        time.Nanosecond, // 0 is a "use default", not "instantaneous"
			},
			nil,
			assert.NoError,
			0,
			nil,
		},
		{
			"non-empty, delayed",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				createSubscriptionClient(t, ctx, client, topic, 0)
			},
			GetSubscriptionMessagesParams{
				// Name will be auto-filled
				MaxMessages: 1,
				MaxBytes:    1,
				MaxWait:     time.Minute,
			},
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, notify <-chan struct{}) {
				select {
				case <-ctx.Done():
					return
				case <-notify:
					// fall through
				case <-time.After(100 * time.Millisecond):
					// fall through, expect failure
				}
				topic := &ent.Topic{}
				topic.ID = xID(t, topic, 0)
				sub := &ent.Subscription{}
				sub.ID = xID(t, sub, 0)
				msg := createMessageClient(t, ctx, client, topic, 0)
				createDeliveryClient(t, ctx, client, sub, msg, 0)
				WakePublishListeners(false, sub.ID)
			},
			assert.NoError,
			1,
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, r *getSubscriptionMessagesResults) {
				d := r.deliveries[0]
				assert.Equal(t, xID(t, &ent.Delivery{}, 0), d.ID)
				// most checking of the delivery is in common code
			},
		},
		{
			"satisfied order, delayed",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				sub := createSubscriptionClient(t, ctx, client, topic, 0, func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
					return sc.SetOrderedDelivery(true)
				})
				msg1 := createMessageClient(t, ctx, client, topic, 0, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetOrderKey(t.Name())
				})
				del1 := createDeliveryClient(t, ctx, client, sub, msg1, 0, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetAttemptAt(time.Now().Add(time.Hour))
				})
				msg2 := createMessageClient(t, ctx, client, topic, 1, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetOrderKey(t.Name())
				})
				createDeliveryClient(t, ctx, client, sub, msg2, 1, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetNotBeforeID(del1.ID)
				})
			},
			GetSubscriptionMessagesParams{
				// Name will be auto-filled
				MaxMessages: 1,
				MaxBytes:    1,
				MaxWait:     time.Minute,
			},
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, notify <-chan struct{}) {
				select {
				case <-ctx.Done():
					return
				case <-notify:
					// fall through
				case <-time.After(100 * time.Millisecond):
					// fall through, expect failure
				}
				assert.NoError(t, client.DoCtxTx(ctx, nil, func(ctx context.Context, tx *ent.Tx) error {
					if err := tx.Delivery.UpdateOneID(xID(t, &ent.Delivery{}, 0)).SetCompletedAt(time.Now()).Exec(ctx); err != nil {
						return err
					}
					notifyPublish(tx, xID(t, &ent.Subscription{}, 0))
					return nil
				}))
			},
			assert.NoError,
			1,
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, r *getSubscriptionMessagesResults) {
				d := r.deliveries[0]
				assert.Equal(t, xID(t, &ent.Delivery{}, 1), d.ID)
				// most checking of the delivery is in common code
			},
		},
		{
			"expired order, delayed",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				sub := createSubscriptionClient(t, ctx, client, topic, 0, func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
					return sc.SetOrderedDelivery(true)
				})
				msg1 := createMessageClient(t, ctx, client, topic, 0, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetOrderKey(t.Name())
				})
				del1 := createDeliveryClient(t, ctx, client, sub, msg1, 0, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetAttemptAt(time.Now().Add(time.Hour)).SetExpiresAt(time.Now().Add(100 * time.Millisecond))
				})
				msg2 := createMessageClient(t, ctx, client, topic, 1, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetOrderKey(t.Name())
				})
				createDeliveryClient(t, ctx, client, sub, msg2, 1, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetNotBeforeID(del1.ID)
				})
			},
			GetSubscriptionMessagesParams{
				// Name will be auto-filled
				MaxMessages: 1,
				MaxBytes:    1,
				MaxWait:     time.Minute,
			},
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, _ <-chan struct{}) {
				assert.NoError(t, client.DoCtxTx(ctx, nil, func(ctx context.Context, tx *ent.Tx) error {
					if err := tx.Delivery.UpdateOneID(xID(t, &ent.Delivery{}, 0)).SetExpiresAt(time.Now()).Exec(ctx); err != nil {
						return err
					}
					notifyPublish(tx, xID(t, &ent.Subscription{}, 0))
					return nil
				}))
			},
			assert.NoError,
			1,
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, r *getSubscriptionMessagesResults) {
				require.NotNil(t, r)
				require.NotNil(t, r.deliveries)
				d := r.deliveries[0]
				assert.Equal(t, xID(t, &ent.Delivery{}, 1), d.ID)
				// most checking of the delivery is in common code
			},
		},
		{
			"deadletter",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				dlTopic := createTopicClient(t, ctx, client, 1)
				sub := createSubscriptionClient(t, ctx, client, topic, 0, withDeadLetter(dlTopic, 1))
				createSubscriptionClient(t, ctx, client, dlTopic, 1)
				msg := createMessageClient(t, ctx, client, topic, 0)
				createDeliveryClient(t, ctx, client, sub, msg, 0, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetAttempts(1)
				})
			},
			GetSubscriptionMessagesParams{
				// Name will be auto-filled
				MaxMessages: 1,
				MaxBytes:    1,
				MaxWait:     time.Nanosecond, // 0 is a "use default", not "instantaneous"
			},
			nil,
			assert.NoError,
			0,
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, r *getSubscriptionMessagesResults) {
				assert.Equal(t, 1, r.numDeadLettered)

				origDelivery, err := client.Delivery.Get(ctx, xID(t, &ent.Delivery{}, 0))
				require.NoError(t, err)

				dlDelivery, err := client.Delivery.Query().
					Where(delivery.SubscriptionID(xID(t, &ent.Subscription{}, 1))).
					Only(ctx)
				require.NoError(t, err)

				assert.Equal(t, origDelivery.MessageID, dlDelivery.MessageID)
				assert.Nil(t, dlDelivery.CompletedAt)
				if assert.NotNil(t, origDelivery.CompletedAt) {
					assert.GreaterOrEqual(t, dlDelivery.AttemptAt.UnixNano(), origDelivery.CompletedAt.UnixNano())
				}
				assert.LessOrEqual(t, dlDelivery.AttemptAt.UnixNano(), time.Now().UnixNano())
			},
		},
	}
	client := enttest.ClientForTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			ctx := testutils.ContextForTest(t)
			if tt.before != nil {
				tt.before(t, ctx, client, &tt)
			}
			if tt.params.Name == "" && tt.params.ID == nil {
				tt.params.Name = nameFor(t, 0)
			}
			notify := make(chan struct{})
			tt.params.Waiting = notify
			a := NewGetSubscriptionMessages(tt.params)
			eg, egCtx := errgroup.WithContext(ctx)
			eg.Go(func() error {
				if tt.concurrent != nil {
					tt.concurrent(t, egCtx, client, &tt, notify)
				}
				return nil
			})
			err := a.ExecuteClient(ctx, client)
			assert.NoError(t, eg.Wait())
			tt.assertion(t, err)
			if err == nil {
				if assert.True(t, a.HasResults()) {
					assert.Equal(t, a.results.deliveries, a.Deliveries())
					for _, d := range a.Deliveries() {
						del, err := client.Delivery.Get(ctx, d.ID)
						require.NoError(t, err)
						msg, err := del.QueryMessage().Only(ctx)
						require.NoError(t, err)
						assert.Equal(t, msg.Attributes, d.Attributes)
						assert.Equal(t, msg.ID, d.MessageID)
						assert.Equal(t, msg.Payload, d.Payload)
						assert.Equal(t, msg.OrderKey, d.OrderKey)
						assert.Equal(t, msg.PublishedAt, d.PublishedAt)
						// zone info will likely be different here, we just care that they
						// represent the same time, within the fuzz boundary
						assert.GreaterOrEqual(t, del.AttemptAt.UnixNano(), d.NextAttemptAt.UnixNano())
						assert.LessOrEqual(t, del.AttemptAt.UnixNano(), d.NextAttemptAt.Add(time.Second).UnixNano())
						assert.Equal(t, del.Attempts, d.NumAttempts)
						assert.Greater(t, del.Attempts, 0)
						assert.Greater(t, del.AttemptAt.UnixNano(), time.Now().UnixNano())
					}
					require.Len(t, a.Deliveries(), tt.numExpected)
				}
			} else {
				assert.False(t, a.HasResults())
				require.Nil(t, a.results)
			}
			if tt.after != nil {
				tt.after(t, ctx, client, &tt, a.results)
			}
		})
	}
}
