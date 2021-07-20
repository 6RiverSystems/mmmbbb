// Copyright (c) 2021 6 River Systems
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

	"github.com/stretchr/testify/assert"

	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/enttest"
)

func TestDeleteTopic_Execute(t *testing.T) {
	type test struct {
		name      string
		before    func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test)
		assertion assert.ErrorAssertionFunc
		expect    *deleteTopicResults
	}
	tests := []test{
		{
			"simple",
			func(t *testing.T, ctx context.Context, tx *ent.Tx, tt *test) {
				createTopic(t, ctx, tx, 0)
			},
			assert.NoError,
			&deleteTopicResults{1},
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
				// can't create a topic in deleted state, so have to create and then
				// update
				topic := createTopic(t, ctx, tx, 0)
				topic.Update().
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
			topicID := xID(t, &ent.Topic{}, 0)
			topicMod := TopicModifiedAwaiter(topicID, nameFor(t, 0))
			defer CancelTopicModifiedAwaiter(topicID, nameFor(t, 0), topicMod)
			assert.NoError(t, client.DoCtxTx(testutils.ContextForTest(t), nil, func(ctx context.Context, tx *ent.Tx) error {
				if tt.before != nil {
					tt.before(t, ctx, tx, &tt)
				}
				a := NewDeleteTopic(nameFor(t, 0))
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
				assertClosed(t, topicMod)
			} else {
				assertOpenEmpty(t, topicMod)
			}
		})
	}
}
