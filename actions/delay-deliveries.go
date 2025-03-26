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
	NumDelayed int
}

type DelayDeliveries struct {
	actionBase[DelayDeliveriesParams, delayDeliveriesResults]
}

var _ Action[DelayDeliveriesParams, delayDeliveriesResults] = (*DelayDeliveries)(nil)

func NewDelayDeliveries(params DelayDeliveriesParams) *DelayDeliveries {
	// we accept negative delays at this layer, some higher layers will enforce tighter constraints
	return &DelayDeliveries{
		actionBase[DelayDeliveriesParams, delayDeliveriesResults]{
			params: params,
		},
	}
}

var delayDeliveriesCounter, delayDeliveriesHistogram = actionMetrics(
	"delay_deliveries",
	"deliveries",
	"delayed",
)

func (a *DelayDeliveries) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(delayDeliveriesHistogram, tx)
	defer timer.Ended()

	// workaround for https://github.com/ent/ent/issues/358: avoid deadlocks by
	// touching deliveries in sorted order by uuid
	sort.Slice(
		a.params.IDs,
		func(i, j int) bool { return parse.UUIDLess(a.params.IDs[i], a.params.IDs[j]) },
	)

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
		NumDelayed: numDelayed,
	}
	timer.Succeeded(func() { delayDeliveriesCounter.Add(float64(numDelayed)) })

	return nil
}
