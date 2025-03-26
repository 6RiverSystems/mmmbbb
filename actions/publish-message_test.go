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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/enttest"
	"go.6river.tech/mmmbbb/ent/message"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/ent/topic"
)

//
//nolint:unparam
func expectMessages(
	t *testing.T,
	ctx context.Context,
	tx *ent.Tx,
	topicOffset, num int,
) []*ent.Message {
	topicID := xID(t, &ent.Topic{}, topicOffset)
	messages, err := tx.Message.Query().
		Where(message.HasTopicWith(topic.ID(topicID))).
		Order(ent.Asc(message.FieldPublishedAt)).
		All(ctx)
	assert.NoError(t, err)
	require.Len(t, messages, num)
	return messages
}

func expectNoDeliveries(t *testing.T, ctx context.Context, tx *ent.Tx) {
	num, err := tx.Delivery.Query().Count(ctx)
	assert.NoError(t, err)
	assert.Zero(t, num)
}

func expectDeliveries(
	t *testing.T,
	ctx context.Context,
	tx *ent.Tx,
	subOffset, num int,
) []*ent.Delivery {
	subID := xID(t, &ent.Subscription{}, subOffset)
	deliveries, err := tx.Delivery.Query().
		Where(delivery.HasSubscriptionWith(subscription.ID(subID))).
		WithMessage().
		WithNotBefore().
		WithSubscription().
		Order(ent.Asc(delivery.FieldPublishedAt)).
		All(ctx)
	assert.NoError(t, err)
	require.Len(t, deliveries, num)
	return deliveries
}

