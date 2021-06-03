package actions

import (
	"context"
	"errors"
	"time"

	"entgo.io/ent/dialect/sql"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/subscription"
)

type DeadLetterDeliveriesParams struct {
	MaxDeliveries int `json:"maxDeliveries"`
}

type DeadLetterDeliveriesResults struct {
	numDeadLettered int // `json:"numDeadLettered"`
}

type DeadLetterDeliveries struct {
	params  DeadLetterDeliveriesParams
	results *DeadLetterDeliveriesResults
}

func NewDeadLetterDeliveries(params DeadLetterDeliveriesParams) *DeadLetterDeliveries {
	if params.MaxDeliveries < 1 {
		panic(errors.New("MaxDeliveries must be > 0"))
	}
	return &DeadLetterDeliveries{
		params: params,
	}
}

// separate counter from the (shared) counter for the number of deliveries
// deadlettered overall. we keep this counter so we can tell how much was
// dead-lettered live on pull/nack vs. on this background service
var deadLetterDeliveriesActionCounter, deadLetterDeliveriesHistogram = actionMetrics("deadletter_deliveries", "deliveries", "deadlettered")

func (a *DeadLetterDeliveries) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(deadLetterDeliveriesHistogram, tx)
	defer timer.Ended()

	now := time.Now()

	// find deliveries that have exceeded their delivery limit and are due for a
	// retry
	deliveries, err := tx.Delivery.Query().
		Where(
			// this big old mess should generate something like:
			// FROM deliveries JOIN subscriptions
			// WHERE s.deleted_at IS NOT NULL
			// AND s.max_delivery_attempts > 0 -- implicitly IS NOT NULL
			// AND s.dead_letter_topic_id IS NOT NULL
			// AND d.attempts >= s.max_delivery_attempts
			// AND d.completed_at IS NULL
			// AND d.expires_at < now
			func(s *sql.Selector) {
				t := sql.Table(subscription.Table)
				s.Join(t).On(s.C(delivery.SubscriptionColumn), t.C(subscription.FieldID))
				// subscription must not be deleted and must have dead-letter config
				s.Where(sql.And(
					sql.IsNull(t.C(subscription.FieldDeletedAt)),
					sql.GT(t.C(subscription.FieldMaxDeliveryAttempts), 0),
					sql.NotNull(t.C(subscription.DeadLetterTopicColumn)),
					sql.GTE(s.C(delivery.FieldAttempts), t.C(subscription.FieldMaxDeliveryAttempts)),
				))
			},
			delivery.CompletedAtIsNil(),
			// spec is that we dead-letter messages that fail their delivery counter,
			// not those that expire
			delivery.ExpiresAtGT(now),
			// we ignore ordering constraints here, since we shouldn't get any matches
			// if it's blocked on ordering, and anyways dead-lettering is not fully
			// supported in that case
		).
		WithSubscription().
		Limit(a.params.MaxDeliveries).
		All(ctx)
	if err != nil {
		return err
	}

	for _, delivery := range deliveries {
		if err = deadLetterDelivery(
			ctx,
			tx,
			delivery.Edges.Subscription,
			delivery,
			now,
			"actions/dead-letter-deliveries",
		); err != nil {
			return err
		}
		deadLetterDeliveriesActionCounter.Inc()
	}

	a.results = &DeadLetterDeliveriesResults{
		numDeadLettered: len(deliveries),
	}

	return nil
}

func (a *DeadLetterDeliveries) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"maxDeliveries": a.params.MaxDeliveries,
	}
}

func (a *DeadLetterDeliveries) NumDeadLettered() int {
	return a.results.numDeadLettered
}

func (a *DeadLetterDeliveries) HasResults() bool {
	return a.results != nil
}

func (a *DeadLetterDeliveries) Results() map[string]interface{} {
	return map[string]interface{}{
		"numDeadLettered": a.results.numDeadLettered,
	}
}
