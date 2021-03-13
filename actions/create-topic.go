package actions

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/topic"
)

type CreateTopicParams struct {
	Name   string
	Labels map[string]string
}
type CreateTopic struct {
	params  CreateTopicParams
	results *struct {
		id uuid.UUID
	}
}

var _ Action = (*CreateTopic)(nil)

func NewCreateTopic(params CreateTopicParams) *CreateTopic {
	if params.Name == "" {
		panic(errors.New("Topic must have a non-empty name"))
	}
	return &CreateTopic{
		params: params,
	}
}

var createTopicsCounter, createTopicsHistogram = actionMetrics("create_topic", "topics", "created")

func (a *CreateTopic) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(createTopicsHistogram, tx)
	defer timer.Ended()

	// TODO: adjust db model so that we don't have a read/modify/update race here
	// if Tx isn't SERIALIZABLE
	exists, err := tx.Topic.Query().
		Where(
			topic.Name(a.params.Name),
			topic.DeletedAtIsNil(),
		).
		Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		return ErrExists
	}

	t, err := tx.Topic.Create().
		SetName(a.params.Name).
		SetLabels(a.params.Labels).
		SetLive(true).
		Save(ctx)
	if err != nil {
		return err
	}

	NotifyModifyTopic(tx, t.ID, t.Name)

	a.results = &struct{ id uuid.UUID }{
		id: t.ID,
	}

	timer.Succeeded(func() { createTopicsCounter.Inc() })

	return nil
}

func (a *CreateTopic) TopicID() uuid.UUID {
	return a.results.id
}

func (a *CreateTopic) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"name": a.params.Name,
	}
}

func (a *CreateTopic) HasResults() bool {
	return a.results != nil
}

func (a *CreateTopic) Results() map[string]interface{} {
	return map[string]interface{}{
		"id": a.results.id,
	}
}
