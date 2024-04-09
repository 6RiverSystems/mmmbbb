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

func TestSeekSubscriptionToSnapshot_Execute(t *testing.T) {
	type test struct {
		name                string
		beforeSnap          func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		setupSnap           func(*testing.T, *ent.SnapshotCreate) *ent.SnapshotCreate
		afterSnap           func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		params              SeekSubscriptionToSnapshotParams
		assertion           assert.ErrorAssertionFunc
		results             *seekSubscriptionToSnapshotResults
		expectPublishNotify uuid.UUID
		expectAcked         []uuid.UUID
		expectNotAcked      []uuid.UUID

		// harness will fill these in
		topic *ent.Topic
		sub   *ent.Subscription
		snap  *ent.Snapshot
	}
	// an arbitrary reference time in the past to use in setups
	refTime := time.Now().Add(-time.Hour)
	tests := []test{
		{
			"no-op to ref time",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				// create a bunch of messages all before the snapshot time, already acked
				for offset := -10; offset < 0; offset++ {
					m := createMessage(t, ctx, tx, tt.topic, offset, func(m *ent.MessageCreate) *ent.MessageCreate {
						return m.SetPublishedAt(refTime.Add(time.Duration(offset) * time.Second))
					})
					d := createDelivery(t, ctx, tx, tt.sub, m, offset, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
						return dc.SetCompletedAt(refTime)
					})
					tt.expectAcked = append(tt.expectAcked, d.ID)
				}
			},
			func(_ *testing.T, sc *ent.SnapshotCreate) *ent.SnapshotCreate {
				return sc.SetAckedMessagesBefore(refTime)
			},
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				// create some more messages
				for offset := 1; offset <= 10; offset++ {
					m := createMessage(t, ctx, tx, tt.topic, offset, func(m *ent.MessageCreate) *ent.MessageCreate {
						return m.SetPublishedAt(refTime.Add(time.Duration(offset) * time.Second))
					})
					d := createDelivery(t, ctx, tx, tt.sub, m, offset)
					tt.expectNotAcked = append(tt.expectNotAcked, d.ID)
				}
				tt.params.SubscriptionName = tt.sub.Name
				tt.params.SnapshotName = tt.snap.Name
				// no expectPublishNotify, because this is expected to be a no-op
			},
			SeekSubscriptionToSnapshotParams{ /* filled in by hooks */ },
			assert.NoError,
			&seekSubscriptionToSnapshotResults{
				NumAcked:   0,
				NumDeAcked: 0,
			},
			uuid.Nil,
			nil, nil, // filled in by hooks
			nil, nil, nil, // filled in by harness
		},
		{
			"de-ack all to ref time",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				// create a bunch of messages all after the snapshot time, already acked
				for offset := 1; offset <= 10; offset++ {
					m := createMessage(t, ctx, tx, tt.topic, offset, func(m *ent.MessageCreate) *ent.MessageCreate {
						return m.SetPublishedAt(refTime.Add(time.Duration(offset) * time.Second))
					})
					d := createDelivery(t, ctx, tx, tt.sub, m, offset, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
						return dc.SetCompletedAt(refTime)
					})
					// we will seek to the ref time and de-ack these
					tt.expectNotAcked = append(tt.expectNotAcked, d.ID)
				}
			},
			func(_ *testing.T, sc *ent.SnapshotCreate) *ent.SnapshotCreate {
				return sc.SetAckedMessagesBefore(refTime)
			},
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				tt.params.SubscriptionName = tt.sub.Name
				tt.params.SnapshotName = tt.snap.Name
				tt.expectPublishNotify = tt.sub.ID
			},
			SeekSubscriptionToSnapshotParams{ /* filled in by hooks */ },
			assert.NoError,
			&seekSubscriptionToSnapshotResults{
				NumAcked:   0,
				NumDeAcked: 10,
			},
			uuid.Nil,
			nil, nil, // filled in by hooks
			nil, nil, nil, // filled in by harness
		},
		{
			"interleaved state",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				// create mesages before and after the ref time, half of each already
				// acked. we will setup the snapshot to _invert_ the half that comes
				// after the ref time.
				for offset := -10; offset <= 10; offset++ {
					if offset == 0 {
						// avoid testing the edge case, while it has a behavior, it's not
						// defined by the spec and is uninteresting in practice
						continue
					}
					m := createMessage(t, ctx, tx, tt.topic, offset, func(m *ent.MessageCreate) *ent.MessageCreate {
						return m.SetPublishedAt(refTime.Add(time.Duration(offset) * time.Second))
					})
					d := createDelivery(t, ctx, tx, tt.sub, m, offset, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
						if offset%2 == 0 {
							dc = dc.SetCompletedAt(refTime)
						}
						return dc
					})
					if offset < 0 {
						tt.expectAcked = append(tt.expectAcked, d.ID)
					} else if d.CompletedAt != nil {
						// expect acked to be come un-acked and vice versa
						tt.expectNotAcked = append(tt.expectNotAcked, d.ID)
					} else {
						tt.expectAcked = append(tt.expectAcked, d.ID)
					}
				}
			},
			func(t *testing.T, sc *ent.SnapshotCreate) *ent.SnapshotCreate {
				// we set even offsets acked initially, invert that in the snapshots to
				// make odd offsets acked.
				ackedIDs := make([]uuid.UUID, 0, 10/2)
				for offset := 1; offset <= 10; offset += 2 {
					ackedIDs = append(ackedIDs, xID(t, &ent.Message{}, offset))
				}
				return sc.
					SetAckedMessagesBefore(refTime).
					SetAckedMessageIDs(ackedIDs)
			},
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				tt.params.SubscriptionName = tt.sub.Name
				tt.params.SnapshotName = tt.snap.Name
				tt.expectPublishNotify = tt.sub.ID
			},
			SeekSubscriptionToSnapshotParams{ /* filled in by hooks */ },
			assert.NoError,
			&seekSubscriptionToSnapshotResults{
				NumAcked:   5 + 5, // 5 before the threshold, 5 after
				NumDeAcked: 5,     // 5 after the threshold
			},
			uuid.Nil,
			nil, nil, // filled in by hooks
			nil, nil, nil, // filled in by harness
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
				tt.topic = createTopic(t, ctx, tx, 0)
				tt.sub = createSubscription(t, ctx, tx, tt.topic, 0)
				if tt.beforeSnap != nil {
					tt.beforeSnap(t, ctx, tx, &tt)
				}
				tt.snap = createSnapshot(t, ctx, tx, tt.topic, 0, func(sc *ent.SnapshotCreate) *ent.SnapshotCreate {
					return tt.setupSnap(t, sc)
				})
				if tt.afterSnap != nil {
					tt.afterSnap(t, ctx, tx, &tt)
				}
				pubNotify = PublishAwaiter(tt.expectPublishNotify)
				a := NewSeekSubscriptionToSnapshot(tt.params)
				tt.assertion(t, a.Execute(ctx, tx))
				assert.Equal(t, tt.results, a.results)
				params := a.Parameters()
				assert.Equal(t, &tt.sub.ID, params.SubscriptionID)
				assert.Equal(t, tt.sub.Name, params.SubscriptionName)
				assert.Equal(t, &tt.snap.ID, params.SnapshotID)
				assert.Equal(t, tt.snap.Name, params.SnapshotName)
				results, ok := a.Results()
				assert.Equal(t, tt.results != nil, ok)
				if tt.results != nil && a.results != nil {
					assert.Equal(t, tt.results.NumAcked, results.NumAcked)
					assert.Equal(t, tt.results.NumDeAcked, results.NumDeAcked)
				}
				expectAckState(t, ctx, tx, tt.expectAcked, true)
				expectAckState(t, ctx, tx, tt.expectNotAcked, false)
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
