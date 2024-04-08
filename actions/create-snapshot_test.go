// Copyright (c) 2024 6 River Systems
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
	"go.6river.tech/mmmbbb/internal/testutil"
)

func TestCreateSnapshot_Execute(t *testing.T) {
	type test struct {
		name      string
		before    func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test, topic *ent.Topic, sub *ent.Subscription)
		params    CreateSnapshotParams
		assertion assert.ErrorAssertionFunc
		expect    *ent.Snapshot
	}
	// an arbitrary reference time in the past to use in setups
	refTime := time.Now().Add(-time.Hour)
	tests := []test{
		{
			"everything acked",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test, topic *ent.Topic, sub *ent.Subscription) {
				// create several already-acked messages in the past
				for offset := 0; offset < 10; offset++ {
					m := createMessage(t, ctx, tx, topic, offset, func(mc *ent.MessageCreate) *ent.MessageCreate {
						return mc.SetPublishedAt(refTime.Add(time.Duration(offset) * time.Second))
					})
					createDelivery(t, ctx, tx, sub, m, offset, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
						return dc.SetCompletedAt(time.Now())
					})
				}
				tt.params.SubscriptionName = sub.Name
				tt.expect.TopicID = topic.ID
				now := time.Now()
				tt.expect.CreatedAt = now
				tt.expect.AckedMessagesBefore = now // nothing un-acked, so defaults to now
				tt.expect.ExpiresAt = now.Add(defaultSnapshotTTL)
			},
			CreateSnapshotParams{
				Name: "the name",
				// SubscriptionName filled by hook
				Labels: map[string]string{"some": "label"},
			},
			assert.NoError,
			&ent.Snapshot{
				Name:   "the name",
				Labels: map[string]string{"some": "label"},
				// more fields filled by hook
			},
		},
		{
			"nothing acked",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test, topic *ent.Topic, sub *ent.Subscription) {
				// create several not-acked messages in the past
				for offset := 0; offset < 10; offset++ {
					m := createMessage(t, ctx, tx, topic, offset, func(mc *ent.MessageCreate) *ent.MessageCreate {
						return mc.SetPublishedAt(refTime.Add(time.Duration(offset) * time.Second))
					})
					createDelivery(t, ctx, tx, sub, m, offset)
				}
				tt.params.SubscriptionName = sub.Name
				tt.expect.TopicID = topic.ID
				now := time.Now()
				tt.expect.CreatedAt = now
				// oldest un-acked is offset zero
				tt.expect.AckedMessagesBefore = refTime
				tt.expect.ExpiresAt = now.Add(defaultSnapshotTTL)
			},
			CreateSnapshotParams{
				Name:   "the name",
				Labels: map[string]string{"another": "value"},
			},
			assert.NoError,
			&ent.Snapshot{
				Name:   "the name",
				Labels: map[string]string{"another": "value"},
			},
		},
		{
			"partial ack",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test, topic *ent.Topic, sub *ent.Subscription) {
				// ack even messages, so the first un-acked is the first odd one, and
				// thus the zero message doesn't appear in the override list
				for offset := 0; offset < 10; offset++ {
					m := createMessage(t, ctx, tx, topic, offset, func(mc *ent.MessageCreate) *ent.MessageCreate {
						return mc.SetPublishedAt(refTime.Add(time.Duration(offset) * time.Second))
					})
					createDelivery(t, ctx, tx, sub, m, offset, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
						if offset%2 == 0 {
							return dc.SetCompletedAt(time.Now())
						}
						return dc
					})
					if offset%2 == 0 && offset > 0 {
						tt.expect.AckedMessageIDs = append(tt.expect.AckedMessageIDs, m.ID)
					}
				}
				tt.params.SubscriptionName = sub.Name
				tt.expect.TopicID = topic.ID
				now := time.Now()
				tt.expect.CreatedAt = now
				// oldest un-acked is offset one
				tt.expect.AckedMessagesBefore = refTime.Add(time.Second)
				tt.expect.ExpiresAt = now.Add(defaultSnapshotTTL)
			},
			CreateSnapshotParams{
				Name:   "the name",
				Labels: map[string]string{"another": "value"},
			},
			assert.NoError,
			&ent.Snapshot{
				Name:   "the name",
				Labels: map[string]string{"another": "value"},
			},
		},
	}
	client := enttest.ClientForTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			assert.NoError(t, client.DoCtxTx(testutil.Context(t), nil, func(ctx context.Context, tx *ent.Tx) error {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				if tt.before != nil {
					tt.before(t, ctx, tx, &tt, topic, sub)
				}
				a := NewCreateSnapshot(tt.params)
				start := time.Now()
				tt.assertion(t, a.Execute(ctx, tx))
				deltaT := time.Since(start)
				if tt.expect != nil {
					assert.NotNil(t, a.results)
					results, ok := a.Results()
					assert.True(t, ok)
					assert.Equal(t, tt.expect.TopicID, results.TopicID)
					if tt.expect.ID != uuid.Nil {
						assert.Equal(t, tt.expect.ID, results.SnapshotID)
						assert.Equal(t, tt.expect.ID, results.Snapshot.ID)
					} else {
						assert.Equal(t, results.Snapshot.ID, results.SnapshotID)
						assert.NotEqual(t, results.SnapshotID, uuid.Nil)
					}
					checkSnapEqual(t, tt.expect, results.Snapshot, deltaT)

					snap, err := tx.Snapshot.Get(ctx, results.SnapshotID)
					assert.NoError(t, err)
					checkSnapEqual(t, results.Snapshot, snap, deltaT)
				} else {
					assert.Nil(t, a.results)
				}
				return nil
			}))
		})
	}
}
