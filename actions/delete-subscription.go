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
	"go.6river.tech/mmmbbb/ent/subscription"
)

type deleteSubscriptionParams struct {
	name string
}
type deleteSubscriptionResults struct {
	numDeleted int
}
type DeleteSubscription struct {
	params  deleteSubscriptionParams
	results *deleteSubscriptionResults
}

var _ Action = (*DeleteSubscription)(nil)

func NewDeleteSubscription(name string) *DeleteSubscription {
	return &DeleteSubscription{
		params: struct{ name string }{
			name: name,
		},
	}
}

var deleteSubscriptionsCounter, deleteSubscriptionsHistogram = actionMetrics("delete_subscription", "subscriptions", "deleted")

func (a *DeleteSubscription) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(deleteSubscriptionsHistogram, tx)
	defer timer.Ended()

	// need to lookup the subs to notify about deleting them
	subs, err := tx.Subscription.Query().
		Where(
			subscription.Name(a.params.name),
			subscription.DeletedAtIsNil(),
		).
		All(ctx)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return ErrNotFound
	}

	ids := make([]uuid.UUID, len(subs))
	for i, s := range subs {
		ids[i] = s.ID
	}
	numDeleted, err := tx.Subscription.Update().
		Where(subscription.IDIn(ids...)).
		SetDeletedAt(time.Now()).
		ClearLive().
		Save(ctx)
	if err != nil {
		return err
	}

	for _, sub := range subs {
		NotifyModifySubscription(tx, sub.ID, sub.Name)
	}

	a.results = &deleteSubscriptionResults{
		numDeleted: numDeleted,
	}

	timer.Succeeded(func() { deleteSubscriptionsCounter.Add(float64(numDeleted)) })

	return nil
}

func (a *DeleteSubscription) NumDeleted() int {
	return a.results.numDeleted
}

func (a *DeleteSubscription) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"name": a.params.name,
	}
}

func (a *DeleteSubscription) HasResults() bool {
	return a.results != nil
}

func (a *DeleteSubscription) Results() map[string]interface{} {
	return map[string]interface{}{
		"numDeleted": a.results.numDeleted,
	}
}
