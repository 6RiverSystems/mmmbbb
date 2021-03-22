package actions

import (
	"context"
	"fmt"
	"time"

	"errors"

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
}
type createSubscriptionResults struct {
	topicID uuid.UUID
	id      uuid.UUID
	sub     *ent.Subscription
}
type CreateSubscription struct {
	params  CreateSubscriptionParams
	results *createSubscriptionResults
}

var _ Action = (*CreateSubscription)(nil)

func NewCreateSubscription(params CreateSubscriptionParams) *CreateSubscription {
	if params.TTL <= 0 {
		panic(errors.New("TTL must be > 0"))
	}
	if params.MessageTTL <= 0 {
		panic(errors.New("messageTTL must be > 0"))
	}
	return &CreateSubscription{
		params: params,
	}
}

var createSubscriptionsCounter, createSubscriptionsHistogram = actionMetrics("create_subscription", "subscriptions", "created")

func (a *CreateSubscription) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(createSubscriptionsHistogram, tx)
	defer timer.Ended()

	exists, err := tx.Subscription.Query().
		Where(
			subscription.Name(a.params.Name),
			subscription.DeletedAtIsNil(),
		).
		Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		return ErrExists
	}

	topic, err := tx.Topic.Query().
		Where(
			topic.Name(a.params.TopicName),
			topic.DeletedAtIsNil(),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}

	var minBackoff, maxBackoff customtypes.IntervalNull
	if a.params.MinBackoff > 0 {
		minBackoff = customtypes.FromDuration(a.params.MinBackoff).AsNullable()
	}
	if a.params.MaxBackoff > 0 {
		maxBackoff = customtypes.FromDuration(a.params.MaxBackoff).AsNullable()
	}

	var pushEndpoint *string
	if a.params.PushEndpoint != "" {
		pushEndpoint = &a.params.PushEndpoint
	}
	var messageFilter *string
	if a.params.Filter != "" {
		// validate the filter before we save it
		var f filter.Filter
		if err := filter.Parser.ParseString(a.params.Name, a.params.Filter, &f); err != nil {
			return fmt.Errorf("invalid message filter: %w", err)
		}
		messageFilter = &a.params.Filter
	}
	s, err := tx.Subscription.Create().
		SetName(a.params.Name).
		SetTopic(topic).
		SetTTL(customtypes.FromDuration(a.params.TTL)).
		SetExpiresAt(time.Now().Add(a.params.TTL)).
		SetMessageTTL(customtypes.FromDuration(a.params.MessageTTL)).
		SetOrderedDelivery(a.params.OrderedDelivery).
		SetLabels(a.params.Labels).
		SetNillablePushEndpoint(pushEndpoint).
		SetMinBackoff(minBackoff).
		SetMaxBackoff(maxBackoff).
		SetNillableMessageFilter(messageFilter).
		Save(ctx)
	if err != nil {
		return err
	}

	NotifyModifySubscription(tx, s.ID, s.Name)

	a.results = &createSubscriptionResults{
		topicID: topic.ID,
		id:      s.ID,
		sub:     s,
	}

	timer.Succeeded(func() { createSubscriptionsCounter.Inc() })

	return nil
}

func (a *CreateSubscription) TopicID() uuid.UUID {
	return a.results.topicID
}

func (a *CreateSubscription) SubscriptionID() uuid.UUID {
	return a.results.id
}

func (a *CreateSubscription) Subscription() *ent.Subscription {
	return a.results.sub
}

func (a *CreateSubscription) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"topicName": a.params.TopicName,
		"name":      a.params.Name,
	}
}

func (a *CreateSubscription) HasResults() bool {
	return a.results != nil
}

func (a *CreateSubscription) Results() map[string]interface{} {
	return map[string]interface{}{
		"id":      a.results.id,
		"topicID": a.results.topicID,
		// ent object is intentionally omitted here
	}
}
