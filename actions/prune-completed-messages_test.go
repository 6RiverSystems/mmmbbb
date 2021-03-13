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
)

func TestPrunedCompletedMessages_Execute(t *testing.T) {
	type test struct {
		name      string
		before    func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		params    PruneCommonParams
		assertion assert.ErrorAssertionFunc
		expect    *PruneCommonResults
		remainder int
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
			0,
		},
		{
			"simple",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				createMessage(t, ctx, tx, topic, 0, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetPublishedAt(time.Now().Add(-time.Hour))
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
			0,
		},
		{
			"age limit",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				createMessage(t, ctx, tx, topic, 0, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetPublishedAt(time.Now().Add(-time.Minute))
				})
			},
			PruneCommonParams{
				MinAge:    time.Hour,
				MaxDelete: 1,
			},
			assert.NoError,
			&PruneCommonResults{
				NumDeleted: 0,
			},
			1,
		},
		{
			"count limit",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				topic := createTopic(t, ctx, tx, 0)
				createMessage(t, ctx, tx, topic, 0, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetPublishedAt(time.Now().Add(-time.Minute))
				})
				createMessage(t, ctx, tx, topic, 1, func(mc *ent.MessageCreate) *ent.MessageCreate {
					return mc.SetPublishedAt(time.Now().Add(-time.Minute))
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
			1,
		},
	}
	client := enttest.ClientForTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			subMod := SubModifiedAwaiter(uuid.UUID{}, t.Name())
			defer CancelSubModifiedAwaiter(uuid.UUID{}, t.Name(), subMod)
			assert.NoError(t, client.DoCtxTx(testutils.ContextForTest(t), nil, func(ctx context.Context, tx *ent.Tx) error {
				if tt.before != nil {
					tt.before(t, ctx, tx, &tt)
				}
				a := NewPruneCompletedMessages(tt.params)
				tt.assertion(t, a.Execute(ctx, tx))
				assert.Equal(t, tt.expect, a.results)
				if tt.expect != nil {
					assert.True(t, a.HasResults())
					assert.Equal(t, tt.expect.NumDeleted, a.NumDeleted())
				}

				numRemaining, err := tx.Message.Query().Count(ctx)
				assert.NoError(t, err)
				assert.Equal(t, tt.remainder, numRemaining)
				return nil
			}))
			// pruning already deleted ones should never send a notify
			assertOpenEmpty(t, subMod)
		})
	}
}
