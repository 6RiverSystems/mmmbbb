package actions

import (
	"context"
	"time"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/subscription"
)

type PruneExpiredDeliveries struct {
	pruneAction
}

var _ Action = (*PruneExpiredDeliveries)(nil)

func NewPruneExpiredDeliveries(params PruneCommonParams) *PruneExpiredDeliveries {
	return &PruneExpiredDeliveries{
		pruneAction: *newPruneAction(params),
	}
}

var pruneExpiredDeliveriesCounter, pruneExpiredDeliveriesHistogram = pruneMetrics("expired_deliveries")

func (a *PruneExpiredDeliveries) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(pruneExpiredDeliveriesHistogram, tx)
	defer timer.Ended()

	// NOTE: notification requires knowing the subs affected, so we require a
	// specific MaxDelete for now. if ent could do limit on delete, plus returning
	// clauses, then we could do this without the pre-query

	var del *ent.DeliveryDelete
	ids, err := tx.Delivery.Query().
		Where(delivery.ExpiresAtLT(time.Now())).
		Limit(a.params.MaxDelete).
		IDs(ctx)
	if err != nil {
		return err
	}

	// we need to notify any subs using ordered delivery to refresh, same as if we ACKed
	orderedSubs, err := tx.Subscription.Query().
		Where(
			subscription.OrderedDelivery(true),
			subscription.HasDeliveriesWith(delivery.IDIn(ids...)),
		).
		Select(subscription.FieldID).
		All(ctx)
	if err != nil {
		return err
	}

	del = tx.Delivery.Delete().Where(delivery.IDIn(ids...))

	numDeleted, err := del.Exec(ctx)
	if err != nil {
		return err
	}

	// subscribers awaiting a publish on an ordered subscription may also
	// be awaiting ack from a message we just deleted, so wake them up
	if len(orderedSubs) != 0 {
		tx.OnCommit(func(c ent.Committer) ent.Committer {
			return ent.CommitFunc(func(ctx context.Context, tx *ent.Tx) error {
				if err := c.Commit(ctx, tx); err != nil {
					return err
				}
				for _, s := range orderedSubs {
					WakePublishListeners(false, s.ID)
				}
				return nil
			})
		})
	}

	a.results = &PruneCommonResults{
		NumDeleted: numDeleted,
	}
	timer.Succeeded(func() { pruneExpiredDeliveriesCounter.Add(float64(numDeleted)) })

	return nil
}
