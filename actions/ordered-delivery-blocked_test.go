// Copyright (c) 2026 6 River Systems
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
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/enttest"
)

// TestOrderedDelivery_Blocked walks an ordered subscription through the actions
// that change which deliveries are blocked on their predecessor.
func TestOrderedDelivery_Blocked(t *testing.T) {
	client := enttest.ClientForTest(t)
	ctx := t.Context()
	// for readable failure messages
	var ids []uuid.UUID
	topic := createTopicClient(t, ctx, client, 0)
	sub := createSubscriptionClient(t, ctx, client, topic, 0,
		func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
			return sc.SetOrderedDelivery(true)
		},
	)

	publish := func() *ent.Delivery {
		t.Helper()
		a := NewPublishMessage(PublishMessageParams{
			TopicID:  &topic.ID,
			Payload:  json.RawMessage(`{}`),
			OrderKey: t.Name(),
		})
		require.NoError(t, client.DoCtxTx(ctx, nil, a.Execute))
		r, ok := a.Results()
		require.True(t, ok)
		d, err := client.Message.GetX(ctx, r.ID).QueryDeliveries().Only(ctx)
		require.NoError(t, err)
		return d
	}
	assertBlocked := func(expected map[uuid.UUID]bool) {
		t.Helper()
		for id, blocked := range expected {
			d, err := client.Delivery.Get(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, blocked, d.Blocked, "delivery %d", slices.Index(ids, id))
		}
	}
	fetch := func() []uuid.UUID {
		t.Helper()
		a := NewGetSubscriptionMessages(GetSubscriptionMessagesParams{
			ID:          &sub.ID,
			MaxMessages: 10,
			MaxBytes:    1 << 20,
			MaxWait:     time.Nanosecond,
		})
		require.NoError(t, a.ExecuteClient(ctx, client))
		r, _ := a.Results()
		var got []uuid.UUID
		for _, d := range r.Deliveries {
			got = append(got, d.ID)
		}
		return got
	}
	ack := func(id uuid.UUID) {
		t.Helper()
		require.NoError(t, client.DoCtxTx(ctx, nil, NewAckDeliveries(id).Execute))
	}
	seek := func(to time.Time) {
		t.Helper()
		require.NoError(t, client.DoCtxTx(ctx, nil, NewSeekSubscriptionToTime(SeekSubscriptionToTimeParams{
			ID:   &sub.ID,
			Time: to,
		}).Execute))
	}

	d0, d1, d2 := publish(), publish(), publish()
	ids = []uuid.UUID{d0.ID, d1.ID, d2.ID}
	assert.Equal(t, uuid.Nil, d0.NotBeforeID)
	assert.Equal(t, d0.ID, d1.NotBeforeID)
	assert.Equal(t, d1.ID, d2.NotBeforeID)
	assertBlocked(map[uuid.UUID]bool{d0.ID: false, d1.ID: true, d2.ID: true})

	// only the head is delivered
	assert.Equal(t, []uuid.UUID{d0.ID}, fetch())

	// acking the head unblocks only its direct successor
	ack(d0.ID)
	assertBlocked(map[uuid.UUID]bool{d1.ID: false, d2.ID: true})
	assert.Equal(t, []uuid.UUID{d1.ID}, fetch())

	// seeking back de-acks the head, which blocks its successor again
	seek(d0.PublishedAt.Add(-time.Millisecond))
	assertBlocked(map[uuid.UUID]bool{d0.ID: false, d1.ID: true, d2.ID: true})
	assert.Equal(t, []uuid.UUID{d0.ID}, fetch())

	// seeking forward acks the first two, unblocking the last
	seek(d1.PublishedAt)
	assertBlocked(map[uuid.UUID]bool{d2.ID: false})
	assert.Equal(t, []uuid.UUID{d2.ID}, fetch())

	// publishing after the predecessor is completed is not blocked
	ack(d2.ID)
	d3 := publish()
	ids = append(ids, d3.ID)
	assert.Equal(t, d2.ID, d3.NotBeforeID)
	assertBlocked(map[uuid.UUID]bool{d3.ID: false})

	// expired predecessors unblock their successors when pruned
	d4 := publish()
	ids = append(ids, d4.ID)
	assertBlocked(map[uuid.UUID]bool{d4.ID: true})
	client.Delivery.UpdateOneID(d3.ID).SetExpiresAt(time.Now()).ExecX(ctx)
	assert.Empty(t, fetch())
	require.NoError(t, client.DoCtxTx(ctx, nil,
		NewPruneExpiredDeliveries(PruneCommonParams{MaxDelete: 100}).Execute))
	assertBlocked(map[uuid.UUID]bool{d4.ID: false})
	assert.Equal(t, []uuid.UUID{d4.ID}, fetch())
}
