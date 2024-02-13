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

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/snapshot"
	"go.6river.tech/mmmbbb/ent/topic"
)

type deleteTopicParams struct {
	Name string
}
type deleteTopicResults struct {
	NumDeleted int
}
type DeleteTopic struct {
	actionBase[deleteTopicParams, deleteTopicResults]
}

var _ Action[deleteTopicParams, deleteTopicResults] = (*DeleteTopic)(nil)

func NewDeleteTopic(name string) *DeleteTopic {
	return &DeleteTopic{
		actionBase[deleteTopicParams, deleteTopicResults]{
			params: deleteTopicParams{
				Name: name,
			},
		},
	}
}

var deleteTopicsCounter, deleteTopicsHistogram = actionMetrics("delete_topic", "topics", "deleted")

func (a *DeleteTopic) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(deleteTopicsHistogram, tx)
	defer timer.Ended()

	topics, err := tx.Topic.Query().
		Where(
			topic.Name(a.params.Name),
			topic.DeletedAtIsNil(),
		).
		All(ctx)
	if err != nil {
		return err
	}
	if len(topics) == 0 {
		return ErrNotFound
	}

	ids := make([]uuid.UUID, len(topics))
	for i, s := range topics {
		ids[i] = s.ID
	}
	numDeleted, err := tx.Topic.Update().
		Where(topic.IDIn(ids...)).
		SetDeletedAt(time.Now()).
		ClearLive().
		Save(ctx)
	if err != nil {
		return err
	}
	// hot-delete any snapshots, no live/dead impl there for now
	if _, err := tx.Snapshot.Delete().
		Where(snapshot.TopicIDIn(ids...)).
		Exec(ctx); err != nil {
		return err
	}

	for _, topic := range topics {
		NotifyModifyTopic(tx, topic.ID, topic.Name)
	}

	a.results = &deleteTopicResults{
		NumDeleted: numDeleted,
	}
	timer.Succeeded(func() { deleteTopicsCounter.Add(float64(numDeleted)) })

	return nil
}
