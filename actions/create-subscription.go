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
	"fmt"
	"time"

	"github.com/google/uuid"

	"go.6river.tech/gosix/ent/customtypes"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/ent/topic"
	"go.6river.tech/mmmbbb/filter"
)

type CreateSubscriptionParams struct {
	TopicName              string
	Name                   string
	TTL                    time.Duration
	MessageTTL             time.Duration
	OrderedDelivery        bool
	Labels                 map[string]string
	PushEndpoint           string
	MinBackoff, MaxBackoff time.Duration
	Filter                 string

	// these two normally should both be present, or not
	MaxDeliveryAttempts int32
	DeadLetterTopic     string
}
type createSubscriptionResults struct {
	TopicID uuid.UUID
	ID      uuid.UUID
	Sub     *ent.Subscription `json:"-"`
}
type CreateSubscription struct {
	actionBase[CreateSubscriptionParams, createSubscriptionResults]
}

var _ Action[CreateSubscriptionParams, createSubscriptionResults] = (*CreateSubscription)(nil)

func NewCreateSubscription(params CreateSubscriptionParams) *CreateSubscription {
	if params.TTL <= 0 {
		panic(errors.New("TTL must be > 0"))
	}
	if params.MessageTTL <= 0 {
		panic(errors.New("messageTTL must be > 0"))
	}
	if params.MaxDeliveryAttempts < 0 {
		panic(errors.New("MaxDeliveryAttempts must be >= 0"))
	}
	if (params.MaxDeliveryAttempts != 0) != (params.DeadLetterTopic != "") {
		panic(errors.New("must set both or neither of MaxDeliveryAttempts and DeadLetterTopic"))
	}
	return &CreateSubscription{
		actionBase[CreateSubscriptionParams, createSubscriptionResults]{
			params: params,
		},
	}
}

var createSubscriptionsCounter, createSubscriptionsHistogram = actionMetrics("create_subscription", "subscriptions", "created")

func (a *CreateSubscription) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(createSubscriptionsHistogram, tx)
	defer timer.Ended()

	if exists, err := tx.Subscription.Query().
		Where(
			subscription.Name(a.params.Name),
			subscription.DeletedAtIsNil(),
		).
		Exist(ctx); err != nil {
		return err
	} else if exists {
		return ErrExists
	}

	topic, err := findTopic(ctx, tx, a.params.TopicName)
	if err != nil {
		return err
	}

	var pushEndpoint *string
	if a.params.PushEndpoint != "" {
		pushEndpoint = &a.params.PushEndpoint
	}
	var messageFilter *string
	if a.params.Filter != "" {
		// validate the filter before we save it
		if _, err := filter.Parser.ParseString(a.params.Name, a.params.Filter); err != nil {
			return fmt.Errorf("invalid message filter: %w", err)
		} else {
			messageFilter = &a.params.Filter
		}
	}
	create := tx.Subscription.Create().
		SetName(a.params.Name).
		SetTopic(topic).
		SetTTL(customtypes.Interval(a.params.TTL)).
		SetExpiresAt(time.Now().Add(a.params.TTL)).
		SetMessageTTL(customtypes.Interval(a.params.MessageTTL)).
		SetOrderedDelivery(a.params.OrderedDelivery).
		SetLabels(a.params.Labels).
		SetNillablePushEndpoint(pushEndpoint).
		SetNillableMessageFilter(messageFilter)
	if a.params.MinBackoff > 0 {
		create = create.SetMinBackoff(customtypes.IntervalPtr(a.params.MinBackoff))
	}
	if a.params.MaxBackoff > 0 {
		create = create.SetMaxBackoff(customtypes.IntervalPtr(a.params.MaxBackoff))
	}
	if a.params.MaxDeliveryAttempts != 0 {
		create = create.SetMaxDeliveryAttempts(a.params.MaxDeliveryAttempts)
	}
	if a.params.DeadLetterTopic != "" {
		dlTopic, err := findTopic(ctx, tx, a.params.DeadLetterTopic)
		if err != nil {
			return err
		}
		create = create.SetDeadLetterTopic(dlTopic)
	}

	s, err := create.Save(ctx)
	if err != nil {
		// in case two creates raced
		if isSqlDuplicateKeyError(err) {
			return ErrExists
		}
		return err
	}

	NotifyModifySubscription(tx, s.ID, s.Name)

	a.results = &createSubscriptionResults{
		TopicID: topic.ID,
		ID:      s.ID,
		Sub:     s,
	}

	timer.Succeeded(func() { createSubscriptionsCounter.Inc() })

	return nil
}

func findTopic(ctx context.Context, tx *ent.Tx, name string) (*ent.Topic, error) {
	topic, err := tx.Topic.Query().
		Where(
			topic.Name(name),
			topic.DeletedAtIsNil(),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return topic, nil
}
