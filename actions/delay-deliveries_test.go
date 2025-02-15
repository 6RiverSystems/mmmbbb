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
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/enttest"
)

func TestDelayDeliveries_Execute(t *testing.T) {
	type test struct {
		name      string
		before    func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		params    DelayDeliveriesParams
		assertion assert.ErrorAssertionFunc
		results   *delayDeliveriesResults
	}
	tests := []test{
		{
			"no-op",
			nil,
			DelayDeliveriesParams{},
			assert.NoError,
			&delayDeliveriesResults{0},
		},
		{
			"delay one",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				msg := createMessage(t, ctx, tx, topic, 0)
				delivery := createDelivery(t, ctx, tx, sub, msg, 0)
				_ = delivery
				tt.params.IDs = []uuid.UUID{xID(t, &ent.Delivery{}, 0)}
			},
			DelayDeliveriesParams{
				/* IDs generated in before */
				Delay: time.Hour,
			},
			assert.NoError,
			&delayDeliveriesResults{
				NumDelayed: 1,
			},
		},
		{
			"id mismatch",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				msg := createMessage(t, ctx, tx, topic, 0)
				delivery := createDelivery(t, ctx, tx, sub, msg, 0)
				_ = delivery
				tt.params.IDs = []uuid.UUID{xID(t, &ent.Delivery{}, 1)}
			},
			DelayDeliveriesParams{
				/* IDs generated in before */
				Delay: time.Hour,
			},
			assert.NoError,
			&delayDeliveriesResults{
				NumDelayed: 0,
			},
		},
	}
	client := enttest.ClientForTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			assert.NoError(t, client.DoCtxTx(t.Context(), nil, func(ctx context.Context, tx *ent.Tx) error {
				if tt.before != nil {
					tt.before(t, ctx, tx, &tt)
				}
				a := NewDelayDeliveries(tt.params)
				tt.assertion(t, a.Execute(ctx, tx))
				assert.Equal(t, tt.results, a.results)
				results, ok := a.Results()
				if tt.results != nil {
					if assert.True(t, ok) {
						assert.Equal(t, tt.results.NumDelayed, results.NumDelayed)
					}
				} else {
					assert.False(t, ok)
				}
				unDelayed, err := tx.Delivery.Query().
					Where(
						delivery.IDIn(tt.params.IDs...),
						delivery.AttemptAtLTE(time.Now()),
					).Count(ctx)
				assert.NoError(t, err)
				assert.Zero(t, unDelayed)
				return nil
			}))
		})
	}
}
