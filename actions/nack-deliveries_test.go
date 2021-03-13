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

func TestNackDeliveries_Execute(t *testing.T) {
	type test struct {
		name      string
		before    func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		params    NackDeliveriesParams
		assertion assert.ErrorAssertionFunc
		results   *nackDeliveriesResults
	}
	tests := []test{
		{
			"no-op",
			nil,
			NackDeliveriesParams{},
			assert.NoError,
			&nackDeliveriesResults{0},
		},
		{
			"nack one",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				msg := createMessage(t, ctx, tx, topic, 0)
				delivery := createDelivery(t, ctx, tx, sub, msg, 0)
				_ = delivery
				tt.params = NackDeliveriesParams{
					[]uuid.UUID{xID(t, &ent.Delivery{}, 0)},
				}
			},
			NackDeliveriesParams{ /* generated in before */ },
			assert.NoError,
			&nackDeliveriesResults{
				numNacked: 1,
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
				tt.params = NackDeliveriesParams{
					[]uuid.UUID{xID(t, &ent.Delivery{}, 1)},
				}
			},
			NackDeliveriesParams{ /* generated in before */ },
			assert.NoError,
			&nackDeliveriesResults{
				numNacked: 0,
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
				a := NewNackDeliveries(tt.params.ids...)
				tt.assertion(t, a.Execute(ctx, tx))
				assert.Equal(t, tt.results, a.results)
				if tt.results != nil && a.results != nil {
					assert.Equal(t, tt.results.numNacked, a.NumNacked())
				}
				unNacked, err := tx.Delivery.Query().
					Where(
						delivery.IDIn(tt.params.ids...),
						delivery.AttemptAtLTE(time.Now()),
					).Count(ctx)
				assert.NoError(t, err)
				assert.Zero(t, unNacked)
				return nil
			}))
		})
	}
}
