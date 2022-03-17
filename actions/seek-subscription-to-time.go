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
	numAcked   int
	numDeAcked int
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

	// the rule for seek to time is that everything published before the given
	// time is marked as acknowledged, and everything published after that time is
	// marked as un-acknowledged

	numAcked, err := tx.Delivery.Update().
		Where(
			delivery.SubscriptionID(sub.ID),
			delivery.ExpiresAtGTE(now),
			delivery.PublishedAtLTE(a.params.Time),
			delivery.CompletedAtIsNil(),
		).
		SetCompletedAt(now).
		Save(ctx)
	if err != nil {
		return err
	}
	numDeAcked, err := tx.Delivery.Update().
		Where(
			delivery.SubscriptionID(sub.ID),
			delivery.ExpiresAtGTE(now),
			delivery.PublishedAtGT(a.params.Time),
			delivery.CompletedAtNotNil(),
		).
		ClearCompletedAt().
		SetExpiresAt(now.Add(time.Duration(sub.MessageTTL))).
		SetAttemptAt(now).
		Save(ctx)
	if err != nil {
		return err
	}

	// we modified any deliveries, wake up any listeners. obviously we need to do
	// so if we de-acked some, but acking might also wake a listener, if any of
	// those were blocking later deliveries in an ordered subscription
	if numAcked != 0 || numDeAcked != 0 {
		notifyPublish(tx, sub.ID)
	}

	a.results = &seekSubscriptionToTimeResults{
		numAcked:   numAcked,
		numDeAcked: numDeAcked,
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

func (a *SeekSubscriptionToTime) NumAcked() int {
	return a.results.numAcked
}

func (a *SeekSubscriptionToTime) NumDeAcked() int {
	return a.results.numDeAcked
}

func (a *SeekSubscriptionToTime) Results() map[string]interface{} {
	return map[string]interface{}{
		"numAcked":   a.results.numAcked,
		"numDeAcked": a.results.numDeAcked,
	}
}
