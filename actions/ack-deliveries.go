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

type AckDeliveriesParams struct {
	ids []uuid.UUID
}
type ackDeliveriesResults struct {
	NumAcked int
}
type AckDeliveries struct {
	actionBase[AckDeliveriesParams, ackDeliveriesResults]
}

var _ Action[AckDeliveriesParams, ackDeliveriesResults] = (*AckDeliveries)(nil)

func NewAckDeliveries(
	ids ...uuid.UUID,
) *AckDeliveries {
	return &AckDeliveries{
		actionBase[AckDeliveriesParams, ackDeliveriesResults]{
			params: AckDeliveriesParams{
				ids: ids,
			},
		},
	}
}

var ackDeliveriesCounter, ackDeliveriesHistogram = actionMetrics(
	"ack_deliveries",
	"deliveries",
	"acked",
)

func (a *AckDeliveries) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(ackDeliveriesHistogram, tx)
	defer timer.Ended()

	// workaround for https://github.com/ent/ent/issues/358: avoid deadlocks by
	// touching deliveries in sorted order by uuid
	sort.Slice(
		a.params.ids,
		func(i, j int) bool { return parse.UUIDLess(a.params.ids[i], a.params.ids[j]) },
	)

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
		NumAcked: numAcked,
	}
	timer.Succeeded(func() { ackDeliveriesCounter.Add(float64(numAcked)) })

	return nil
}
