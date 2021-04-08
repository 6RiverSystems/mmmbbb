package actions

import (
	"context"
	"time"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
)

type PruneCompletedDeliveries struct {
	pruneAction
}

var _ Action = (*PruneCompletedDeliveries)(nil)

func NewPruneCompletedDeliveries(params PruneCommonParams) *PruneCompletedDeliveries {
	return &PruneCompletedDeliveries{
		pruneAction: *newPruneAction(params),
	}
}

var pruneCompletedDeliveriesCounter, pruneCompletedDeliveriesHistogram = pruneMetrics("completed_deliveries")

func (pcd *PruneCompletedDeliveries) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(pruneCompletedDeliveriesHistogram, tx)
	defer timer.Ended()

	// ent doesn't support limit on delete commands (that may be a PostgreSQL
	// extension), so have to do a query-then-delete
	ids, err := tx.Delivery.Query().
		Where(delivery.CompletedAtLTE(time.Now().Add(-pcd.params.MinAge))).
		Limit(pcd.params.MaxDelete).
		IDs(ctx)
	if err != nil {
		return err
	}
	numDeleted, err := tx.Delivery.Delete().Where(delivery.IDIn(ids...)).Exec(ctx)
	if err != nil {
		return err
	}

	pcd.results = &PruneCommonResults{
		NumDeleted: numDeleted,
	}
	timer.Succeeded(func() { pruneCompletedDeliveriesCounter.Add(float64(numDeleted)) })

	return nil
}
