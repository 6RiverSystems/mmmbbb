package actions

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/ent/topic"
	"go.6river.tech/mmmbbb/parse"
)

type NackDeliveriesParams struct {
	ids []uuid.UUID
}

type nackDeliveriesResults struct {
	numNacked       int
	numDeadLettered int
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

	// look up all the subs once
	var subIDs []uuid.UUID
	subById := map[uuid.UUID]*ent.Subscription{}
	for _, d := range deliveries {
		if _, ok := subById[d.SubscriptionID]; !ok {
			subIDs = append(subIDs, d.SubscriptionID)
			subById[d.SubscriptionID] = nil
		}
	}
	subs, err := tx.Subscription.Query().
		Where(subscription.IDIn(subIDs...)).
		All(ctx)
	if err != nil {
		return err
	}
	for _, s := range subs {
		subById[s.ID] = s
	}

	numDeadLettered := 0
	for _, d := range deliveries {
		sub := subById[d.SubscriptionID]
		if sub.HasFullDeadLetterConfig() && d.Attempts >= int(*sub.MaxDeliveryAttempts) {
			if err := deadLetterDelivery(ctx, tx, sub, d, now, "actions/nack-deliveries"); err != nil {
				return err
			}
			numDeadLettered++
		} else {
			_, fuzzedDelay := NextDelayFor(sub, d.Attempts)
			if err := tx.Delivery.UpdateOne(d).
				SetAttemptAt(now.Add(fuzzedDelay)).
				Exec(ctx); err != nil {
				return err
			}
		}
	}

	a.results = &nackDeliveriesResults{
		numNacked:       len(deliveries),
		numDeadLettered: numDeadLettered,
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

func (a *NackDeliveries) NumDeadLettered() int {
	return a.results.numDeadLettered
}

func (a *NackDeliveries) Results() map[string]interface{} {
	return map[string]interface{}{
		"numNacked":       a.results.numNacked,
		"numDeadLettered": a.results.numDeadLettered,
	}
}

func deadLetterDelivery(
	ctx context.Context,
	tx *ent.Tx,
	sub *ent.Subscription,
	delivery *ent.Delivery,
	now time.Time,
	loggerName string,
) error {
	// send to dead letter instead: make new deliveries in the dead letter
	// subscriptions for the original message, avoids duplicating the message
	// itself.

	// we don't need the topic to do the dead letter delivery, but structuring
	// the query this way helps ignore deleted topics _and_ deleted
	// subscriptions.
	dlTopic, err := sub.QueryTopic().
		Where(topic.DeletedAtIsNil()).
		WithSubscriptions(func(sq *ent.SubscriptionQuery) {
			sq.Where(subscription.DeletedAtIsNil())
		}).
		Only(ctx)
	// ignore not found here, just means the topic was deleted
	if err != nil && !ent.IsNotFound(err) {
		return err
	}
	if dlTopic != nil && len(dlTopic.Edges.Subscriptions) != 0 {
		m := delivery.Edges.Message
		if m == nil {
			m, err = delivery.QueryMessage().Only(ctx)
			if err != nil {
				return err
			}
		}
		var dlc []*ent.DeliveryCreate
		for _, s := range dlTopic.Edges.Subscriptions {
			if dc, err := deliverToSubscription(ctx, tx, s, m, now, loggerName); err != nil {
				return err
			} else if dc != nil {
				dlc = append(dlc, dc)
			}
		}
		_, err = tx.Delivery.CreateBulk(dlc...).Save(ctx)
		if err != nil {
			return err
		}
	}

	// and delete the original delivery
	if err := tx.Delivery.DeleteOne(delivery).
		Exec(ctx); err != nil {
		return err
	}
	deadLetterDeliveriesCounter.Inc()
	return nil
}
