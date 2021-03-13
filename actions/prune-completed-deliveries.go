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

	var del *ent.DeliveryDelete
	cond := delivery.CompletedAtLTE(time.Now().Add(-pcd.params.MinAge))
	if pcd.params.MaxDelete == 0 {
		del = tx.Delivery.Delete().Where(cond)
	} else {
		// ent doesn't support limit on delete commands (that may be a PostgreSQL
		// extension), so have to do a query-then-delete
		ids, err := tx.Delivery.Query().Where(cond).Limit(pcd.params.MaxDelete).IDs(ctx)
		if err != nil {
			return err
		}
		del = tx.Delivery.Delete().Where(delivery.IDIn(ids...))
	}

	numDeleted, err := del.Exec(ctx)
	if err != nil {
		return err
	}

	pcd.results = &PruneCommonResults{
		NumDeleted: numDeleted,
	}
	timer.Succeeded(func() { pruneCompletedDeliveriesCounter.Add(float64(numDeleted)) })

	return nil
}