func TestPublishMessage_Execute(t *testing.T) {
	type test struct {
		name                string
		before              func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		params              PublishMessageParams
		assertion           assert.ErrorAssertionFunc
		results             *publishMessageResults
		expectPublishNotify []uuid.UUID
		after               func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
	}
	tests := []test{
		{
			// publish to a topic with no sub
			"no sub",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				tt.params.TopicName = topic.Name
			},
			PublishMessageParams{
				// TopicName set in before
				Payload: json.RawMessage(`{}`),
			},
			assert.NoError,
			&publishMessageResults{
				NumDeliveries: 0,
			},
			[]uuid.UUID{
				// zero: expect no sub to receive a message
				{},
			},
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				expectMessages(t, ctx, tx, 0, 1)
				expectNoDeliveries(t, ctx, tx)
			},
		},
		{
			// publish to a topic with one basic sub, by name
			"simple by name",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				tt.params.TopicName = topic.Name
				tt.expectPublishNotify = append(tt.expectPublishNotify, sub.ID)
			},
			PublishMessageParams{
				// TopicName set in before
				Payload: json.RawMessage(`{}`),
			},
			assert.NoError,
			&publishMessageResults{
				NumDeliveries: 1,
			},
			[]uuid.UUID{ /* filled in before */ },
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				expectMessages(t, ctx, tx, 0, 1)
				expectDeliveries(t, ctx, tx, 0, 1)
			},
		},
		{
			// publish to a topic with one basic sub, by id
			"simple by id",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				tt.params.TopicID = &topic.ID
				tt.expectPublishNotify = append(tt.expectPublishNotify, sub.ID)
			},
			PublishMessageParams{
				// TopicID set in before
				Payload: json.RawMessage(`{}`),
			},
			assert.NoError,
			&publishMessageResults{
				NumDeliveries: 1,
			},
			[]uuid.UUID{ /* filled in before */ },
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				expectMessages(t, ctx, tx, 0, 1)
				expectDeliveries(t, ctx, tx, 0, 1)
			},
		},
		{
			// publish to a topic with one basic sub, by id+name
			"simple by id+name",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				tt.params.TopicName = topic.Name
				tt.params.TopicID = &topic.ID
				tt.expectPublishNotify = append(tt.expectPublishNotify, sub.ID)
			},
			PublishMessageParams{
				// TopicName and TopicID set in before
				Payload: json.RawMessage(`{}`),
			},
			assert.NoError,
			&publishMessageResults{
				NumDeliveries: 1,
			},
			[]uuid.UUID{ /* filled in before */ },
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				expectMessages(t, ctx, tx, 0, 1)
				expectDeliveries(t, ctx, tx, 0, 1)
			},
		},
		{
			// publish a message with attributes
			"with attrs",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				tt.params.TopicName = topic.Name
				tt.expectPublishNotify = append(tt.expectPublishNotify, sub.ID)
			},
			PublishMessageParams{
				// TopicName set in before
				Payload: json.RawMessage(`{}`),
				Attributes: map[string]string{
					"hello": "world",
				},
			},
			assert.NoError,
			&publishMessageResults{
				NumDeliveries: 1,
			},
			[]uuid.UUID{ /* filled in before */ },
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				expectMessages(t, ctx, tx, 0, 1)
				expectDeliveries(t, ctx, tx, 0, 1)
			},
		},
		{
			// publish two a topic with multiple subs
			"multiple subs",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub1 := createSubscription(t, ctx, tx, topic, 0)
				sub2 := createSubscription(t, ctx, tx, topic, 1)
				tt.params.TopicName = topic.Name
				tt.expectPublishNotify = append(tt.expectPublishNotify, sub1.ID, sub2.ID)
			},
			PublishMessageParams{
				// TopicName set in before
				Payload: json.RawMessage(`{}`),
			},
			assert.NoError,
			&publishMessageResults{
				NumDeliveries: 2,
			},
			[]uuid.UUID{ /* filled in before */ },
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				expectMessages(t, ctx, tx, 0, 1)
				expectDeliveries(t, ctx, tx, 0, 1)
				expectDeliveries(t, ctx, tx, 1, 1)
			},
		},
		{
			// publish to a topic with an ordered sub
			"ordered sub",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(
					t,
					ctx,
					tx,
					topic,
					0,
					func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
						return sc.SetOrderedDelivery(true)
					},
				)
				// create an earlier delivery, new message should be "after" it
				msg := createMessage(
					t,
					ctx,
					tx,
					topic,
					0,
					func(mc *ent.MessageCreate) *ent.MessageCreate {
						return mc.SetOrderKey(t.Name())
					},
				)
				createDelivery(t, ctx, tx, sub, msg, 0)
				tt.params.TopicName = topic.Name
				tt.params.OrderKey = t.Name()
				tt.expectPublishNotify = append(tt.expectPublishNotify, sub.ID)
			},
			PublishMessageParams{
				// TopicName set in before
				Payload: json.RawMessage(`{}`),
				// OrderKey set in before
			},
			assert.NoError,
			&publishMessageResults{
				NumDeliveries: 1,
			},
			[]uuid.UUID{ /* filled in before */ },
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				messages := expectMessages(t, ctx, tx, 0, 2)
				deliveries := expectDeliveries(t, ctx, tx, 0, 2)
				assert.Equal(t, xID(t, &ent.Delivery{}, 0), deliveries[0].ID)
				assert.NotEqual(t, json.RawMessage(`{}`), messages[0].Payload)
				assert.Equal(t, json.RawMessage(`{}`), messages[1].Payload)
				assert.Equal(t, messages[0].ID, deliveries[0].Edges.Message.ID)
				assert.Equal(t, messages[1].ID, deliveries[1].Edges.Message.ID)
				assert.Equal(t, deliveries[0].ID, deliveries[1].Edges.NotBefore.ID)
			},
		},
		{
			// publish to a topic with multiple ordered subs
			"multiple ordered sub",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub1 := createSubscription(
					t,
					ctx,
					tx,
					topic,
					0,
					func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
						return sc.SetOrderedDelivery(true)
					},
				)
				sub2 := createSubscription(
					t,
					ctx,
					tx,
					topic,
					1,
					func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
						return sc.SetOrderedDelivery(true)
					},
				)
				// create an earlier delivery, new message should be "after" it
				msg := createMessage(
					t,
					ctx,
					tx,
					topic,
					0,
					func(mc *ent.MessageCreate) *ent.MessageCreate {
						return mc.SetOrderKey(t.Name())
					},
				)
				createDelivery(t, ctx, tx, sub1, msg, 0)
				createDelivery(t, ctx, tx, sub2, msg, 1)
				tt.params.TopicName = topic.Name
				tt.params.OrderKey = t.Name()
				tt.expectPublishNotify = append(tt.expectPublishNotify, sub1.ID, sub2.ID)
			},
			PublishMessageParams{
				// TopicName set in before
				Payload: json.RawMessage(`{}`),
				// OrderKey set in before
			},
			assert.NoError,
			&publishMessageResults{
				NumDeliveries: 2,
			},
			[]uuid.UUID{ /* filled in before */ },
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				messages := expectMessages(t, ctx, tx, 0, 2)
				deliveries1 := expectDeliveries(t, ctx, tx, 0, 2)
				deliveries2 := expectDeliveries(t, ctx, tx, 1, 2)
				assert.Equal(t, xID(t, &ent.Delivery{}, 0), deliveries1[0].ID)
				assert.Equal(t, xID(t, &ent.Delivery{}, 1), deliveries2[0].ID)
				assert.Equal(
					t,
					xID(t, &ent.Subscription{}, 0),
					deliveries1[0].Edges.Subscription.ID,
				)
				assert.Equal(
					t,
					xID(t, &ent.Subscription{}, 1),
					deliveries2[0].Edges.Subscription.ID,
				)
				assert.Equal(
					t,
					xID(t, &ent.Subscription{}, 0),
					deliveries1[1].Edges.Subscription.ID,
				)
				assert.Equal(
					t,
					xID(t, &ent.Subscription{}, 1),
					deliveries2[1].Edges.Subscription.ID,
				)
				assert.NotEqual(t, json.RawMessage(`{}`), messages[0].Payload)
				assert.Equal(t, json.RawMessage(`{}`), messages[1].Payload)
				assert.Equal(t, messages[0].ID, deliveries1[0].Edges.Message.ID)
				assert.Equal(t, messages[0].ID, deliveries2[0].Edges.Message.ID)
				assert.Equal(t, messages[1].ID, deliveries1[1].Edges.Message.ID)
				assert.Equal(t, messages[1].ID, deliveries2[1].Edges.Message.ID)
				assert.Equal(t, deliveries1[0].ID, deliveries1[1].Edges.NotBefore.ID)
				assert.Equal(t, deliveries2[0].ID, deliveries2[1].Edges.NotBefore.ID)
			},
		},
		{
			// publish to a topic with ordered and un-ordered subs
			"mixed ordered sub",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub1 := createSubscription(
					t,
					ctx,
					tx,
					topic,
					0,
					func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
						return sc.SetOrderedDelivery(true)
					},
				)
				sub2 := createSubscription(t, ctx, tx, topic, 1)
				// create an earlier delivery, new message should be "after" it
				msg := createMessage(
					t,
					ctx,
					tx,
					topic,
					0,
					func(mc *ent.MessageCreate) *ent.MessageCreate {
						return mc.SetOrderKey(t.Name())
					},
				)
				createDelivery(t, ctx, tx, sub1, msg, 0)
				createDelivery(t, ctx, tx, sub2, msg, 1)
				tt.params.TopicName = topic.Name
				tt.params.OrderKey = t.Name()
				tt.expectPublishNotify = append(tt.expectPublishNotify, sub1.ID, sub2.ID)
			},
			PublishMessageParams{
				// TopicName set in before
				Payload: json.RawMessage(`{}`),
				// OrderKey set in before
			},
			assert.NoError,
			&publishMessageResults{
				NumDeliveries: 2,
			},
			[]uuid.UUID{ /* filled in before */ },
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				messages := expectMessages(t, ctx, tx, 0, 2)
				deliveries1 := expectDeliveries(t, ctx, tx, 0, 2)
				deliveries2 := expectDeliveries(t, ctx, tx, 1, 2)
				assert.Equal(t, xID(t, &ent.Delivery{}, 0), deliveries1[0].ID)
				assert.Equal(t, xID(t, &ent.Delivery{}, 1), deliveries2[0].ID)
				assert.Equal(
					t,
					xID(t, &ent.Subscription{}, 0),
					deliveries1[0].Edges.Subscription.ID,
				)
				assert.Equal(
					t,
					xID(t, &ent.Subscription{}, 1),
					deliveries2[0].Edges.Subscription.ID,
				)
				assert.Equal(
					t,
					xID(t, &ent.Subscription{}, 0),
					deliveries1[1].Edges.Subscription.ID,
				)
				assert.Equal(
					t,
					xID(t, &ent.Subscription{}, 1),
					deliveries2[1].Edges.Subscription.ID,
				)
				assert.NotEqual(t, json.RawMessage(`{}`), messages[0].Payload)
				assert.Equal(t, json.RawMessage(`{}`), messages[1].Payload)
				assert.Equal(t, messages[0].ID, deliveries1[0].Edges.Message.ID)
				assert.Equal(t, messages[0].ID, deliveries2[0].Edges.Message.ID)
				assert.Equal(t, messages[1].ID, deliveries1[1].Edges.Message.ID)
				assert.Equal(t, messages[1].ID, deliveries2[1].Edges.Message.ID)
				assert.Equal(t, deliveries1[0].ID, deliveries1[1].Edges.NotBefore.ID)
				assert.Nil(t, deliveries2[1].Edges.NotBefore)
			},
		},
		{
			"filtered sub, match",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(
					t,
					ctx,
					tx,
					topic,
					0,
					func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
						return sc.SetMessageFilter("attributes:deliver")
					},
				)
				tt.params.TopicName = topic.Name
				tt.params.OrderKey = t.Name()
				tt.expectPublishNotify = append(tt.expectPublishNotify, sub.ID)
			},
			PublishMessageParams{
				Payload:    json.RawMessage(`{}`),
				Attributes: map[string]string{"deliver": ""},
			},
			assert.NoError,
			&publishMessageResults{
				NumDeliveries: 1,
			},
			nil,
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				messages := expectMessages(t, ctx, tx, 0, 1)
				deliveries := expectDeliveries(t, ctx, tx, 0, 1)
				assert.Equal(t, json.RawMessage(`{}`), messages[0].Payload)
				assert.Equal(t, messages[0].ID, deliveries[0].Edges.Message.ID)
			},
		},
		{
			"filtered sub, non-match",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				createSubscription(
					t,
					ctx,
					tx,
					topic,
					0,
					func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
						return sc.SetMessageFilter("attributes:deliver")
					},
				)
				tt.params.TopicName = topic.Name
				tt.params.OrderKey = t.Name()
			},
			PublishMessageParams{
				Payload:    json.RawMessage(`{}`),
				Attributes: map[string]string{"skip": ""},
			},
			assert.NoError,
			&publishMessageResults{
				NumDeliveries: 0,
			},
			nil,
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				messages := expectMessages(t, ctx, tx, 0, 1)
				expectDeliveries(t, ctx, tx, 0, 0)
				assert.Equal(t, json.RawMessage(`{}`), messages[0].Payload)
			},
		},
		{
			// error handling: publish to a topic that doesn't exist, by name
			"no topic by name",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				tt.params.TopicName = uuid.NewString()
				createTopic(t, ctx, tx, 0)
			},
			PublishMessageParams{
				// TopicName set in before
				Payload: json.RawMessage(`{}`),
			},
			func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, ErrNotFound)
			},
			nil,
			[]uuid.UUID{
				// zero: expect no sub to receive a message
				{},
			},
			nil,
		},
		{
			// error handling: publish to a topic that doesn't exist, by id
			"no topic by id",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				id := uuid.New()
				tt.params.TopicID = &id
				createTopic(t, ctx, tx, 0)
			},
			PublishMessageParams{
				// TopicID set in before
				Payload: json.RawMessage(`{}`),
			},
			func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, ErrNotFound)
			},
			nil,
			[]uuid.UUID{
				// zero: expect no sub to receive a message
				{},
			},
			nil,
		},
		{
			// error handling: publish to a topic that doesn't exist, by id+name
			"no topic by id+name",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				tt.params.TopicName = t.Name()
				id := uuid.New()
				tt.params.TopicID = &id
				// create topics that match name, id, but not both
				createTopic(t, ctx, tx, 0)
				createTopic(t, ctx, tx, 1, func(tc *ent.TopicCreate) *ent.TopicCreate {
					return tc.SetID(id).SetName(uuid.NewString())
				})
			},
			PublishMessageParams{
				// TopicName and TopicID set in before
				Payload: json.RawMessage(`{}`),
			},
			func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, ErrNotFound)
			},
			nil,
			[]uuid.UUID{
				// zero: expect no sub to receive a message
				{},
			},
			nil,
		},
	}
	client := enttest.ClientForTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			pubNotifies := map[uuid.UUID]chan struct{}{}
			// have to make this indirect as the expected notify may be generated in
			// before
			defer func() {
				for id, n := range pubNotifies {
					CancelPublishAwaiter(id, n)
				}
			}()
			assert.NoError(
				t,
				client.DoCtxTx(t.Context(), nil, func(ctx context.Context, tx *ent.Tx) error {
					if tt.before != nil {
						tt.before(t, ctx, tx, &tt)
					}
					for _, id := range tt.expectPublishNotify {
						pubNotifies[id] = PublishAwaiter(id)
					}
					a := NewPublishMessage(tt.params)
					tt.assertion(t, a.Execute(ctx, tx))
					// TODO: if err, verify no message
					// TODO: if no error, verify message
					results, ok := a.Results()
					if tt.results != nil {
						require.NotNil(t, a.results)
						assert.True(t, ok)
						assert.Equal(t, tt.results.NumDeliveries, a.results.NumDeliveries)
						assert.Equal(t, tt.results.NumDeliveries, results.NumDeliveries)
						assert.NotZero(t, results.ID)

						m, err := tx.Message.Get(ctx, results.ID)
						require.NoError(t, err)
						assert.Equal(t, m.Payload, tt.params.Payload)
						assert.Equal(t, m.Attributes, tt.params.Attributes)
						d, err := m.QueryDeliveries().All(ctx)
						require.NoError(t, err)
						assert.Len(t, d, results.NumDeliveries)
					} else {
						assert.Nil(t, a.results)
						assert.False(t, ok)
						// TODO: verify no deliveries created
					}
					if tt.after != nil {
						tt.after(t, ctx, tx, &tt)
					}
					return nil
				}),
			)
			for id, n := range pubNotifies {
				// if we get a watch-anything expect, we expect nothing to have happened
				if id != uuid.Nil {
					assertClosed(t, n)
				} else {
					assertOpenEmpty(t, n)
				}
			}
		})
	}
}
