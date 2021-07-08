package actions

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"go.6river.tech/gosix/testutils"
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
				for i := 0; i < 10; i++ {
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
			&seekSubscriptionToTimeResults{numAcked: 10, numDeAcked: 0},
			uuid.UUID{},
			nil, nil, nil, nil,
		},
		{
			"simple replay",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				begin := time.Now()
				tt.params.ID = &tt.sub.ID
				tt.expectPublishNotify = tt.sub.ID
				for i := 0; i < 10; i++ {
					m := createMessage(t, ctx, tx, tt.topic, i)
					d := createDelivery(t, ctx, tx, tt.sub, m, i, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
						return dc.SetCompletedAt(begin)
					})
					tt.expectNotAcked = append(tt.expectNotAcked, d.ID)
				}
				// need to ensure this is strictly before all our deliveries
				tt.params.Time = begin.Add(-time.Millisecond)
			},
			// lots of details here will be filled in by the before hook
			SeekSubscriptionToTimeParams{},
			assert.NoError,
			&seekSubscriptionToTimeResults{numAcked: 0, numDeAcked: 10},
			uuid.UUID{},
			nil, nil, nil, nil,
		},
		{
			"no-op purge",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				tt.params.ID = &tt.sub.ID
				// don't set tt.expectPublishNotify -- we expect no changes, so we don't
				// expect any notifications
				for i := 0; i < 10; i++ {
					m := createMessage(t, ctx, tx, tt.topic, i)
					d := createDelivery(t, ctx, tx, tt.sub, m, i, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
						return dc.SetCompletedAt(time.Now())
					})
					tt.expectAcked = append(tt.expectAcked, d.ID)
				}
				// need to refresh this so it's >= our deliveries
				tt.params.Time = time.Now()
			},
			// lots of details here will be filled in by the before hook
			SeekSubscriptionToTimeParams{},
			assert.NoError,
			&seekSubscriptionToTimeResults{numAcked: 0, numDeAcked: 0},
			uuid.UUID{},
			nil, nil, nil, nil,
		},
		{
			"no-op replay",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				begin := time.Now()
				tt.params.ID = &tt.sub.ID
				// don't set tt.expectPublishNotify -- we expect no changes, so we don't
				// expect any notifications
				for i := 0; i < 10; i++ {
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
			&seekSubscriptionToTimeResults{numAcked: 0, numDeAcked: 0},
			uuid.UUID{},
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
			assert.NoError(t, client.DoCtxTx(testutils.ContextForTest(t), nil, func(ctx context.Context, tx *ent.Tx) error {
				tt.topic = createTopic(t, ctx, tx, 0)
				tt.sub = createSubscription(t, ctx, tx, tt.topic, 0)
				if tt.before != nil {
					tt.before(t, ctx, tx, &tt)
				}
				pubNotify = PublishAwaiter(tt.expectPublishNotify)
				a := NewSeekSubscriptionToTime(tt.params)
				tt.assertion(t, a.Execute(ctx, tx))
				assert.Equal(t, tt.results, a.results)
				jsonParams := a.Parameters()
				assert.Equal(t, &tt.sub.ID, jsonParams["id"])
				assert.Equal(t, tt.sub.Name, jsonParams["name"])
				assert.Equal(t, tt.params.Time, jsonParams["time"])
				assert.Equal(t, tt.results != nil, a.HasResults())
				if tt.results != nil && a.results != nil {
					assert.Equal(t, tt.results.numAcked, a.NumAcked())
					assert.Equal(t, tt.results.numDeAcked, a.NumDeAcked())
					jsonResults := a.Results()
					assert.Equal(t, tt.results.numAcked, jsonResults["numAcked"])
					assert.Equal(t, tt.results.numDeAcked, jsonResults["numDeAcked"])
				}
				expectAckState(t, ctx, tx, tt.expectAcked, true)
				expectAckState(t, ctx, tx, tt.expectNotAcked, false)
				return nil
			}))
			if tt.expectPublishNotify != (uuid.UUID{}) {
				assertClosed(t, pubNotify)
			} else {
				assertOpenEmpty(t, pubNotify)
			}
		})
	}
}

func expectAckState(t *testing.T, ctx context.Context, tx *ent.Tx, ids []uuid.UUID, acked bool) bool {
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
