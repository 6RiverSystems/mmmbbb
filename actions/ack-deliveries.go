package actions

import (
	"context"
	"sort"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/predicate"
	"go.6river.tech/mmmbbb/parse"
)

type AckDeliveriesParams struct {
	ids []uuid.UUID
}
type ackDeliveriesResults struct {
	numAcked int
}
type AckDeliveries struct {
	params  AckDeliveriesParams
	results *ackDeliveriesResults
}

var _ Action = (*AckDeliveries)(nil)

func NewAckDeliveries(
	ids ...uuid.UUID,
) *AckDeliveries {
	return &AckDeliveries{
		params: AckDeliveriesParams{
			ids: ids,
		},
	}
}

var ackDeliveriesCounter, ackDeliveriesHistogram = actionMetrics("ack_deliveries", "deliveries", "acked")

func (a *AckDeliveries) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(ackDeliveriesHistogram, tx)
	defer timer.Ended()

	// workaround for https://github.com/ent/ent/issues/358: avoid deadlocks by
	// touching deliveries in sorted order by uuid
	sort.Slice(a.params.ids, func(i, j int) bool { return parse.UUIDLess(a.params.ids[i], a.params.ids[j]) })

	predicates := []predicate.Delivery{
		delivery.IDIn(a.params.ids...),
		delivery.CompletedAtIsNil(),
	}

	// we need to know which subs to wake up when we process the acks
	var subIDs []uuid.UUID
	err := tx.Delivery.Query().
		Where(predicates...).
		// this one isn't a predicate, but it's how we hook to ask for a `SELECT
		// DISTINCT`
		Where(func(s *sql.Selector) {
			s.Distinct()
		}).
		Select(delivery.SubscriptionColumn).
		Scan(ctx, &subIDs)
	if err != nil {
		return err
	}

	numAcked, err := tx.Delivery.Update().
		Where(predicates...).
		SetCompletedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return err
	}

	// subscribers awaiting a publish on an ordered subscription may also be
	// awaiting a prior delivery ack, so wake them up. streaming subscribers may
	// be waiting on an external ack to clear flow control blockages, so just wake
	// up everyone.
	tx.OnCommit(func(c ent.Committer) ent.Committer {
		return ent.CommitFunc(func(ctx context.Context, tx *ent.Tx) error {
			if err := c.Commit(ctx, tx); err != nil {
				return err
			}
			for _, s := range subIDs {
				WakePublishListeners(false, s)
			}
			return nil
		})
	})

	a.results = &ackDeliveriesResults{
		numAcked: numAcked,
	}
	timer.Succeeded(func() { ackDeliveriesCounter.Add(float64(numAcked)) })

	return nil
}

func (a *AckDeliveries) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"ids": a.params.ids,
	}
}

func (a *AckDeliveries) HasResults() bool {
	return a.results != nil
}

func (a *AckDeliveries) NumAcked() int {
	return a.results.numAcked
}

func (a *AckDeliveries) Results() map[string]interface{} {
	return map[string]interface{}{
		"numAcked": a.results.numAcked,
	}
}
