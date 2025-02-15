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
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/internal/sqltypes"
)

func nameFor(t testing.TB, offset int) string {
	return t.Name() + "_" + strconv.Itoa(offset)
}

func createTopic(
	t *testing.T, ctx context.Context, tx *ent.Tx, offset int,
	opts ...func(*ent.TopicCreate) *ent.TopicCreate,
) *ent.Topic {
	return createTopicClient(t, ctx, tx.Client(), offset, opts...)
}

func createTopicClient(
	t *testing.T, ctx context.Context, client *ent.Client, offset int,
	opts ...func(*ent.TopicCreate) *ent.TopicCreate,
) *ent.Topic {
	tc := client.Topic.Create().
		SetID(xID(t, &ent.Topic{}, offset)).
		SetName(nameFor(t, offset))
	for _, o := range opts {
		tc = o(tc)
	}
	topic, err := tc.Save(ctx)
	require.NoError(t, err)
	return topic
}

func createSubscription(
	t *testing.T, ctx context.Context, tx *ent.Tx, topic *ent.Topic, offset int,
	opts ...func(*ent.SubscriptionCreate) *ent.SubscriptionCreate,
) *ent.Subscription {
	return createSubscriptionClient(t, ctx, tx.Client(), topic, offset, opts...)
}

func withDeadLetter(
	dlTopic *ent.Topic,
	maxAttempts int32,
) func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
	return func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
		return sc.SetDeadLetterTopic(dlTopic).SetMaxDeliveryAttempts(maxAttempts)
	}
}

func createSubscriptionClient(
	t *testing.T, ctx context.Context, client *ent.Client, topic *ent.Topic, offset int,
	opts ...func(*ent.SubscriptionCreate) *ent.SubscriptionCreate,
) *ent.Subscription {
	sc := client.Subscription.Create().
		SetID(xID(t, &ent.Subscription{}, offset)).
		SetName(nameFor(t, offset)).
		SetTopic(topic).
		SetExpiresAt(time.Now().Add(time.Minute)).
		SetTTL(sqltypes.Interval(time.Minute)).
		SetMessageTTL(sqltypes.Interval(time.Minute))
	for _, o := range opts {
		sc = o(sc)
	}
	subscription, err := sc.Save(ctx)
	require.NoError(t, err)
	return subscription
}

func createSnapshot(
	t *testing.T, ctx context.Context, tx *ent.Tx, topic *ent.Topic, offset int,
	opts ...func(*ent.SnapshotCreate) *ent.SnapshotCreate,
) *ent.Snapshot {
	return createSnapshotClient(t, ctx, tx.Client(), topic, offset, opts...)
}

func createSnapshotClient(
	t *testing.T, ctx context.Context, client *ent.Client, topic *ent.Topic, offset int,
	opts ...func(*ent.SnapshotCreate) *ent.SnapshotCreate,
) *ent.Snapshot {
	sc := client.Snapshot.Create().
		SetID(xID(t, &ent.Snapshot{}, offset)).
		SetName(nameFor(t, offset)).
		SetTopic(topic).
		SetExpiresAt(time.Now().Add(time.Minute)).
		SetAckedMessagesBefore(time.Now()).
		SetAckedMessageIDs([]uuid.UUID{})
	for _, o := range opts {
		sc = o(sc)
	}
	snap, err := sc.Save(ctx)
	require.NoError(t, err)
	return snap
}

func createMessage(
	t *testing.T, ctx context.Context, tx *ent.Tx, topic *ent.Topic, offset int,
	opts ...func(*ent.MessageCreate) *ent.MessageCreate,
) *ent.Message {
	return createMessageClient(t, ctx, tx.Client(), topic, offset, opts...)
}

func createMessageClient(
	t *testing.T, ctx context.Context, client *ent.Client, topic *ent.Topic, offset int,
	opts ...func(*ent.MessageCreate) *ent.MessageCreate,
) *ent.Message {
	p, err := json.Marshal(t.Name())
	require.NoError(t, err)
	mc := client.Message.Create().
		SetID(xID(t, &ent.Message{}, offset)).
		SetPayload(p).
		SetTopic(topic)
	for _, o := range opts {
		mc = o(mc)
	}
	msg, err := mc.Save(ctx)
	require.NoError(t, err)
	return msg
}

