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
	"sort"
	"time"

	"github.com/google/uuid"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/parse"
)

type NackDeliveriesParams struct {
	IDs []uuid.UUID
}

type nackDeliveriesResults struct {
	NumNacked       int
	NumDeadLettered int
}

type NackDeliveries struct {
	actionBase[NackDeliveriesParams, nackDeliveriesResults]
}

var _ Action[NackDeliveriesParams, nackDeliveriesResults] = (*NackDeliveries)(nil)

func NewNackDeliveries(
	ids ...uuid.UUID,
) *NackDeliveries {
	return &NackDeliveries{
		actionBase[NackDeliveriesParams, nackDeliveriesResults]{
			params: NackDeliveriesParams{
				IDs: ids,
			},
		},
	}
}

var nackDeliveriesCounter, nackDeliveriesHistogram = actionMetrics(
	"nack_deliveries",
	"deliveries",
	"nacked",
)

func (a *NackDeliveries) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(nackDeliveriesHistogram, tx)
	defer timer.Ended()

	// this is like GetSubscriptionMessages in how it updates the next attempt,
	// but it doesn't update the lastAttemptedAt nor attempts

	now := time.Now()

	deliveries, err := tx.Delivery.Query().
		Where(
			delivery.IDIn(a.params.IDs...),
			delivery.CompletedAtIsNil(),
			delivery.ExpiresAtGT(now),
		).
		All(ctx)
	if err != nil {
		return err
	}

	// workaround for https://github.com/ent/ent/issues/358: avoid deadlocks by
	// touching deliveries in sorted order by uuid
	sort.Slice(
		deliveries,
		func(i, j int) bool { return parse.UUIDLess(deliveries[i].ID, deliveries[j].ID) },
	)

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
			if err := deadLetterDelivery(
				ctx,
				tx,
				deadLetterDataFromEntities(d, sub),
				now,
				"actions/nack-deliveries",
			); err != nil {
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
		NumNacked:       len(deliveries),
		NumDeadLettered: numDeadLettered,
	}
	timer.Succeeded(func() { nackDeliveriesCounter.Add(float64(len(deliveries))) })

	return nil
}
