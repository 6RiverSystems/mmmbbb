package actions

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"go.6river.tech/gosix/logging"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/message"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/ent/topic"
	"go.6river.tech/mmmbbb/filter"
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
		var f filter.Filter
		if err := filter.Parser.ParseString(s.Name, *s.MessageFilter, &f); err != nil {
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
		SetPublishedAt(now)
	if s.OrderedDelivery && m.OrderKey != nil && *m.OrderKey != "" {
		// set the delivery NotBefore the most recent non-expired delivery
		lastDelivery, err := tx.Subscription.QueryDeliveries(s).
			Where(
				delivery.ExpiresAtGT(now),
				delivery.HasMessageWith(
					message.OrderKey(*m.OrderKey),
					// ent uses a very wide sub-select, this helps narrow it down
					// UPSTREAM: ticket for HasRelationWith efficiency
					// TODO: this won't work correctly if this is a dead letter delivery,
					// but then dead letter delivery isn't nominally supported for
					// ordered message delivery
					message.TopicID(m.TopicID),
				),
			).
			Order(ent.Desc(delivery.FieldPublishedAt)).
			First(ctx)
		if err == nil {
			createDelivery.SetNotBefore(lastDelivery)
		} else if err != nil && !ent.IsNotFound(err) {
			return nil, err
		}
	}
	notifyPublish(tx, s.ID)
	return createDelivery, nil
}

// SQL tags here need to match entities/queries used in this package
type deadLetterData struct {
	DeliveryID        uuid.UUID `sql:"id"`
	DeliveryMessageID uuid.UUID `sql:"message_id"`
	DeadLetterTopicID uuid.UUID `sql:"dead_letter_topic_id"`
}

func deadLetterDataFromEntities(delivery *ent.Delivery, sub *ent.Subscription) deadLetterData {
	if sub == nil && delivery.Edges.Subscription != nil {
		sub = delivery.Edges.Subscription
	}
	return deadLetterData{
		DeliveryID:        delivery.ID,
		DeliveryMessageID: delivery.MessageID,
		DeadLetterTopicID: *sub.DeadLetterTopicID,
	}
}

func deadLetterDelivery(
	ctx context.Context,
	tx *ent.Tx,
	x deadLetterData,
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
			topic.ID(x.DeadLetterTopicID),
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
		m, err := tx.Message.Get(ctx, x.DeliveryMessageID)
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

	// and delete the original delivery
	if err := tx.Delivery.DeleteOneID(x.DeliveryID).
		Exec(ctx); err != nil {
		return err
	}
	deadLetterDeliveriesCounter.Inc()
	return nil
}
