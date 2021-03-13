package actions

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/enttest"
	"go.6river.tech/mmmbbb/ent/subscription"
)

func TestDeleteExpiredSubscriptions_Execute(t *testing.T) {
	type test struct {
		name      string
		before    func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		params    PruneCommonParams
		assertion assert.ErrorAssertionFunc
		expect    *PruneCommonResults
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
		},
		{
			"simple",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				createSubscription(t, ctx, tx, topic, 0, func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
					return sc.SetExpiresAt(time.Now().Add(-time.Hour))
				})
			},
			PruneCommonParams{
				MinAge:    time.Minute,
				MaxDelete: 1,
			},
			assert.NoError,
			&PruneCommonResults{
				NumDeleted: 1,
			},
		},
		{
			// age limit does not apply to subscription expiration!
			"ignores age limit",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				createSubscription(t, ctx, tx, topic, 0, func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
					return sc.SetExpiresAt(time.Now().Add(-time.Minute))
				})
			},
			PruneCommonParams{
				MinAge:    time.Hour,
				MaxDelete: 1,
			},
			assert.NoError,
			&PruneCommonResults{
				NumDeleted: 1,
			},
		},
		{
			"count limit",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				createSubscription(t, ctx, tx, topic, 0, func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
					return sc.SetExpiresAt(time.Now().Add(-time.Hour))
				})
				createSubscription(t, ctx, tx, topic, 1, func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
					return sc.SetExpiresAt(time.Now().Add(-time.Hour))
				})
			},
			PruneCommonParams{
				MinAge:    time.Minute,
				MaxDelete: 1,
			},
			assert.NoError,
			&PruneCommonResults{
				NumDeleted: 1,
			},
		},
	}
	client := enttest.ClientForTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			subMod := SubModifiedAwaiter(uuid.UUID{}, nameFor(t, 0))
			defer CancelSubModifiedAwaiter(uuid.UUID{}, nameFor(t, 0), subMod)
			assert.NoError(t, client.DoCtxTx(testutils.ContextForTest(t), nil, func(ctx context.Context, tx *ent.Tx) error {
				if tt.before != nil {
					tt.before(t, ctx, tx, &tt)
				}
				a := NewDeleteExpiredSubscriptions(tt.params)
				tt.assertion(t, a.Execute(ctx, tx))
				assert.Equal(t, tt.expect, a.results)
				if tt.expect != nil {
					assert.True(t, a.HasResults())
					assert.Equal(t, tt.expect.NumDeleted, a.NumDeleted())

					// deleted subs should match the result expectation
					deleted, err := tx.Subscription.Query().Where(subscription.DeletedAtNotNil()).All(ctx)
					assert.NoError(t, err)
					assert.Len(t, deleted, tt.expect.NumDeleted)
					// make sure they were proper to be deleted
					for _, d := range deleted {
						assert.NotZero(t, d.ExpiresAt)
						assert.Less(t, d.ExpiresAt.UnixNano(), time.Now().UnixNano())
					}
				}
				return nil
			}))
			if tt.expect != nil && tt.expect.NumDeleted > 0 {
				assertClosed(t, subMod)
			} else {
				assertOpenEmpty(t, subMod)
			}
		})
	}
}
