package actions

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/predicate"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/ent/topic"
)

type PublishMessageParams struct {
	TopicName  string
	TopicID    *uuid.UUID
	Payload    json.RawMessage
	Attributes map[string]string
	OrderKey   string
}
type publishMessageResults struct {
	id            uuid.UUID
	numDeliveries int
}
type PublishMessage struct {
	params  PublishMessageParams
	results *publishMessageResults
}

var _ Action = (*PublishMessage)(nil)

func NewPublishMessage(params PublishMessageParams) *PublishMessage {
	if params.TopicName == "" && params.TopicID == nil {
		panic(errors.New("Must provide Name or ID"))
	}
	return &PublishMessage{
		params: params,
	}
}

func (a *PublishMessage) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(publishedMessagesHistogram, tx)
	defer timer.Ended()

	now := time.Now()

	topicMatch := []predicate.Topic{topic.DeletedAtIsNil()}
	if a.params.TopicID != nil {
		topicMatch = append(topicMatch, topic.ID(*a.params.TopicID))
	}
	if a.params.TopicName != "" {
		topicMatch = append(topicMatch, topic.Name(a.params.TopicName))
	}

	t, err := tx.Topic.Query().
		Where(topicMatch...).
		// preload the active subscriptions, just get their ID and messageTTL, we don't need the rest
		WithSubscriptions(func(sq *ent.SubscriptionQuery) {
			// NOTE: not checking expiresAt here, expiration service will mark it
			// deleted when it's time
			sq.Where(subscription.DeletedAtIsNil())
		}).
		Only(ctx)
	if err != nil {
		var nfe *ent.NotFoundError
		if errors.As(err, &nfe) {
			return ErrNotFound
		}
		return err
	}

	var orderKey *string
	if a.params.OrderKey != "" {
		orderKey = &a.params.OrderKey
	}

	// create the message
	m, err := tx.Message.Create().
		SetTopic(t).
		SetPublishedAt(now).
		SetPayload(a.params.Payload).
		SetAttributes(a.params.Attributes).
		SetNillableOrderKey(orderKey).
		Save(ctx)
	if err != nil {
		return err
	}

	// add a delivery for each active subscription
	dc := make([]*ent.DeliveryCreate, 0, len(t.Edges.Subscriptions))
	for _, s := range t.Edges.Subscriptions {
		if createDelivery, err := deliverToSubscription(ctx, tx, s, m, now, "actions/publish-message"); err != nil {
			return err
		} else if createDelivery != nil {
			dc = append(dc, createDelivery)
		}
	}

	_, err = tx.Delivery.CreateBulk(dc...).Save(ctx)
	if err != nil {
		return err
	}

	a.results = &publishMessageResults{
		id:            m.ID,
		numDeliveries: len(dc),
	}
	timer.Succeeded(func() {
		publishedMessagesCounter.Inc()
		enqueuedDeliveriesCounter.Add(float64(len(dc)))
	})

	return nil
}

func (a *PublishMessage) MessageID() uuid.UUID {
	return a.results.id
}

func (a *PublishMessage) NumDeliveries() int {
	return a.results.numDeliveries
}

func (a *PublishMessage) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"topicName": a.params.TopicName,
		"topicID":   a.params.TopicID,
		"payload":   a.params.Payload,
		"orderKey":  a.params.OrderKey,
	}
}

func (a *PublishMessage) HasResults() bool {
	return a.results != nil
}

func (a *PublishMessage) Results() map[string]interface{} {
	return map[string]interface{}{
		"id":            a.results.id,
		"numDeliveries": a.results.numDeliveries,
	}
}
