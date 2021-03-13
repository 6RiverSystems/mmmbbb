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

type DelayDeliveriesParams struct {
	IDs   []uuid.UUID
	Delay time.Duration
}

type delayDeliveriesResults struct {
	numDelayed int
}

type DelayDeliveries struct {
	params  DelayDeliveriesParams
	results *delayDeliveriesResults
}

var _ Action = (*DelayDeliveries)(nil)

func NewDelayDeliveries(params DelayDeliveriesParams) *DelayDeliveries {
	// we accept negative delays at this layer, some higher layers will enforce tighter constraints
	return &DelayDeliveries{
		params: params,
	}
}

var delayDeliveriesCounter, delayDeliveriesHistogram = actionMetrics("delay_deliveries", "deliveries", "delayed")

func (a *DelayDeliveries) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(delayDeliveriesHistogram, tx)
	defer timer.Ended()

	// workaround for https://github.com/ent/ent/issues/358: avoid deadlocks by
	// touching deliveries in sorted order by uuid
	sort.Slice(a.params.IDs, func(i, j int) bool { return parse.UUIDLess(a.params.IDs[i], a.params.IDs[j]) })

	predicates := []predicate.Delivery{
		delivery.IDIn(a.params.IDs...),
		delivery.CompletedAtIsNil(),
	}

	newAttemptAt := time.Now().Add(a.params.Delay)

	// if this is a nack, we need to know which subs to wake up when we process it
	var subIDs []uuid.UUID
	if a.params.Delay <= 0 {
		err := tx.Delivery.Query().
			Where(predicates...).
			// this one isn't a predicate, but it's how we hook to ask for a `SELECT DISTINCT`
			Where(func(s *sql.Selector) {
				s.Distinct()
			}).
			Select(delivery.SubscriptionColumn).
			Scan(ctx, &subIDs)
		if err != nil {
			return err
		}
	} else {
		// not a nack, only move next-attempt later, never sooner
		predicates = append(predicates, delivery.AttemptAtLT(newAttemptAt))
	}

	numDelayed, err := tx.Delivery.Update().
		Where(predicates...).
		SetAttemptAt(newAttemptAt).
		Save(ctx)
	if err != nil {
		return err
	}

	// if the delay is zero, this is a nack, and we should wake up subscribers
	// (subIDs will only be populated if this is a nack)
	if len(subIDs) != 0 {
		notifyPublish(tx, subIDs...)
	}

	a.results = &delayDeliveriesResults{
		numDelayed: numDelayed,
	}
	timer.Succeeded(func() { delayDeliveriesCounter.Add(float64(numDelayed)) })

	return nil
}

func (a *DelayDeliveries) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"ids":   a.params.IDs,
		"delay": a.params.Delay,
	}
}

func (a *DelayDeliveries) HasResults() bool {
	return a.results != nil
}

func (a *DelayDeliveries) NumDelayed() int {
	return a.results.numDelayed
}

func (a *DelayDeliveries) Results() map[string]interface{} {
	return map[string]interface{}{
		"numDelayed": a.results.numDelayed,
	}
}
