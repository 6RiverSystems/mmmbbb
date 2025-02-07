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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/enttest"
	"go.6river.tech/mmmbbb/internal/testutil"
)

func TestAckDeliveries_Execute(t *testing.T) {
	type test struct {
		name                string
		before              func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		params              AckDeliveriesParams
		assertion           assert.ErrorAssertionFunc
		results             *ackDeliveriesResults
		expectPublishNotify uuid.UUID
	}
	tests := []test{
		{
			"no-op",
			nil,
			AckDeliveriesParams{},
			assert.NoError,
			&ackDeliveriesResults{0},
			uuid.UUID{},
		},
		{
			"ack one",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				msg := createMessage(t, ctx, tx, topic, 0)
				delivery := createDelivery(t, ctx, tx, sub, msg, 0)
				_ = delivery
				tt.params = AckDeliveriesParams{
					[]uuid.UUID{xID(t, &ent.Delivery{}, 0)},
				}
			},
			AckDeliveriesParams{ /* generated in before */ },
			assert.NoError,
			&ackDeliveriesResults{
				NumAcked: 1,
			},
			uuid.UUID{},
		},
		{
			"id mismatch",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				msg := createMessage(t, ctx, tx, topic, 0)
				delivery := createDelivery(t, ctx, tx, sub, msg, 0)
				_ = delivery
				tt.params = AckDeliveriesParams{
					[]uuid.UUID{xID(t, &ent.Delivery{}, 1)},
				}
			},
			AckDeliveriesParams{ /* generated in before */ },
			assert.NoError,
			&ackDeliveriesResults{
				NumAcked: 0,
			},
			uuid.UUID{},
		},
		{
			"ack ordered sends pub notify",
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
				tt.expectPublishNotify = sub.ID
				msg := createMessage(t, ctx, tx, topic, 0)
				delivery := createDelivery(t, ctx, tx, sub, msg, 0)
				_ = delivery
				tt.params = AckDeliveriesParams{
					[]uuid.UUID{xID(t, &ent.Delivery{}, 0)},
				}
			},
			AckDeliveriesParams{ /* generated in before */ },
			assert.NoError,
			&ackDeliveriesResults{
				NumAcked: 1,
			},
			uuid.UUID{ /* generated in before */ },
		},
	}
	client := enttest.ClientForTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			var pubNotify chan struct{}
			// have to make this indirect as the expected notify may be generated in
			// before
			defer func() { CancelPublishAwaiter(tt.expectPublishNotify, pubNotify) }()
			assert.NoError(t, client.DoCtxTx(testutil.Context(t), nil, func(ctx context.Context, tx *ent.Tx) error {
				if tt.before != nil {
					tt.before(t, ctx, tx, &tt)
				}
				pubNotify = PublishAwaiter(tt.expectPublishNotify)
				a := NewAckDeliveries(tt.params.ids...)
				tt.assertion(t, a.Execute(ctx, tx))
				assert.Equal(t, tt.results, a.results)
				results, ok := a.Results()
				assert.Equal(t, tt.results != nil, ok)
				if tt.results != nil {
					assert.Equal(t, *tt.results, results)
				}
				unAcked, err := tx.Delivery.Query().
					Where(
						delivery.IDIn(tt.params.ids...),
						delivery.CompletedAtIsNil(),
					).Count(ctx)
				assert.NoError(t, err)
				assert.Zero(t, unAcked)
				return nil
			}))
			if tt.expectPublishNotify != uuid.Nil {
				assertClosed(t, pubNotify)
			} else {
				assertOpenEmpty(t, pubNotify)
			}
		})
	}
}
