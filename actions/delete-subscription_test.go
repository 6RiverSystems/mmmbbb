package actions

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/enttest"
)

func TestDeleteSubscription_Execute(t *testing.T) {
	type test struct {
		name      string
		before    func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		assertion assert.ErrorAssertionFunc
		expect    *deleteSubscriptionResults
	}
	tests := []test{
		{
			"simple",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				createSubscription(t, ctx, tx, topic, 0)
			},
			assert.NoError,
			&deleteSubscriptionResults{1},
		},
		{
			"non-existent",
			nil,
			func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, ErrNotFound)
			},
			nil,
		},
		{
			"already deleted",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				createSubscription(t, ctx, tx, topic, 0).Update().
					SetDeletedAt(time.Now()).
					ClearLive().
					ExecX(ctx)
			},
			func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, ErrNotFound)
			},
			nil,
		},
	}
	client := enttest.ClientForTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			subID := xID(t, &ent.Subscription{}, 0)
			subMod := SubModifiedAwaiter(subID, nameFor(t, 0))
			defer CancelSubModifiedAwaiter(subID, nameFor(t, 0), subMod)
			assert.NoError(t, client.DoCtxTx(testutils.ContextForTest(t), nil, func(ctx context.Context, tx *ent.Tx) error {
				if tt.before != nil {
					tt.before(t, ctx, tx, &tt)
				}
				a := NewDeleteSubscription(nameFor(t, 0))
				tt.assertion(t, a.Execute(ctx, tx))
				assert.Equal(t, tt.expect, a.results)
				if tt.expect != nil {
					assert.True(t, a.HasResults())
					assert.Equal(t, tt.expect.numDeleted, a.NumDeleted())
				} else {
					assert.Nil(t, a.results)
				}
				return nil
			}))
			if tt.expect != nil && tt.expect.numDeleted > 0 {
				assertClosed(t, subMod)
			} else {
				assertOpenEmpty(t, subMod)
			}
		})
	}
}
