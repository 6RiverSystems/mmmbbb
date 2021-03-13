package actions

import (
	"context"
	"time"

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

	var del *ent.SubscriptionDelete
	cond := subscription.And(
		subscription.DeletedAtLTE(time.Now().Add(-a.params.MinAge)),
		// we rely on deliveries being pruned to then allow messages to be pruned
		// TODO: ticket for HasRelationWith efficiency
		subscription.Not(subscription.HasDeliveries()),
	)
	if a.params.MaxDelete == 0 {
		del = tx.Subscription.Delete().Where(cond)
	} else {
		// ent doesn't support limit on delete commands (that may be a PostgreSQL
		// extension), so have to do a query-then-delete
		ids, err := tx.Subscription.Query().Where(cond).Limit(a.params.MaxDelete).IDs(ctx)
		if err != nil {
			return err
		}
		del = tx.Subscription.Delete().Where(subscription.IDIn(ids...))
	}

	numDeleted, err := del.Exec(ctx)
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
