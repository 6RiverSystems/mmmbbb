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
	"go.6river.tech/mmmbbb/ent/snapshot"
	"go.6river.tech/mmmbbb/ent/subscription"
)

type CreateSnapshotParams struct {
	SubscriptionName string
	Name             string
	Labels           map[string]string
}

type createSnapshotResults struct {
	TopicID        uuid.UUID
	TopicName      string
	SubscriptionID uuid.UUID
	SnapshotID     uuid.UUID
	Snapshot       *ent.Snapshot `json:"-"`
}

type CreateSnapshot struct {
	actionBase[CreateSnapshotParams, createSnapshotResults]
}

var _ Action[CreateSnapshotParams, createSnapshotResults] = (*CreateSnapshot)(nil)

func NewCreateSnapshot(params CreateSnapshotParams) *CreateSnapshot {
	if params.SubscriptionName == "" {
		panic(errors.New("missing subscription name"))
	}
	if params.Name == "" {
		panic(errors.New("missing snapshot name"))
	}
	return &CreateSnapshot{
		actionBase[CreateSnapshotParams, createSnapshotResults]{
			params: params,
		},
	}
}

var createSnapshotsCounter, createSnapshotsHistogram = actionMetrics("create_snapshot", "snapshots", "created")

func (a *CreateSnapshot) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(createSnapshotsHistogram, tx)
	defer timer.Ended()
	now := time.Now()

	if exists, err := tx.Snapshot.Query().
		Where(snapshot.Name(a.params.Name)).
		Exist(ctx); err != nil {
		return err
	} else if exists {
		return ErrExists
	}

	sub, err := tx.Subscription.Query().
		Where(
			subscription.Name(a.params.SubscriptionName),
			subscription.DeletedAtIsNil(),
		).
		WithTopic().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}

	create := tx.Snapshot.Create().
		SetID(uuid.New()).
		SetName(a.params.Name).
		SetTopicID(sub.TopicID).
		SetCreatedAt(now).
		SetExpiresAt(now.Add(defaultSnapshotTTL))
	if a.params.Labels != nil {
		create = create.SetLabels(a.params.Labels)
	} else {
		create = create.SetLabels(map[string]string{})
	}

	// we need to find the oldest un-ack'd message and get its timestamp, and then
	// the list of all the ack'd messages newer than that. first part is easy,
	// second part is a bit of a nasty join.

	oldestUnAcked, err := sub.QueryDeliveries().
		Where(
			delivery.CompletedAtIsNil(),
			delivery.ExpiresAtGT(now),
		).
		Order(delivery.ByPublishedAt()).
		Limit(1).
		First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}
	if oldestUnAcked == nil {
		// easy, but unlikely path: nothing is acked, so we just have the timestamp and an empty slice
		create = create.
			SetAckedMessagesBefore(now).
			SetAckedMessageIDs([]uuid.UUID{})
	} else {
		// complex path
		create = create.SetAckedMessagesBefore(oldestUnAcked.PublishedAt)
		// acked messages might have a completed delivery, or they might have no
		// delivery at all, so we left join the topic messages to find both.
		// TODO: this may return a LOT of data, paginate it
		ackedIDs, err := tx.Message.Query().
			Where(
				message.TopicID(sub.TopicID),
				message.PublishedAtGTE(oldestUnAcked.PublishedAt),
				func(s *sql.Selector) {
					t := sql.Table(delivery.Table).As("d")
					s.LeftJoin(t).On(s.C(message.FieldID), t.C(delivery.FieldMessageID))
					s.Where(sql.Or(
						// no delivery
						sql.IsNull(t.C(delivery.FieldID)),
						// completed delivery
						sql.NotNull(t.C(delivery.FieldCompletedAt)),
					))
				},
			).
			IDs(ctx)
		if err != nil {
			return err
		}
		create = create.SetAckedMessageIDs(ackedIDs)
	}

	snap, err := create.Save(ctx)
	if err != nil {
		return err
	}

	a.results = &createSnapshotResults{
		TopicID:        sub.TopicID,
		TopicName:      sub.Edges.Topic.Name,
		SubscriptionID: sub.ID,
		SnapshotID:     snap.ID,
		Snapshot:       snap,
	}

	timer.Succeeded(func() { createSnapshotsCounter.Inc() })

	return nil
}

const defaultSnapshotTTL = 7 * 24 * time.Hour
