package actions

import (
	"context"
	"time"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/subscription"
)

type PruneDeletedSubscriptionDeliveries struct {
	pruneAction
}

var _ Action = (*PruneDeletedSubscriptionDeliveries)(nil)

func NewPruneDeletedSubscriptionDeliveries(params PruneCommonParams) *PruneDeletedSubscriptionDeliveries {
	return &PruneDeletedSubscriptionDeliveries{
		pruneAction: *newPruneAction(params),
	}
}

var pruneDeletedSubscriptionDeliveriesCounter, pruneDeletedSubscriptionDeliveriesHistogram = pruneMetrics("deleted_subscription_deliveries")

func (a *PruneDeletedSubscriptionDeliveries) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(pruneDeletedSubscriptionDeliveriesHistogram, tx)
	defer timer.Ended()

	// ent doesn't support limit on delete commands (that may be a PostgreSQL
	// extension), so have to do a query-then-delete
	ids, err := tx.Delivery.Query().
		Where(
			// TODO: ticket for HasRelationWith efficiency
			delivery.HasSubscriptionWith(
				subscription.DeletedAtLTE(time.Now().Add(-a.params.MinAge)),
			),
		).
		Limit(a.params.MaxDelete).
		IDs(ctx)
	if err != nil {
		return err
	}
	numDeleted, err := tx.Delivery.Delete().Where(delivery.IDIn(ids...)).Exec(ctx)
	if err != nil {
		return err
	}

	a.results = &PruneCommonResults{
		NumDeleted: numDeleted,
	}
	timer.Succeeded(func() { pruneDeletedSubscriptionDeliveriesCounter.Add(float64(numDeleted)) })

	return nil
}
