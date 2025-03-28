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

func TestSeekSubscriptionToTime_Execute(t *testing.T) {
	type test struct {
		name                string
		before              func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		params              SeekSubscriptionToTimeParams
		assertion           assert.ErrorAssertionFunc
		results             *seekSubscriptionToTimeResults
		expectPublishNotify uuid.UUID
		expectAcked         []uuid.UUID
		expectNotAcked      []uuid.UUID

		// harness will fill these in
		topic *ent.Topic
		sub   *ent.Subscription
	}
	tests := []test{
		{
			"simple purge",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				tt.params.ID = &tt.sub.ID
				tt.expectPublishNotify = tt.sub.ID
				for i := range 10 {
					m := createMessage(t, ctx, tx, tt.topic, i)
					d := createDelivery(t, ctx, tx, tt.sub, m, i)
					tt.expectAcked = append(tt.expectAcked, d.ID)
				}
				// need to refresh this so it's >= our deliveries
				tt.params.Time = time.Now()
			},
			// lots of details here will be filled in by the before hook
			SeekSubscriptionToTimeParams{},
			assert.NoError,
			&seekSubscriptionToTimeResults{NumAcked: 10, NumDeAcked: 0},
			uuid.Nil,
			nil, nil, nil, nil,
		},
		{
			"simple replay",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				begin := time.Now()
				tt.params.ID = &tt.sub.ID
				tt.expectPublishNotify = tt.sub.ID
				for i := range 10 {
					m := createMessage(t, ctx, tx, tt.topic, i)
					d := createDelivery(
						t,
						ctx,
						tx,
						tt.sub,
						m,
						i,
						func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
							return dc.SetCompletedAt(begin)
						},
					)
					tt.expectNotAcked = append(tt.expectNotAcked, d.ID)
				}
				// need to ensure this is strictly before all our deliveries
				tt.params.Time = begin.Add(-time.Millisecond)
			},
			// lots of details here will be filled in by the before hook
			SeekSubscriptionToTimeParams{},
			assert.NoError,
			&seekSubscriptionToTimeResults{NumAcked: 0, NumDeAcked: 10},
			uuid.Nil,
			nil, nil, nil, nil,
		},
		{
			"no-op purge",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				tt.params.ID = &tt.sub.ID
				// don't set tt.expectPublishNotify -- we expect no changes, so we don't
				// expect any notifications
				for i := range 10 {
					m := createMessage(t, ctx, tx, tt.topic, i)
					d := createDelivery(
						t,
						ctx,
						tx,
						tt.sub,
						m,
						i,
						func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
							return dc.SetCompletedAt(time.Now())
						},
					)
					tt.expectAcked = append(tt.expectAcked, d.ID)
				}
				// need to refresh this so it's >= our deliveries
				tt.params.Time = time.Now()
			},
			// lots of details here will be filled in by the before hook
			SeekSubscriptionToTimeParams{},
			assert.NoError,
			&seekSubscriptionToTimeResults{NumAcked: 0, NumDeAcked: 0},
			uuid.Nil,
			nil, nil, nil, nil,
		},
		{
			"no-op replay",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				begin := time.Now()
				tt.params.ID = &tt.sub.ID
				// don't set tt.expectPublishNotify -- we expect no changes, so we don't
				// expect any notifications
				for i := range 10 {
					m := createMessage(t, ctx, tx, tt.topic, i)
					d := createDelivery(t, ctx, tx, tt.sub, m, i)
					tt.expectNotAcked = append(tt.expectNotAcked, d.ID)
				}
				// need to ensure this is strictly before all our deliveries
				tt.params.Time = begin.Add(-time.Millisecond)
			},
			// lots of details here will be filled in by the before hook
			SeekSubscriptionToTimeParams{},
			assert.NoError,
			&seekSubscriptionToTimeResults{NumAcked: 0, NumDeAcked: 0},
			uuid.Nil,
			nil, nil, nil, nil,
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
					tt.topic = createTopic(t, ctx, tx, 0)
					tt.sub = createSubscription(t, ctx, tx, tt.topic, 0)
					if tt.before != nil {
						tt.before(t, ctx, tx, &tt)
					}
					pubNotify = PublishAwaiter(tt.expectPublishNotify)
					a := NewSeekSubscriptionToTime(tt.params)
					tt.assertion(t, a.Execute(ctx, tx))
					assert.Equal(t, tt.results, a.results)
					params := a.Parameters()
					assert.Equal(t, &tt.sub.ID, params.ID)
					assert.Equal(t, tt.sub.Name, params.Name)
					assert.Equal(t, tt.params.Time, params.Time)
					results, ok := a.Results()
					assert.Equal(t, tt.results != nil, ok)
					if tt.results != nil && a.results != nil {
						assert.Equal(t, tt.results.NumAcked, results.NumAcked)
						assert.Equal(t, tt.results.NumDeAcked, results.NumDeAcked)
					}
					expectAckState(t, ctx, tx, tt.expectAcked, true)
					expectAckState(t, ctx, tx, tt.expectNotAcked, false)
					return nil
				}),
			)
			if tt.expectPublishNotify != (uuid.Nil) {
				assertClosed(t, pubNotify)
			} else {
				assertOpenEmpty(t, pubNotify)
			}
		})
	}
}

//
//nolint:unparam
func expectAckState(
	t *testing.T,
	ctx context.Context,
	tx *ent.Tx,
	ids []uuid.UUID,
	acked bool,
) bool {
	numAcked, err := tx.Delivery.Query().
		Where(delivery.IDIn(ids...), delivery.CompletedAtNotNil()).
		Count(ctx)
	if !assert.NoError(t, err) {
		return false
	}
	numNotAcked, err := tx.Delivery.Query().
		Where(delivery.IDIn(ids...), delivery.CompletedAtIsNil()).
		Count(ctx)
	if !assert.NoError(t, err) {
		return false
	}
	if acked {
		return assert.Equal(t, len(ids), numAcked, "expect deliveries to be acked") &&
			assert.Equal(t, 0, numNotAcked, "expect deliveries not to be un-acked")
	} else {
		return assert.Equal(t, len(ids), numNotAcked, "expect deliveries to be un-acked") &&
			assert.Equal(t, 0, numAcked, "expect deliveries not to be acked")
	}
}
