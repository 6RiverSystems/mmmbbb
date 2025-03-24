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

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/enttest"
)

func TestPrunedExpiredDeliveries_Execute(t *testing.T) {
	type test struct {
		name                string
		before              func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		params              PruneCommonParams
		assertion           assert.ErrorAssertionFunc
		expect              *PruneCommonResults
		remainder           int
		expectPublishNotify uuid.UUID
	}
	tests := []test{
		{
			"empty",
			nil,
			PruneCommonParams{
				MinAge:    time.Minute,
				MaxDelete: 1,
			},
			assert.NoError,
			&PruneCommonResults{
				NumDeleted: 0,
			},
			0,
			uuid.UUID{},
		},
		{
			"simple",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				msg := createMessage(t, ctx, tx, topic, 0)
				createDelivery(
					t,
					ctx,
					tx,
					sub,
					msg,
					0,
					func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
						return dc.SetExpiresAt(time.Now().Add(-time.Hour))
					},
				)
			},
			PruneCommonParams{
				MinAge:    time.Minute,
				MaxDelete: 1,
			},
			assert.NoError,
			&PruneCommonResults{
				NumDeleted: 1,
			},
			0,
			uuid.UUID{},
		},
		{
			"wake ordered",
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
				createDelivery(
					t,
					ctx,
					tx,
					sub,
					msg,
					0,
					func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
						return dc.SetExpiresAt(time.Now().Add(-time.Hour))
					},
				)
			},
			PruneCommonParams{
				MinAge:    time.Minute,
				MaxDelete: 1,
			},
			assert.NoError,
			&PruneCommonResults{
				NumDeleted: 1,
			},
			0,
			uuid.UUID{ /* filled in before */ },
		},
		{
			"ignores age limit",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				msg := createMessage(t, ctx, tx, topic, 0)
				createDelivery(
					t,
					ctx,
					tx,
					sub,
					msg,
					0,
					func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
						return dc.SetExpiresAt(time.Now().Add(-time.Minute))
					},
				)
			},
			PruneCommonParams{
				MinAge:    time.Hour,
				MaxDelete: 1,
			},
			assert.NoError,
			&PruneCommonResults{
				NumDeleted: 1,
			},
			0,
			uuid.UUID{},
		},
		{
			"count limit",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				msg := createMessage(t, ctx, tx, topic, 0)
				createDelivery(
					t,
					ctx,
					tx,
					sub,
					msg,
					0,
					func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
						return dc.SetExpiresAt(time.Now().Add(-time.Hour))
					},
				)
				sub2 := createSubscription(t, ctx, tx, topic, 1)
				createDelivery(
					t,
					ctx,
					tx,
					sub2,
					msg,
					1,
					func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
						return dc.SetExpiresAt(time.Now().Add(-time.Hour))
					},
				)
			},
			PruneCommonParams{
				MinAge:    time.Minute,
				MaxDelete: 1,
			},
			assert.NoError,
			&PruneCommonResults{
				NumDeleted: 1,
			},
			1,
			uuid.UUID{},
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
			assert.NoError(
				t,
				client.DoCtxTx(t.Context(), nil, func(ctx context.Context, tx *ent.Tx) error {
					if tt.before != nil {
						tt.before(t, ctx, tx, &tt)
					}
					pubNotify = PublishAwaiter(tt.expectPublishNotify)
					a := NewPruneExpiredDeliveries(tt.params)
					tt.assertion(t, a.Execute(ctx, tx))
					assert.Equal(t, tt.expect, a.results)
					results, ok := a.Results()
					if tt.expect != nil {
						if assert.True(t, ok) {
							assert.Equal(t, tt.expect.NumDeleted, results.NumDeleted)
						}
					} else {
						assert.False(t, ok)
					}

					numRemaining, err := tx.Delivery.Query().Count(ctx)
					assert.NoError(t, err)
					assert.Equal(t, tt.remainder, numRemaining)
					return nil
				}),
			)
			// deleting expired ones should send a notify
			if tt.expectPublishNotify != uuid.Nil {
				assertClosed(t, pubNotify)
			} else {
				assertOpenEmpty(t, pubNotify)
			}
		})
	}
}
