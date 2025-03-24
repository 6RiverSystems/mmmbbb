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

func TestPrunedDeletedSubscriptionDeliveries_Execute(t *testing.T) {
	type test struct {
		name      string
		before    func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		params    PruneCommonParams
		assertion assert.ErrorAssertionFunc
		expect    *PruneCommonResults
		remainder int
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
		},
		{
			"simple",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0).Update().
					SetDeletedAt(time.Now().Add(-time.Hour)).
					ClearLive().
					SaveX(ctx)
				msg := createMessage(t, ctx, tx, topic, 0)
				createDelivery(t, ctx, tx, sub, msg, 0)
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
		},
		{
			"age limit",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0).Update().
					SetDeletedAt(time.Now().Add(-time.Minute)).
					ClearLive().
					SaveX(ctx)
				msg := createMessage(t, ctx, tx, topic, 0)
				createDelivery(t, ctx, tx, sub, msg, 0)
			},
			PruneCommonParams{
				MinAge:    time.Hour,
				MaxDelete: 1,
			},
			assert.NoError,
			&PruneCommonResults{
				NumDeleted: 0,
			},
			1,
		},
		{
			"count limit",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0).Update().
					SetDeletedAt(time.Now().Add(-time.Hour)).
					ClearLive().
					SaveX(ctx)
				msg := createMessage(t, ctx, tx, topic, 0)
				createDelivery(t, ctx, tx, sub, msg, 0)
				sub2 := createSubscription(t, ctx, tx, topic, 1).Update().
					SetDeletedAt(time.Now().Add(-time.Hour)).
					ClearLive().
					SaveX(ctx)
				createDelivery(t, ctx, tx, sub2, msg, 1)
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
		},
	}
	client := enttest.ClientForTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			subMod := SubModifiedAwaiter(uuid.UUID{}, t.Name())
			defer CancelSubModifiedAwaiter(uuid.UUID{}, t.Name(), subMod)
			assert.NoError(
				t,
				client.DoCtxTx(t.Context(), nil, func(ctx context.Context, tx *ent.Tx) error {
					if tt.before != nil {
						tt.before(t, ctx, tx, &tt)
					}
					a := NewPruneDeletedSubscriptionDeliveries(tt.params)
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
			// pruning already deleted ones should never send a notify
			assertOpenEmpty(t, subMod)
		})
	}
}
