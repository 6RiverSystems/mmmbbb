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

	"go.6river.tech/gosix/ent/customtypes"
	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/ent"
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
func createSubscriptionClient(
	t *testing.T, ctx context.Context, client *ent.Client, topic *ent.Topic, offset int,
	opts ...func(*ent.SubscriptionCreate) *ent.SubscriptionCreate,
) *ent.Subscription {
	sc := client.Subscription.Create().
		SetID(xID(t, &ent.Subscription{}, offset)).
		SetName(nameFor(t, offset)).
		SetTopic(topic).
		SetExpiresAt(testutils.DeadlineForTest(t)).
		SetTTL(customtypes.FromDuration(time.Minute)).
		SetMessageTTL(customtypes.FromDuration(time.Minute))
	for _, o := range opts {
		sc = o(sc)
	}
	subscription, err := sc.Save(ctx)
	require.NoError(t, err)
	return subscription
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
		SetExpiresAt(testutils.DeadlineForTest(t))
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

func checkNullableIntervalEqual(t *testing.T, expected, actual *customtypes.IntervalNull) {
	if expected == nil || expected.Duration == nil {
		if actual != nil {
			assert.Nil(t, actual.Duration)
		}
	} else {
		assert.NotNil(t, actual)
		if actual != nil {
			assert.Equal(t, expected.Duration, actual.Duration)
		}
	}
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
