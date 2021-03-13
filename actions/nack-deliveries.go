package actions

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/parse"
)

type NackDeliveriesParams struct {
	ids []uuid.UUID
}

type nackDeliveriesResults struct {
	numNacked int
}

type NackDeliveries struct {
	params  NackDeliveriesParams
	results *nackDeliveriesResults
}

var _ Action = (*NackDeliveries)(nil)

func NewNackDeliveries(
	ids ...uuid.UUID,
) *NackDeliveries {
	return &NackDeliveries{
		params: struct{ ids []uuid.UUID }{
			ids: ids,
		},
	}
}

var nackDeliveriesCounter, nackDeliveriesHistogram = actionMetrics("nack_deliveries", "deliveries", "nacked")

func (a *NackDeliveries) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(nackDeliveriesHistogram, tx)
	defer timer.Ended()

	// this is like GetSubscriptionMessages in how it updates the next attempt,
	// but it doesn't update the lastAttemptedAt nor attempts

	now := time.Now()

	deliveries, err := tx.Delivery.Query().
		Where(
			delivery.IDIn(a.params.ids...),
			delivery.CompletedAtIsNil(),
			delivery.ExpiresAtGT(now),
		).
		All(ctx)
	if err != nil {
		return err
	}

	// workaround for https://github.com/ent/ent/issues/358: avoid deadlocks by
	// touching deliveries in sorted order by uuid
	sort.Slice(deliveries, func(i, j int) bool { return parse.UUIDLess(deliveries[i].ID, deliveries[j].ID) })

	for _, d := range deliveries {
		if d.Edges.Subscription == nil {
			d.Edges.Subscription, err = d.QuerySubscription().Only(ctx)
			if err != nil {
				return err
			}
		}
		if err := tx.Delivery.UpdateOne(d).
			SetAttemptAt(now.Add(NextDelayFor(d.Edges.Subscription, d.Attempts))).
			Exec(ctx); err != nil {
			return err
		}
	}

	a.results = &nackDeliveriesResults{
		numNacked: len(deliveries),
	}
	timer.Succeeded(func() { nackDeliveriesCounter.Add(float64(len(deliveries))) })

	return nil
}

func (a *NackDeliveries) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"ids": a.params.ids,
	}
}

func (a *NackDeliveries) HasResults() bool {
	return a.results != nil
}

func (a *NackDeliveries) NumNacked() int {
	return a.results.numNacked
}

func (a *NackDeliveries) Results() map[string]interface{} {
	return map[string]interface{}{
		"numNacked": a.results.numNacked,
	}
}
