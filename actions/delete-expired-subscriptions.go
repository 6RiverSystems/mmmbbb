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
	"go.6river.tech/mmmbbb/logging"
)

type DeleteExpiredSubscriptions struct {
	pruneAction
}

var _ Action[PruneCommonParams, PruneCommonResults] = (*DeleteExpiredSubscriptions)(nil)

func NewDeleteExpiredSubscriptions(params PruneCommonParams) *DeleteExpiredSubscriptions {
	return &DeleteExpiredSubscriptions{
		pruneAction: newPruneAction(params),
	}
}

var DeleteExpiredSubscriptionsCounter, DeleteExpiredSubscriptionsHistogram = pruneMetrics(
	"expired_subscriptions",
)

func (a *DeleteExpiredSubscriptions) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(DeleteExpiredSubscriptionsHistogram, tx)
	defer timer.Ended()

	now := time.Now()
	// NOTE: notification requires knowing the subs affected, so we require a
	// specific MaxDelete for now. if ent could do limit on delete, plus returning
	// clauses, then we could do this without the pre-query
	subs, err := tx.Subscription.Query().
		Where(
			subscription.ExpiresAtLT(now),
			subscription.DeletedAtIsNil(),
			// NOTE: we don't use MinAge here: we could use it as a grace period, but
			// that doesn't seem helpful
		).
		Limit(a.params.MaxDelete).
		All(ctx)
	if err != nil {
		return err
	}

	ids := make([]uuid.UUID, len(subs))
	logger := logging.GetLogger("actions/delete-expired-subscriptions")
	for i, s := range subs {
		ids[i] = s.ID
		logger.Info().
			Str("subscriptionName", s.Name).
			Stringer("subscriptionID", s.ID).
			Msg("deleting expired subscription")
	}
	numDeleted, err := tx.Subscription.Update().
		Where(subscription.IDIn(ids...)).
		SetDeletedAt(now).
		ClearLive().
		Save(ctx)
	if err != nil {
		return err
	}

	for _, sub := range subs {
		NotifyModifySubscription(tx, sub.ID, sub.Name)
	}

	a.results = &PruneCommonResults{
		NumDeleted: numDeleted,
	}

	timer.Succeeded(func() { DeleteExpiredSubscriptionsCounter.Add(float64(numDeleted)) })

	return nil
}
