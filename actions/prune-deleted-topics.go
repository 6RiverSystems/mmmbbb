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

	"github.com/google/uuid"

	"go.6river.tech/gosix/logging"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/topic"
)

type PruneDeletedTopics struct {
	pruneAction
}

var _ Action = (*PruneDeletedTopics)(nil)

func NewPruneDeletedTopics(params PruneCommonParams) *PruneDeletedTopics {
	return &PruneDeletedTopics{
		pruneAction: *newPruneAction(params),
	}
}

var pruneDeletedTopicsCounter, pruneDeletedTopicsHistogram = pruneMetrics("deleted_topics")

func (a *PruneDeletedTopics) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(pruneDeletedTopicsHistogram, tx)
	defer timer.Ended()

	// ent doesn't support limit on delete commands (that may be a PostgreSQL
	// extension), so have to do a query-then-delete
	topics, err := tx.Topic.Query().
		Where(
			topic.DeletedAtLTE(time.Now().Add(-a.params.MinAge)),
			// we rely on subscriptions being pruned to then allow topics to be pruned
			// UPSTREAM: ticket for HasRelationWith efficiency
			topic.Not(topic.HasSubscriptions()),
		).
		Limit(a.params.MaxDelete).
		All(ctx)
	if err != nil {
		return err
	}
	ids := make([]uuid.UUID, len(topics))
	logger := logging.GetLogger("actions/prune-deleted-topics")
	for i, t := range topics {
		ids[i] = t.ID
		logger.Info().
			Str("topicName", t.Name).
			Stringer("topicID", t.ID).
			Time("deletedAt", *t.DeletedAt).
			Msg("pruning deleted topic")
	}

	numDeleted, err := tx.Topic.Delete().Where(topic.IDIn(ids...)).Exec(ctx)
	if err != nil {
		return err
	}

	// we assume any topic modify listeners already got woken up when the topic
	// was marked for deletion, and won't care about pruning

	a.results = &PruneCommonResults{
		NumDeleted: numDeleted,
	}
	timer.Succeeded(func() { pruneDeletedTopicsCounter.Add(float64(numDeleted)) })

	return nil
}