func createDelivery(
	t *testing.T, ctx context.Context, tx *ent.Tx, sub *ent.Subscription, msg *ent.Message, offset int,
	opts ...func(*ent.DeliveryCreate) *ent.DeliveryCreate,
) *ent.Delivery {
	return createDeliveryClient(t, ctx, tx.Client(), sub, msg, offset, opts...)
}

func createDeliveryClient(
	t *testing.T, ctx context.Context, client *ent.Client, sub *ent.Subscription, msg *ent.Message, offset int,
	opts ...func(*ent.DeliveryCreate) *ent.DeliveryCreate,
) *ent.Delivery {
	dc := client.Delivery.Create().
		SetID(xID(t, &ent.Delivery{}, offset)).
		SetMessage(msg).
		SetSubscription(sub).
		SetExpiresAt(time.Now().Add(time.Minute)).
		SetPublishedAt(msg.PublishedAt)
	for _, o := range opts {
		dc = o(dc)
	}
	delivery, err := dc.Save(ctx)
	require.NoError(t, err)
	return delivery
}

var idNS = uuid.New()

func xID(t *testing.T, entityType fmt.Stringer, offset int) uuid.UUID {
	entityTypeName := reflect.ValueOf(entityType).Type().Elem().Name()
	val := t.Name() + "\n" + entityTypeName + "\n" + strconv.Itoa(offset)
	return uuid.NewSHA1(idNS, ([]byte)(val))
}

func checkSubEqual(t *testing.T, expected, actual *ent.Subscription) {
	// ignoring ID because it is random
	assert.Equal(t, expected.Name, actual.Name)
	// ignoring CreatedAt because we don't care, and it depends on time.Now()
	// ignoring ExpiresAt because it depends on time.Now()
	// ignoring the value of DeletedAt because it depends on time.Now()
	if expected.DeletedAt != nil {
		assert.NotNil(t, actual.DeletedAt)
	} else {
		assert.Nil(t, actual.DeletedAt)
	}
	assert.Equal(t, expected.TTL, actual.TTL)
	assert.Equal(t, expected.MessageTTL, actual.MessageTTL)
	assert.Equal(t, expected.OrderedDelivery, actual.OrderedDelivery)
	assert.Equal(t, expected.Labels, actual.Labels)
	checkNullableIntervalEqual(t, expected.MinBackoff, actual.MinBackoff)
	checkNullableIntervalEqual(t, expected.MaxBackoff, actual.MaxBackoff)
	// ignoring Edges
}

func checkSnapEqual(t *testing.T, expected, actual *ent.Snapshot, timeEpsilon time.Duration) {
	// ignoring: ID, TopicID, CreatedAt
	assert.Equal(t, expected.Name, actual.Name)
	assert.WithinDuration(t, expected.ExpiresAt, actual.ExpiresAt, timeEpsilon,
		"ExpiresAt")
	assert.Equal(t, expected.Labels, actual.Labels)
	assert.WithinDuration(t, expected.AckedMessagesBefore, actual.AckedMessagesBefore, timeEpsilon,
		"AckedMessagesBefore")
	// don't care about order here
	assert.ElementsMatch(t, expected.AckedMessageIDs, actual.AckedMessageIDs)
}

//nolint:unparam
func checkNullableIntervalEqual(t *testing.T, expected, actual *sqltypes.Interval) bool {
	return assert.Equal(t, expected, actual)
}

func assertClosed(t *testing.T, c <-chan struct{}) {
	assert.NotNil(t, c)
	select {
	case _, ok := <-c:
		assert.False(t, ok, "receive on expected-closed channel should confirm closed")
	default:
		assert.Fail(t, "unable to receive on expected-closed channel")
	}
}

func assertOpenEmpty(t *testing.T, c <-chan struct{}) {
	assert.NotNil(t, c)
	select {
	case _, ok := <-c:
		if ok {
			assert.Fail(t, "receive on expected-open-empty channel should not receive a value")
		} else {
			assert.Fail(t, "receive on expected-open-empty channel should not confirm closed")
		}
	default:
		// PASS
	}
}
