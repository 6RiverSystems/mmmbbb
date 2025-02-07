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

type PruneDeletedSubscriptions struct {
	pruneAction
}

var _ Action[PruneCommonParams, PruneCommonResults] = (*PruneDeletedSubscriptions)(nil)

func NewPruneDeletedSubscriptions(params PruneCommonParams) *PruneDeletedSubscriptions {
	return &PruneDeletedSubscriptions{
		pruneAction: newPruneAction(params),
	}
}

var pruneDeletedSubscriptionsCounter, pruneDeletedSubscriptionsHistogram = pruneMetrics(
	"deleted_subscriptions",
)

func (a *PruneDeletedSubscriptions) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(pruneDeletedSubscriptionsHistogram, tx)
	defer timer.Ended()

	subs, err := tx.Subscription.Query().
		Where(
			subscription.DeletedAtLTE(time.Now().Add(-a.params.MinAge)),
			// we rely on deliveries being pruned to then allow messages to be pruned
			// UPSTREAM: ticket for HasRelationWith efficiency
			subscription.Not(subscription.HasDeliveries()),
		).
		Limit(a.params.MaxDelete).
		All(ctx)
	if err != nil {
		return err
	}
	ids := make([]uuid.UUID, len(subs))
	logger := logging.GetLogger("actions/prune-deleted-subscriptions")
	for i, s := range subs {
		ids[i] = s.ID
		logger.Info().
			Str("subscriptionName", s.Name).
			Stringer("subscriptionID", s.ID).
			Time("deletedAt", *s.DeletedAt).
			Msg("pruning deleted subscription")
	}

	numDeleted, err := tx.Subscription.Delete().Where(subscription.IDIn(ids...)).Exec(ctx)
	if err != nil {
		return err
	}

	// we assume any sub modify listeners already got woken up when the sub was
	// marked for deletion, and won't care about pruning

	a.results = &PruneCommonResults{
		NumDeleted: numDeleted,
	}
	timer.Succeeded(func() { pruneDeletedSubscriptionsCounter.Add(float64(numDeleted)) })

	return nil
}
