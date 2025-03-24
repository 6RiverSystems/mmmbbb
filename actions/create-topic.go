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
	"errors"

	"github.com/google/uuid"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/topic"
)

type CreateTopicParams struct {
	Name   string
	Labels map[string]string
}
type createTopicResults struct {
	ID uuid.UUID
}
type CreateTopic struct {
	actionBase[CreateTopicParams, createTopicResults]
}

var _ Action[CreateTopicParams, createTopicResults] = (*CreateTopic)(nil)

func NewCreateTopic(params CreateTopicParams) *CreateTopic {
	if params.Name == "" {
		panic(errors.New("topic must have a non-empty name"))
	}
	return &CreateTopic{
		actionBase[CreateTopicParams, createTopicResults]{
			params: params,
		},
	}
}

var createTopicsCounter, createTopicsHistogram = actionMetrics("create_topic", "topics", "created")

func (a *CreateTopic) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(createTopicsHistogram, tx)
	defer timer.Ended()

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
		// in case two creates raced
		if isSqlDuplicateKeyError(err) {
			return ErrExists
		}
		return err
	}

	NotifyModifyTopic(tx, t.ID, t.Name)

	a.results = &createTopicResults{
		ID: t.ID,
	}

	timer.Succeeded(func() { createTopicsCounter.Inc() })

	return nil
}
