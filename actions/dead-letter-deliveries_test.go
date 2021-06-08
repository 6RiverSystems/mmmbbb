package actions

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/enttest"
	"go.6river.tech/mmmbbb/ent/subscription"
)

func TestDeadLetterDeliveries_Execute(t *testing.T) {
	type test struct {
		name      string
		before    func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		params    DeadLetterDeliveriesParams
		assertion assert.ErrorAssertionFunc
		results   *DeadLetterDeliveriesResults
	}
	tests := []test{
		{
			"no-op",
			nil,
			DeadLetterDeliveriesParams{
				MaxDeliveries: 1,
			},
			assert.NoError,
			&DeadLetterDeliveriesResults{0},
		},
		{
			"deadletter one",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				dlTopic := createTopic(t, ctx, tx, 1)
				sub := createSubscription(t, ctx, tx, topic, 0, withDeadLetter(dlTopic, 1))
				createSubscription(t, ctx, tx, dlTopic, 1)
				msg := createMessage(t, ctx, tx, topic, 0)
				createDelivery(t, ctx, tx, sub, msg, 0, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetAttempts(1)
				})
			},
			DeadLetterDeliveriesParams{
				MaxDeliveries: 1,
			},
			assert.NoError,
			&DeadLetterDeliveriesResults{
				numDeadLettered: 1,
			},
		},
		{
			"deadletter one of two",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				dlTopic := createTopic(t, ctx, tx, 1)
				sub := createSubscription(t, ctx, tx, topic, 0, withDeadLetter(dlTopic, 1))
				createSubscription(t, ctx, tx, dlTopic, 1)
				msg := createMessage(t, ctx, tx, topic, 0)
				createDelivery(t, ctx, tx, sub, msg, 0, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetAttempts(1)
				})
				createDelivery(t, ctx, tx, sub, msg, 1, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetAttempts(1)
				})
			},
			DeadLetterDeliveriesParams{
				MaxDeliveries: 1,
			},
			assert.NoError,
			&DeadLetterDeliveriesResults{
				numDeadLettered: 1,
			},
		},
		{
			"ignore attempts below maxAttempts",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				dlTopic := createTopic(t, ctx, tx, 1)
				sub := createSubscription(t, ctx, tx, topic, 0, withDeadLetter(dlTopic, 2))
				createSubscription(t, ctx, tx, dlTopic, 1)
				msg := createMessage(t, ctx, tx, topic, 0)
				createDelivery(t, ctx, tx, sub, msg, 0, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetAttempts(1)
				})
			},
			DeadLetterDeliveriesParams{
				MaxDeliveries: 1,
			},
			assert.NoError,
			&DeadLetterDeliveriesResults{
				numDeadLettered: 0,
			},
		},
		{
			"ignore attempts exceeded but attemptAt not",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				dlTopic := createTopic(t, ctx, tx, 1)
				sub := createSubscription(t, ctx, tx, topic, 0, withDeadLetter(dlTopic, 1))
				createSubscription(t, ctx, tx, dlTopic, 1)
				msg := createMessage(t, ctx, tx, topic, 0)
				createDelivery(t, ctx, tx, sub, msg, 0, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetAttempts(1).SetAttemptAt(time.Now().Add(time.Hour))
				})
			},
			DeadLetterDeliveriesParams{
				MaxDeliveries: 1,
			},
			assert.NoError,
			&DeadLetterDeliveriesResults{
				numDeadLettered: 0,
			},
		},
		{
			"deadletter one to multiple",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				dlTopic := createTopic(t, ctx, tx, 1)
				sub := createSubscription(t, ctx, tx, topic, 0, withDeadLetter(dlTopic, 1))
				createSubscription(t, ctx, tx, dlTopic, 1)
				createSubscription(t, ctx, tx, dlTopic, 2)
				msg := createMessage(t, ctx, tx, topic, 0)
				createDelivery(t, ctx, tx, sub, msg, 0, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetAttempts(1)
				})
			},
			DeadLetterDeliveriesParams{
				MaxDeliveries: 1,
			},
			assert.NoError,
			&DeadLetterDeliveriesResults{
				numDeadLettered: 1,
			},
		},
		{
			"deadletter one to none",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				dlTopic := createTopic(t, ctx, tx, 1)
				sub := createSubscription(t, ctx, tx, topic, 0, withDeadLetter(dlTopic, 1))
				msg := createMessage(t, ctx, tx, topic, 0)
				createDelivery(t, ctx, tx, sub, msg, 0, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetAttempts(1)
				})
			},
			DeadLetterDeliveriesParams{
				MaxDeliveries: 1,
			},
			assert.NoError,
			&DeadLetterDeliveriesResults{
				numDeadLettered: 1,
			},
		},
	}
	client := enttest.ClientForTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			assert.NoError(t, client.DoCtxTx(testutils.ContextForTest(t), nil, func(ctx context.Context, tx *ent.Tx) error {
				if tt.before != nil {
					tt.before(t, ctx, tx, &tt)
				}
				countBefore, err := tx.Delivery.Query().
					Where(delivery.CompletedAtIsNil()).
					Count(ctx)
				require.NoError(t, err)
				a := NewDeadLetterDeliveries(tt.params)
				tt.assertion(t, a.Execute(ctx, tx))
				assert.Equal(t, tt.results, a.results)
				if tt.results != nil {
					assert.True(t, a.HasResults())
					if a.results != nil {
						assert.Equal(t, tt.results.numDeadLettered, a.NumDeadLettered())
					}
				}
				subID := xID(t, &ent.Subscription{}, 0)
				remaining, err := tx.Delivery.Query().
					Where(
						delivery.SubscriptionID(subID),
						delivery.CompletedAtIsNil(),
					).Count(ctx)
				assert.NoError(t, err)
				if tt.results != nil {
					assert.Equal(t, countBefore-tt.results.numDeadLettered, remaining)
				} else {
					assert.Equal(t, countBefore, remaining)
				}
				countDLSubs, err := tx.Subscription.Query().
					Where(subscription.IDNEQ(subID)).
					Count(ctx)
				assert.NoError(t, err)
				countDLDeliveries, err := tx.Delivery.Query().
					Where(
						delivery.SubscriptionIDNEQ(subID),
						delivery.CompletedAtIsNil(),
					).Count(ctx)
				assert.NoError(t, err)
				if tt.results != nil {
					assert.Equal(t, tt.results.numDeadLettered*countDLSubs, countDLDeliveries)
				} else {
					assert.Zero(t, countDLDeliveries)
				}
				return nil
			}))
		})
	}
}