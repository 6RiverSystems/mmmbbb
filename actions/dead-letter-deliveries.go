// Copyright (c) 2021 6 River Systems
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

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
	NumDeadLettered int
}

type DeadLetterDeliveries struct {
	actionBase[DeadLetterDeliveriesParams, DeadLetterDeliveriesResults]
}

func NewDeadLetterDeliveries(params DeadLetterDeliveriesParams) *DeadLetterDeliveries {
	if params.MaxDeliveries < 1 {
		panic(errors.New("maxDeliveries must be > 0"))
	}
	return &DeadLetterDeliveries{
		actionBase[DeadLetterDeliveriesParams, DeadLetterDeliveriesResults]{
			params: params,
		},
	}
}

// separate counter from the (shared) counter for the number of deliveries
// deadlettered overall. we keep this counter so we can tell how much was
// dead-lettered live on pull/nack vs. on this background service
var deadLetterDeliveriesActionCounter, deadLetterDeliveriesHistogram = actionMetrics(
	"deadletter_deliveries",
	"deliveries",
	"deadlettered",
)

func (a *DeadLetterDeliveries) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(deadLetterDeliveriesHistogram, tx)
	defer timer.Ended()

	now := time.Now()

	// find deliveries that have exceeded their delivery limit and are due for a
	// retry. relies on `sql` tagging of `deadLetterData` matching the query below
	deliveryData := make([]deadLetterData, 0)
	err := tx.Delivery.Query().
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
					sql.NotNull(t.C(subscription.FieldDeadLetterTopicID)),
					sql.ColumnsGTE(
						s.C(delivery.FieldAttempts),
						t.C(subscription.FieldMaxDeliveryAttempts),
					),
				))
				// this column alias needs to match the `sql` tag on `deadLetterData`
				s.AppendSelect(
					sql.As(t.C(subscription.FieldDeadLetterTopicID), "dead_letter_topic_id"),
				)
			},
			// ignore completed deliveries, of course
			delivery.CompletedAtIsNil(),
			// spec is that we dead-letter messages that fail their delivery counter,
			// not those that expire
			delivery.ExpiresAtGT(now),
			// must not deadletter things on their last attempt until that has expired
			delivery.AttemptAtLTE(now),
			// we ignore ordering constraints here, since we shouldn't get any matches
			// if it's blocked on ordering, and anyways dead-lettering is not fully
			// supported in that case
		).
		Limit(a.params.MaxDeliveries).
		Select(delivery.FieldID, delivery.FieldMessageID, delivery.FieldSubscriptionID).
		Scan(ctx, &deliveryData)
	if err != nil {
		return err
	}

	for _, datum := range deliveryData {
		if err = deadLetterDelivery(
			ctx,
			tx,
			datum,
			now,
			"actions/dead-letter-deliveries",
		); err != nil {
			return err
		}
	}

	a.results = &DeadLetterDeliveriesResults{
		NumDeadLettered: len(deliveryData),
	}

	timer.Succeeded(func() { deadLetterDeliveriesActionCounter.Add(float64(len(deliveryData))) })

	return nil
}
