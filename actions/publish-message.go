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
	ID            uuid.UUID
	NumDeliveries int
}
type PublishMessage struct {
	actionBase[PublishMessageParams, publishMessageResults]
}

var _ Action[PublishMessageParams, publishMessageResults] = (*PublishMessage)(nil)

func NewPublishMessage(params PublishMessageParams) *PublishMessage {
	if params.TopicName == "" && params.TopicID == nil {
		panic(errors.New("Must provide Name or ID"))
	}
	return &PublishMessage{
		actionBase[PublishMessageParams, publishMessageResults]{
			params: params,
		},
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
		ID:            m.ID,
		NumDeliveries: len(dc),
	}
	timer.Succeeded(func() {
		publishedMessagesCounter.Inc()
		enqueuedDeliveriesCounter.Add(float64(len(dc)))
	})

	return nil
}
