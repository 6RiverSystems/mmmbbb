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
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/message"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/ent/topic"
	"go.6river.tech/mmmbbb/filter"
	"go.6river.tech/mmmbbb/logging"
)

func deliverToSubscription(
	ctx context.Context,
	tx *ent.Tx,
	s *ent.Subscription,
	m *ent.Message,
	now time.Time,
	loggerName string,
) (*ent.DeliveryCreate, error) {
	if s.MessageFilter != nil && *s.MessageFilter != "" {
		// TODO: cache parsed filters
		if f, err := filter.Parser.ParseString(s.Name, *s.MessageFilter); err != nil {
			// filter errors should have been caught at subscription create/update.
			// do not break delivery because one sub has a broken filter, assume the
			// filter matches nothing and drop the message.
			logging.GetLoggerWith(loggerName, func(c zerolog.Context) zerolog.Context {
				return c.
					Str("subscriptionName", s.Name).
					Stringer("subscriptionID", s.ID)
			}).Err(err).Msg("skipping delivery due to subscription filter parse error")
			filterErrorsCounter.Inc()
			return nil, nil
		} else if match, err := f.Evaluate(m.Attributes); err != nil {
			// same idea
			logging.GetLoggerWith(loggerName, func(c zerolog.Context) zerolog.Context {
				return c.
					Str("subscriptionName", s.Name).
					Stringer("subscriptionID", s.ID)
			}).Err(err).Msg("skipping delivery due to subscription filter evaluation error")
			filterErrorsCounter.Inc()
			return nil, nil
		} else if !match {
			// quietly ignore this one, no logging
			filterNoMatchCounter.Inc()
			return nil, nil
		}
	}
	createDelivery := tx.Delivery.Create().
		SetMessage(m).
		SetSubscription(s).
		SetExpiresAt(now.Add(time.Duration(s.MessageTTL))).
		SetPublishedAt(now).
		SetAttemptAt(now.Add(time.Duration(s.DeliveryDelay)))
	if s.OrderedDelivery && m.OrderKey != nil && *m.OrderKey != "" {
		// set the delivery NotBefore the most recent non-expired delivery
		lastDelivery, err := tx.Subscription.QueryDeliveries(s).
			Where(
				delivery.ExpiresAtGT(now),
				// delivery.HasMessageWith is much slower than a join here, because it
				// uses an inefficient `in` subquery
				func(s *sql.Selector) {
					t := sql.Table(message.Table)
					s.Join(t).On(s.C(delivery.MessageColumn), t.C(message.FieldID))
					s.Where(sql.And(
						// not necessary? maybe helps with indexes?
						sql.EQ(t.C(message.TopicColumn), m.TopicID),
					))
				},
			).
			Order(ent.Desc(delivery.FieldPublishedAt)).
			First(ctx)
		if err == nil {
			createDelivery.SetNotBefore(lastDelivery)
		} else if !ent.IsNotFound(err) {
			return nil, err
		}
	}
	notifyPublish(tx, s.ID)
	return createDelivery, nil
}

// SQL tags here need to match entities/queries used in this package
type deadLetterData struct {
	DeliveryID             uuid.UUID `sql:"id"`
	DeliverySubscriptionID uuid.UUID `sql:"subscription_id"`
	DeliveryMessageID      uuid.UUID `sql:"message_id"`
	DeadLetterTopicID      uuid.UUID `sql:"dead_letter_topic_id"`
}

func deadLetterDataFromEntities(delivery *ent.Delivery, sub *ent.Subscription) deadLetterData {
	if sub == nil && delivery.Edges.Subscription != nil {
		sub = delivery.Edges.Subscription
	}
	return deadLetterData{
		DeliveryID:             delivery.ID,
		DeliverySubscriptionID: delivery.SubscriptionID,
		DeliveryMessageID:      delivery.MessageID,
		DeadLetterTopicID:      *sub.DeadLetterTopicID,
	}
}

func deadLetterDelivery(
	ctx context.Context,
	tx *ent.Tx,
	data deadLetterData,
	now time.Time,
	loggerName string,
) error {
	// send to dead letter instead: make new deliveries in the dead letter
	// subscriptions for the original message, avoids duplicating the message
	// itself.

	// we don't need the topic to do the dead letter delivery, but structuring
	// the query this way helps ignore deleted topics _and_ deleted
	// subscriptions.
	dlTopic, err := tx.Topic.Query().
		Where(
			topic.ID(data.DeadLetterTopicID),
			topic.DeletedAtIsNil(),
		).
		WithSubscriptions(func(sq *ent.SubscriptionQuery) {
			sq.Where(subscription.DeletedAtIsNil())
		}).
		Only(ctx)
	// ignore not found here, just means the topic was deleted
	if err != nil && !ent.IsNotFound(err) {
		return err
	}
	if dlTopic != nil && len(dlTopic.Edges.Subscriptions) != 0 {
		m, err := tx.Message.Get(ctx, data.DeliveryMessageID)
		if err != nil {
			return err
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

	// mark the original delivery complete
	if err := tx.Delivery.UpdateOneID(data.DeliveryID).
		SetCompletedAt(now).
		Exec(ctx); err != nil {
		return err
	}
	deadLetterDeliveriesCounter.Inc()

	// and wake any subscribers, in case this dead-lettered delivery was blocking
	// another delivery
	tx.OnCommit(func(c ent.Committer) ent.Committer {
		return ent.CommitFunc(func(ctx context.Context, tx *ent.Tx) error {
			if err := c.Commit(ctx, tx); err != nil {
				return err
			}
			WakePublishListeners(false, data.DeliverySubscriptionID)
			return nil
		})
	})

	return nil
}
