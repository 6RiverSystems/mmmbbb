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
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"go.6river.tech/gosix/ent/customtypes"
	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/enttest"
)

func TestCreateSubscription_Execute(t *testing.T) {
	type expect struct {
		topicID      uuid.UUID
		subscription ent.Subscription
	}
	type test struct {
		name      string
		before    func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		params    CreateSubscriptionParams
		assertion assert.ErrorAssertionFunc
		expect    *expect
	}
	tests := []test{
		{
			"simple",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				tt.params.TopicName = topic.Name
				tt.params.Name = t.Name()
				tt.expect.topicID = topic.ID
				tt.expect.subscription.Name = t.Name()
			},
			CreateSubscriptionParams{
				TTL:        time.Hour,
				MessageTTL: time.Minute,
			},
			assert.NoError,
			&expect{
				// topicID set in before
				subscription: ent.Subscription{
					// Name set in before
					TTL:        customtypes.Interval(time.Hour),
					MessageTTL: customtypes.Interval(time.Minute),
				},
			},
		},
		{
			"all options",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				tt.params.TopicName = topic.Name
				tt.params.Name = t.Name()
				tt.expect.topicID = topic.ID
				tt.expect.subscription.Name = t.Name()
			},
			CreateSubscriptionParams{
				TTL:             time.Hour,
				MessageTTL:      time.Minute,
				OrderedDelivery: true,
				Labels: map[string]string{
					"xyzzy": "frood",
				},
				Filter: "attributes:x",
			},
			assert.NoError,
			&expect{
				// topicID set in before
				subscription: ent.Subscription{
					// Name set in before
					TTL:             customtypes.Interval(time.Hour),
					MessageTTL:      customtypes.Interval(time.Minute),
					OrderedDelivery: true,
					Labels: map[string]string{
						"xyzzy": "frood",
					},
					MessageFilter: stringPtr("attributes:x"),
				},
			},
		},
		{
			"non-existent topic",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				tt.params.TopicName = t.Name()
				tt.params.Name = t.Name()
			},
			CreateSubscriptionParams{
				TTL:        time.Hour,
				MessageTTL: time.Minute,
			},
			assert.Error,
			nil,
		},
		{
			"deleted topic",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0).Update().
					SetDeletedAt(time.Now()).
					ClearLive().
					SaveX(ctx)
				tt.params.TopicName = topic.Name
				tt.params.Name = t.Name()
			},
			CreateSubscriptionParams{
				TTL:        time.Hour,
				MessageTTL: time.Minute,
			},
			assert.Error,
			nil,
		},
		{
			"fail create duplicate",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				tt.params.TopicName = topic.Name
				tt.params.Name = sub.Name
			},
			CreateSubscriptionParams{
				TTL:        time.Hour,
				MessageTTL: time.Minute,
			},
			func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, ErrExists, i...)
			},
			nil,
		},
		{
			"bad filter",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				tt.params.TopicName = topic.Name
				tt.params.Name = t.Name()
			},
			CreateSubscriptionParams{
				TTL:        time.Hour,
				MessageTTL: time.Minute,
				Filter:     "attribute:x",
			},
			func(tt assert.TestingT, err error, i ...interface{}) bool {
				if !assert.Error(t, err, i...) {
					return false
				}
				return assert.Regexp(t, `invalid.*filter.*unexpected token.*attribute`, err.Error(), i...)
			},
			nil,
		},
	}
	client := enttest.ClientForTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			subMod := SubModifiedAwaiter(uuid.UUID{}, t.Name())
			defer CancelSubModifiedAwaiter(uuid.UUID{}, t.Name(), subMod)
			assert.NoError(t, client.DoCtxTx(testutils.ContextForTest(t), nil, func(ctx context.Context, tx *ent.Tx) error {
				if tt.before != nil {
					tt.before(t, ctx, tx, &tt)
				}
				a := NewCreateSubscription(tt.params)
				tt.assertion(t, a.Execute(ctx, tx))
				if tt.expect != nil {
					assert.NotNil(t, a.results)
					assert.Equal(t, tt.expect.topicID, a.TopicID())
					if tt.expect.subscription.ID != (uuid.UUID{}) {
						assert.Equal(t, tt.expect.subscription.ID, a.SubscriptionID())
						assert.Equal(t, tt.expect.subscription.ID, a.Subscription().ID)
						checkSubEqual(t, &tt.expect.subscription, a.Subscription())
					} else {
						assert.Equal(t, a.Subscription().ID, a.SubscriptionID())
						assert.NotEqual(t, a.SubscriptionID(), uuid.UUID{})
					}

					sub, err := tx.Subscription.Get(ctx, a.SubscriptionID())
					assert.NoError(t, err)
					checkSubEqual(t, a.Subscription(), sub)
				} else {
					assert.Nil(t, a.results)
				}
				return nil
			}))
			if tt.expect != nil {
				assertClosed(t, subMod)
			} else {
				assertOpenEmpty(t, subMod)
			}
		})
	}
}

func stringPtr(s string) *string { return &s }
