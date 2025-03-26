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
	"time"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/message"
)

type PruneCompletedMessages struct {
	pruneAction
}

var _ Action[PruneCommonParams, PruneCommonResults] = (*PruneCompletedMessages)(nil)

func NewPruneCompletedMessages(params PruneCommonParams) *PruneCompletedMessages {
	return &PruneCompletedMessages{
		pruneAction: newPruneAction(params),
	}
}

var pruneCompletedMessagesCounter, pruneCompletedMessagesHistogram = pruneMetrics(
	"completed_messages",
)

func (a *PruneCompletedMessages) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(pruneCompletedMessagesHistogram, tx)
	defer timer.Ended()

	// ent doesn't support limit on delete commands (that may be a PostgreSQL
	// extension), so have to do a query-then-delete
	ids, err := tx.Message.Query().
		Where(
			// messages can only be pruned by publish time, not completion time, because
			// we only get here when all the completions have been pruned
			message.PublishedAtLTE(time.Now().Add(-a.params.MinAge)),
			// we rely on deliveries being pruned to then allow messages to be pruned.
			// this being efficient relies on the fixes in
			// https://github.com/ent/ent/pull/3492
			message.Not(message.HasDeliveries()),
		).
		Limit(a.params.MaxDelete).
		IDs(ctx)
	if err != nil {
		return err
	}
	numDeleted, err := tx.Message.Delete().Where(message.IDIn(ids...)).Exec(ctx)
	if err != nil {
		return err
	}

	a.results = &PruneCommonResults{
		NumDeleted: numDeleted,
	}
	timer.Succeeded(func() { pruneCompletedMessagesCounter.Add(float64(numDeleted)) })

	return nil
}
