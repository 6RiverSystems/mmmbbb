package actions

import (
	"context"
	"time"

	"github.com/google/uuid"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/topic"
)

type deleteTopicParams struct {
	name string
}
type deleteTopicResults struct {
	numDeleted int
}
type DeleteTopic struct {
	params  deleteTopicParams
	results *deleteTopicResults
}

var _ Action = (*DeleteTopic)(nil)

func NewDeleteTopic(name string) *DeleteTopic {
	return &DeleteTopic{
		params: struct{ name string }{
			name: name,
		},
	}
}

var deleteTopicsCounter, deleteTopicsHistogram = actionMetrics("delete_topic", "topics", "deleted")

func (a *DeleteTopic) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(deleteTopicsHistogram, tx)
	defer timer.Ended()

	topics, err := tx.Topic.Query().
		Where(
			topic.Name(a.params.name),
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

	for _, topic := range topics {
		NotifyModifyTopic(tx, topic.ID, topic.Name)
	}

	a.results = &deleteTopicResults{
		numDeleted: numDeleted,
	}
	timer.Succeeded(func() { deleteTopicsCounter.Add(float64(numDeleted)) })

	return nil
}

func (a *DeleteTopic) NumDeleted() int {
	return a.results.numDeleted
}

func (a *DeleteTopic) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"name": a.params.name,
	}
}

func (a *DeleteTopic) HasResults() bool {
	return a.results != nil
}

func (a *DeleteTopic) Results() map[string]interface{} {
	return map[string]interface{}{
		"numDeleted": a.results.numDeleted,
	}
}
