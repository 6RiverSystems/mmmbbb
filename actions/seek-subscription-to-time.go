package actions

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/predicate"
	"go.6river.tech/mmmbbb/ent/subscription"
)

type SeekSubscriptionToTimeParams struct {
	Name string
	ID   *uuid.UUID
	Time time.Time
}

type seekSubscriptionToTimeResults struct {
	numDeliveries int
}

type SeekSubscriptionToTime struct {
	params  SeekSubscriptionToTimeParams
	results *seekSubscriptionToTimeResults
}

var _ Action = (*SeekSubscriptionToTime)(nil)

func NewSeekSubscriptionToTime(params SeekSubscriptionToTimeParams) *SeekSubscriptionToTime {
	if params.Name == "" && params.ID == nil {
		panic(errors.New("Must provide Name or ID"))
	}
	if params.Time.IsZero() || (params.Time == time.Time{}) {
		panic(errors.New("Target time must be specified as a non-zero value"))
	}
	// FUTURE: do we want to bound how far in the future or past the target can be?
	return &SeekSubscriptionToTime{
		params: params,
	}
}

func (a *SeekSubscriptionToTime) Execute(ctx context.Context, tx *ent.Tx) error {
	subMatch := []predicate.Subscription{subscription.DeletedAtIsNil()}
	if a.params.ID != nil {
		subMatch = append(subMatch, subscription.ID(*a.params.ID))
	}
	if a.params.Name != "" {
		subMatch = append(subMatch, subscription.Name(a.params.Name))
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
	a.params.ID, a.params.Name = &sub.ID, sub.Name
	now := time.Now()
	numUpdated, err := tx.Delivery.Update().
		Where(
			delivery.SubscriptionID(sub.ID),
			delivery.ExpiresAtGTE(now),
			delivery.CompletedAtGTE(a.params.Time),
		).
		ClearCompletedAt().
		SetExpiresAt(now.Add(time.Duration(sub.MessageTTL))).
		SetAttemptAt(now).
		Save(ctx)
	if err != nil {
		return err
	}

	// we just un-ack'd some deliveries, so wake up any listeners
	if numUpdated != 0 {
		notifyPublish(tx, sub.ID)
	}

	a.results = &seekSubscriptionToTimeResults{
		numDeliveries: numUpdated,
	}

	return nil
}

func (a *SeekSubscriptionToTime) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"id":   a.params.ID,
		"name": a.params.Name,
		"time": a.params.Time,
	}
}

func (a *SeekSubscriptionToTime) HasResults() bool {
	return a.results != nil
}

func (a *SeekSubscriptionToTime) NumDeliveries() int {
	return a.results.numDeliveries
}

func (a *SeekSubscriptionToTime) Results() map[string]interface{} {
	return map[string]interface{}{
		"numDeliveries": a.results.numDeliveries,
	}
}
