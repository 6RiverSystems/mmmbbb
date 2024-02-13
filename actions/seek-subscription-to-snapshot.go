// Copyright (c) 2024 6 River Systems
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
	"github.com/google/uuid"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/message"
	"go.6river.tech/mmmbbb/ent/predicate"
	"go.6river.tech/mmmbbb/ent/snapshot"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/filter"
)

type SeekSubscriptionToSnapshotParams struct {
	SubscriptionName string
	SubscriptionID   *uuid.UUID
	SnapshotName     string
	SnapshotID       *uuid.UUID
}

type seekSubscriptionToSnapshotResults = seekSubscriptionResults

type SeekSubscriptionToSnapshot struct {
	actionBase[SeekSubscriptionToSnapshotParams, seekSubscriptionToSnapshotResults]
}

var _ Action[SeekSubscriptionToSnapshotParams, seekSubscriptionToSnapshotResults] = (*SeekSubscriptionToSnapshot)(nil)

func NewSeekSubscriptionToSnapshot(params SeekSubscriptionToSnapshotParams) *SeekSubscriptionToSnapshot {
	if params.SubscriptionName == "" && params.SubscriptionID == nil {
		panic(errors.New("must provide subscription name or id"))
	}
	if params.SnapshotName == "" && params.SnapshotID == nil {
		panic(errors.New("must provide snapshot name or id"))
	}
	return &SeekSubscriptionToSnapshot{
		actionBase[SeekSubscriptionToSnapshotParams, seekSubscriptionToSnapshotResults]{
			params: params,
		},
	}
}

func (a *SeekSubscriptionToSnapshot) Execute(ctx context.Context, tx *ent.Tx) error {
	subMatch := []predicate.Subscription{subscription.DeletedAtIsNil()}
	if a.params.SubscriptionID != nil {
		subMatch = append(subMatch, subscription.ID(*a.params.SubscriptionID))
	}
	if a.params.SubscriptionName != "" {
		subMatch = append(subMatch, subscription.Name(a.params.SubscriptionName))
	}
	sub, err := tx.Subscription.Query().
		Where(subMatch...).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	// save resolved ID & Name so Parameters() can report it
	a.params.SubscriptionID, a.params.SubscriptionName = &sub.ID, sub.Name

	var snapMatch []predicate.Snapshot
	if a.params.SnapshotID != nil {
		snapMatch = append(snapMatch, snapshot.ID(*a.params.SnapshotID))
	}
	if a.params.SnapshotName != "" {
		snapMatch = append(snapMatch, snapshot.Name(a.params.SnapshotName))
	}
	snap, err := tx.Snapshot.Query().
		Where(snapMatch...).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	a.params.SnapshotID, a.params.SnapshotName = &snap.ID, snap.Name

	now := time.Now()

	numAcked, numDeAcked := 0, 0
	// ack everything before the threshold
	if na, err := tx.Delivery.Update().
		Where(
			delivery.SubscriptionID(sub.ID),
			delivery.ExpiresAtGTE(now),
			delivery.PublishedAtLT(snap.AckedMessagesBefore),
			delivery.CompletedAtIsNil(),
		).
		SetCompletedAt(now).
		Save(ctx); err != nil {
		return err
	} else {
		numAcked += na
	}
	// ack things in the ack list
	if len(snap.AckedMessageIDs) != 0 {
		if na, err := tx.Delivery.Update().
			Where(
				delivery.SubscriptionID(sub.ID),
				delivery.ExpiresAtGTE(now),
				delivery.MessageIDIn(snap.AckedMessageIDs...),
				delivery.CompletedAtIsNil(),
			).
			SetCompletedAt(now).
			Save(ctx); err != nil {
			return err
		} else {
			numAcked += na
		}
	}

	// de-ack existing deliveries after the threshold
	if nda, err := tx.Delivery.Update().
		Where(
			delivery.SubscriptionID(sub.ID),
			delivery.ExpiresAtGTE(now),
			delivery.PublishedAtGTE(snap.AckedMessagesBefore),
			delivery.MessageIDNotIn(snap.AckedMessageIDs...),
			delivery.CompletedAtNotNil(),
		).
		ClearCompletedAt().
		SetExpiresAt(now.Add(time.Duration(sub.MessageTTL))).
		SetAttemptAt(now).
		Save(ctx); err != nil {
		return err
	} else {
		numDeAcked += nda
	}

	// recreate missing deliveries from after the threshold. we use an "on
	// conflict ignore" insert instead of trying to find which ones are missing.
	msgs, err := tx.Message.Query().
		Where(
			message.TopicID(sub.TopicID),
			message.PublishedAtGTE(snap.AckedMessagesBefore),
			message.IDNotIn(snap.AckedMessageIDs...),
		).
		Order(message.ByPublishedAt()).
		All(ctx)
	if err != nil {
		return err
	}
	// since this is all for the same sub, caching the filter will help
	var f *filter.Condition
	var newDeliveries []*ent.DeliveryCreate
	for _, m := range msgs {
		if cd, err := deliverToSubscription(ctx, tx, sub, m, now, "actions/seek-subscription-to-snapshot", &f); err != nil {
			return err
		} else if cd != nil {
			newDeliveries = append(newDeliveries, cd)
		}
	}

	if err := tx.Delivery.
		CreateBulk(newDeliveries...).
		OnConflict(sql.DoNothing()).
		Exec(ctx); err != nil {
		return err
	}
	// this won't be quite right
	numDeAcked += len(newDeliveries)

	if numAcked != 0 || numDeAcked != 0 {
		notifyPublish(tx, sub.ID)
	}

	a.results = &seekSubscriptionResults{
		NumAcked:   numAcked,
		NumDeAcked: numDeAcked,
	}

	return nil
}
