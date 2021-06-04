package actions

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/enttest"
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
				sub := createSubscription(t, ctx, tx, topic, 0)
				msg := createMessage(t, ctx, tx, topic, 0)
				delivery := createDelivery(t, ctx, tx, sub, msg, 0)
				_ = delivery
			},
			DeadLetterDeliveriesParams{
				MaxDeliveries: 1,
			},
			assert.NoError,
			&DeadLetterDeliveriesResults{
				numDeadLettered: 1,
			},
		},
		// TODO: test limit number affected
		// TODO: test ignoring those that haven't exceeded maxAttempts
		// TODO: test ignoring those that have exceeded maxAttempts but not attemptAt
	}
	client := enttest.ClientForTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			assert.NoError(t, client.DoCtxTx(testutils.ContextForTest(t), nil, func(ctx context.Context, tx *ent.Tx) error {
				if tt.before != nil {
					tt.before(t, ctx, tx, &tt)
				}
				a := NewDeadLetterDeliveries(tt.params)
				tt.assertion(t, a.Execute(ctx, tx))
				assert.Equal(t, tt.results, a.results)
				if tt.results != nil {
					assert.True(t, a.HasResults())
					if a.results != nil {
						assert.Equal(t, tt.results.numDeadLettered, a.NumDeadLettered())
					}
				}
				unDelayed, err := tx.Delivery.Query().
					Where(
						// FIXME: delivery.IDIn(tt.params.IDs...),
						delivery.AttemptAtLTE(time.Now()),
					).Count(ctx)
				assert.NoError(t, err)
				assert.Zero(t, unDelayed)
				return nil
			}))
		})
	}
}
