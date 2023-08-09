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
	Name string
}
type deleteSubscriptionResults struct {
	NumDeleted int
}
type DeleteSubscription struct {
	actionBase[deleteSubscriptionParams, deleteSubscriptionResults]
}

var _ Action[deleteSubscriptionParams, deleteSubscriptionResults] = (*DeleteSubscription)(nil)

func NewDeleteSubscription(name string) *DeleteSubscription {
	return &DeleteSubscription{
		actionBase[deleteSubscriptionParams, deleteSubscriptionResults]{
			params: deleteSubscriptionParams{
				Name: name,
			},
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
			subscription.Name(a.params.Name),
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
		NumDeleted: numDeleted,
	}

	timer.Succeeded(func() { deleteSubscriptionsCounter.Add(float64(numDeleted)) })

	return nil
}
