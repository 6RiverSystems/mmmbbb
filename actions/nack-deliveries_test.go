package actions

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		after     func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
	}
	tests := []test{
		{
			"no-op",
			nil,
			NackDeliveriesParams{},
			assert.NoError,
			&nackDeliveriesResults{0, 0},
			nil,
		},
		{
			"nack one",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				msg := createMessage(t, ctx, tx, topic, 0)
				delivery := createDelivery(t, ctx, tx, sub, msg, 0)
				tt.params = NackDeliveriesParams{
					[]uuid.UUID{delivery.ID},
				}
			},
			NackDeliveriesParams{ /* generated in before */ },
			assert.NoError,
			&nackDeliveriesResults{
				numNacked: 1,
			},
			nil,
		},
		{
			"id mismatch",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				sub := createSubscription(t, ctx, tx, topic, 0)
				msg := createMessage(t, ctx, tx, topic, 0)
				createDelivery(t, ctx, tx, sub, msg, 0)
				tt.params = NackDeliveriesParams{
					[]uuid.UUID{xID(t, &ent.Delivery{}, 1)},
				}
			},
			NackDeliveriesParams{ /* generated in before */ },
			assert.NoError,
			&nackDeliveriesResults{
				numNacked: 0,
			},
			nil,
		},
		{
			"deadletter",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				dlTopic := createTopic(t, ctx, tx, 1)
				sub := createSubscription(t, ctx, tx, topic, 0, withDeadLetter(dlTopic, 1))
				createSubscription(t, ctx, tx, dlTopic, 1)
				msg := createMessage(t, ctx, tx, topic, 0)
				delivery := createDelivery(t, ctx, tx, sub, msg, 0, func(dc *ent.DeliveryCreate) *ent.DeliveryCreate {
					return dc.SetAttempts(1)
				})
				tt.params = NackDeliveriesParams{
					[]uuid.UUID{delivery.ID},
				}
			},
			NackDeliveriesParams{ /* generated in before */ },
			assert.NoError,
			&nackDeliveriesResults{
				numNacked:       1,
				numDeadLettered: 1,
			},
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				origDelivery, err := tx.Delivery.Get(ctx, xID(t, &ent.Delivery{}, 0))
				require.NoError(t, err)

				dlDelivery, err := tx.Delivery.Query().
					Where(delivery.SubscriptionID(xID(t, &ent.Subscription{}, 1))).
					Only(ctx)
				require.NoError(t, err)

				assert.Equal(t, origDelivery.MessageID, dlDelivery.MessageID)
				assert.Nil(t, dlDelivery.CompletedAt)
				if assert.NotNil(t, origDelivery.CompletedAt) {
					assert.GreaterOrEqual(t, dlDelivery.AttemptAt.UnixNano(), origDelivery.CompletedAt.UnixNano())
				}
				assert.LessOrEqual(t, dlDelivery.AttemptAt.UnixNano(), time.Now().UnixNano())
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
					assert.Equal(t, tt.results.numDeadLettered, a.NumDeadLettered())
				}
				unNacked, err := tx.Delivery.Query().
					Where(
						delivery.IDIn(tt.params.ids...),
						delivery.AttemptAtLTE(time.Now()),
						delivery.CompletedAtIsNil(),
					).Count(ctx)
				assert.NoError(t, err)
				assert.Zero(t, unNacked)
				numDeadLettered, err := tx.Delivery.Query().
					Where(
						delivery.IDIn(tt.params.ids...),
						delivery.CompletedAtNotNil(),
					).Count(ctx)
				assert.NoError(t, err)
				if tt.results != nil {
					assert.Equal(t, tt.results.numDeadLettered, numDeadLettered)
				} else {
					assert.Zero(t, numDeadLettered)
				}
				if tt.after != nil {
					tt.after(t, ctx, tx, &tt)
				}
				return nil
			}))
		})
	}
}
