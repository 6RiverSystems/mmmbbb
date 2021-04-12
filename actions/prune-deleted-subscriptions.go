package actions

import (
	"context"
	"time"

	"github.com/google/uuid"

	"go.6river.tech/gosix/logging"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/subscription"
)

type PruneDeletedSubscriptions struct {
	pruneAction
}

var _ Action = (*PruneDeletedSubscriptions)(nil)

func NewPruneDeletedSubscriptions(params PruneCommonParams) *PruneDeletedSubscriptions {
	return &PruneDeletedSubscriptions{
		pruneAction: *newPruneAction(params),
	}
}

var pruneDeletedSubscriptionsCounter, pruneDeletedSubscriptionsHistogram = pruneMetrics("deleted_subscriptions")

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
