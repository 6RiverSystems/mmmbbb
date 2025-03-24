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
	"hash/crc32"
	"math"
	"sort"
	"time"
	"unsafe"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"go.6river.tech/mmmbbb/db/postgres"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/predicate"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/parse"
)

type GetSubscriptionMessagesParams struct {
	Name           string
	ID             *uuid.UUID
	MaxMessages    int
	MaxBytes       int
	MaxBytesStrict bool
	MaxWait        time.Duration
	// Waiting, if non-nil, will be closed when the action first goes into a delay
	Waiting chan<- struct{} `json:"-"`
}
type SubscriptionMessageDelivery struct {
	ID          uuid.UUID `json:"id"`
	MessageID   uuid.UUID `json:"messageID"`
	PublishedAt time.Time `json:"publishedAt"`
	// NumAttempts is approximate
	NumAttempts   int       `json:"numAttempts"`
	NextAttemptAt time.Time `json:"nextAttemptAt"`
	fuzzDelay     time.Duration
	OrderKey      *string           `json:"orderKey,omitempty"`
	Payload       json.RawMessage   `json:"payload"`
	Attributes    map[string]string `json:"attributes"`
}
type getSubscriptionMessagesResults struct {
	Deliveries      []*SubscriptionMessageDelivery
	NumDeadLettered int
}
type GetSubscriptionMessages struct {
	actionBase[GetSubscriptionMessagesParams, getSubscriptionMessagesResults]
}

var _ Action[GetSubscriptionMessagesParams, getSubscriptionMessagesResults] = (*GetSubscriptionMessages)(nil)

func NewGetSubscriptionMessages(params GetSubscriptionMessagesParams) *GetSubscriptionMessages {
	if params.MaxMessages < 1 {
		panic(errors.New("maxMessages must be > 0"))
	}
	if params.MaxBytes < 1 {
		panic(errors.New("maxBytes must be > 0"))
	}
	if params.MaxWait < 0 {
		// we allow 0 here as "use the default"
		panic(errors.New("maxWait must be >= 0"))
	}
	if params.Name == "" && params.ID == nil {
		panic(errors.New("must provide Name or ID"))
	}
	return &GetSubscriptionMessages{
		actionBase[GetSubscriptionMessagesParams, getSubscriptionMessagesResults]{
			params: params,
		},
	}
}

// Deprecated: use ExecuteClient instead.
//
// Using the standard Execute method will cause excessive consumption of
// database connections, as the transaction will be held open until message(s)
// are received, or the call times out. This also simply will not work correctly
// with SQLite. Using ExecuteClient will avoid holding a connection open while
// waiting for messages, and will work properly with SQLite.
func (a *GetSubscriptionMessages) Execute(ctx context.Context, tx *ent.Tx) error {
	return a.execute(ctx, tx, func(f func(*ent.Tx) error) error {
		return f(tx)
	})
}

func (a *GetSubscriptionMessages) ExecuteClient(ctx context.Context, client *ent.Client) error {
	return a.execute(ctx, nil, func(f func(*ent.Tx) error) error {
		return client.DoCtxTxRetry(
			ctx,
			nil,
			func(ctx context.Context, tx *ent.Tx) error { return f(tx) },
			postgres.RetryOnErrorCode(postgres.DeadlockDetected),
		)
	})
}

func (a *GetSubscriptionMessages) execute(
	ctx context.Context,
	timerTx *ent.Tx,
	runTx func(func(tx *ent.Tx) error) error,
) error {
	timer := startActionTimer(getSubscriptionMessagesHistogram, timerTx)
	defer timer.Ended()

	findTimeout := a.initializeMaxWait()

	var pubAwaiter PublishNotifier
	defer func() {
		if a.params.ID != nil {
			CancelPublishAwaiter(*a.params.ID, pubAwaiter)
		}
	}()

	if err := runTx(func(tx *ent.Tx) error {
		// verify the subscription and, if valid, update its expiration, in case we
		// don't get to do so via successful-ish return through applyResults
		if sub, err := a.verifySub(ctx, tx); err != nil {
			return err
		} else if err := tx.Subscription.UpdateOne(sub).
			SetExpiresAt(time.Now().Add(time.Duration(sub.TTL))).
			Exec(ctx); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

RETRY:
	for {
		var sub *ent.Subscription
		var next *time.Time

		// initialize the publish awaiter before we query, to avoid a race where a
		// publish happens between receiving empty query results and initializing
		// this waiter. IMPORTANT: this has to be _before_ we potentially start a
		// txn, at least for SQLite. Which means we need an extra Tx the first time
		// around.
		// cancel old awaiter before we create a new one
		CancelPublishAwaiter(*a.params.ID, pubAwaiter)
		pubAwaiter = PublishAwaiter(*a.params.ID)

		err := runTx(func(tx *ent.Tx) error {
			// re-check the sub before we query it, to ensure it is still valid, abort
			// if it's e.g. deleted
			var err error
			if sub, err = a.verifySub(ctx, tx); err != nil {
				return err
			}

			deliveries, err := a.queryAndLockDeliveriesOnce(ctx, tx, sub)
			if err != nil {
				return err
			}

			// SQLite is always serialized, so retrying within the same transaction
			// won't help, but if we aren't bound to a single transaction, then we can
			// do retries
			if len(deliveries) != 0 || timerTx != nil && timerTx.Dialect() == dialect.SQLite {
				if err = a.applyResults(ctx, tx, sub, deliveries); err != nil {
					return err
				}
				timer.Succeeded(func() { getSubscriptionMessagesCounter.Add(float64(len(a.results.Deliveries))) })
			} else {
				if next, err = a.nextAttempt(ctx, tx, sub); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			timer.ReportRollback()
			return err
		}
		if a.results != nil {
			timer.ReportCommit()
			return nil
		}

		// nothing found: instead of having the client thrash, wait for a
		// notification that there is a new message or that the sub changed (sub
		// changes will fire the publish awaiter too), or for the next attempt for
		// some message to be due

		var nextAttempt <-chan time.Time
		if next != nil {
			delay := time.Until(*next)
			nextAttempt = time.After(delay)
		}

		if a.params.Waiting != nil {
			close(a.params.Waiting)
			a.params.Waiting = nil
		}

		select {
		case <-findTimeout:
			// max wait has elapsed, give up, return empty results
			return runTx(func(tx *ent.Tx) error {
				return a.applyResults(ctx, tx, sub, nil)
			})
		case <-nextAttempt:
			continue RETRY
		case <-pubAwaiter:
			continue RETRY
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (a *GetSubscriptionMessages) initializeMaxWait() <-chan time.Time {
	// after this long, we just give up. chosen to avoid a 60 second
	// idle-in-transaction timeout
	maxWait := time.Minute - time.Second
	if a.params.MaxWait > 0 {
		maxWait = a.params.MaxWait
	}
	return time.After(maxWait)
}

func (a *GetSubscriptionMessages) verifySub(
	ctx context.Context,
	tx *ent.Tx,
) (*ent.Subscription, error) {
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
			return nil, ErrNotFound
		}
		return nil, err
	}
	// bind subscription UUID for future checks
	a.params.ID = &sub.ID
	return sub, nil
}

func (a *GetSubscriptionMessages) queryAndLockDeliveriesOnce(
	ctx context.Context,
	tx *ent.Tx,
	sub *ent.Subscription,
) ([]*ent.Delivery, error) {
	// fresh now for every retry
	now := time.Now()

	q := a.buildDeliveryQuery(tx, sub, delivery.AttemptAtLTE(now))
	// SQLite doesn't support `for update`, and doesn't need it because its
	// always doing SERIALIZABLE transactions
	if tx.Dialect() != dialect.SQLite {
		q = q.ForUpdate(
			// we don't need to lock the messages table, just the deliveries we're
			// about to try to deliver. also if we self-joined to deliveries for the
			// no-before, this will only lock the delivery we are considering due to
			// the other copy having a table alias.
			sql.WithLockTables(delivery.Table),
			sql.WithLockAction(sql.SkipLocked),
		)
	}
	deliveries, err := q.
		// this helps the query planner to know that it should use the index. not
		// sure why it needs this hint, but it seems to help.
		Order(delivery.ByAttemptAt()).
		Limit(a.params.MaxMessages).
		WithMessage().
		All(ctx)
	if err != nil {
		return nil, err
	}
	return deliveries, nil
}

func (a *GetSubscriptionMessages) buildDeliveryQuery(
	tx *ent.Tx,
	sub *ent.Subscription,
	extraPredicates ...predicate.Delivery,
) *ent.DeliveryQuery {
	now := time.Now()

	predicates := make([]predicate.Delivery, 0, len(extraPredicates)+4)
	// only add the in-order check if we need it, as this generates some complex
	// and performance-impacting SQL. we have to do this as a custom-ish predicate
	// to get the join predicate applied to the "root" selector
	if sub.OrderedDelivery {
		// check the delivery we're looking at either (a) has no "not before"
		// predecessor delivery, or that predecessor is completed or expired.
		predicates = append(predicates, func(s *sql.Selector) {
			// do this as a left join instead of an "in" and a sub-select: left join
			// requires reading at most one more row per candidate in this
			// subscription (pending delivery), sub-select requires reading all the
			// completed and expired deliveries _for all subscriptions_.
			t := sql.Table(delivery.Table).As("dnb")
			s.LeftJoin(t).On(s.C(delivery.NotBeforeColumn), t.C(delivery.FieldID))
			s.Where(sql.Or(
				sql.IsNull(s.C(delivery.NotBeforeColumn)),
				sql.NotNull(t.C(delivery.FieldCompletedAt)),
				sql.LTE(t.C(delivery.FieldExpiresAt), now),
			))
		})
	}
	predicates = append(predicates, extraPredicates...)
	predicates = append(predicates,
		delivery.CompletedAtIsNil(),
		delivery.ExpiresAtGT(now),
		delivery.SubscriptionID(sub.ID),
	)

	return tx.Delivery.Query().Where(predicates...)
}

// nextAttempt looks up the time at which the next eligible message will be
// ready for an attempted delivery, or when the next outstanding message
// expires, whichever is first, nor nil if there are no outstanding messages
func (a *GetSubscriptionMessages) nextAttempt(
	ctx context.Context,
	tx *ent.Tx,
	sub *ent.Subscription,
) (*time.Time, error) {
	nextAttempt, err := a.buildDeliveryQuery(tx, sub).
		Order(ent.Asc(delivery.FieldAttemptAt)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	nextExpires, err := a.buildDeliveryQuery(tx, sub).
		Order(ent.Asc(delivery.FieldAttemptAt)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	next := nextAttempt.AttemptAt
	if nextExpires.ExpiresAt.Before(next) {
		next = nextExpires.ExpiresAt
	}

	return &next, nil
}

func (a *GetSubscriptionMessages) applyResults(
	ctx context.Context,
	tx *ent.Tx,
	sub *ent.Subscription,
	deliveries []*ent.Delivery,
) error {
	bytes := 0
	now := time.Now()

	// refresh the subscription expiration
	err := tx.Subscription.UpdateOne(sub).
		SetExpiresAt(now.Add(time.Duration(sub.TTL))).
		Exec(ctx)
	if err != nil {
		return err
	}

	results := &getSubscriptionMessagesResults{
		Deliveries: make([]*SubscriptionMessageDelivery, 0, len(deliveries)),
	}

	hasDeadLettering := sub.HasFullDeadLetterConfig()

	// can't do a bulk update of the deliveries, because their nextAttempt
	// computations will be different, so we have to update them one by one
	for i, d := range deliveries {
		// unless in strict mode, always deliver at least one message, even if it
		// exceeds maxBytes
		if (a.params.MaxBytesStrict || i > 0) &&
			bytes+len(d.Edges.Message.Payload) > a.params.MaxBytes {
			// keep trying other messages, maybe they will fit within the budget
			continue
		}

		// if we missed the nack and this delivery has exceeded its attempt limit,
		// dead letter it instead of delivering it
		if hasDeadLettering && d.Attempts >= int(*sub.MaxDeliveryAttempts) {
			if err = deadLetterDelivery(
				ctx,
				tx,
				deadLetterDataFromEntities(d, sub),
				now,
				"actions/get-subscription-messages",
			); err != nil {
				return err
			}
			results.NumDeadLettered++
			continue
		}

		nominalDelay, fuzzedDelay := NextDelayFor(sub, d.Attempts+1)

		delivery := &SubscriptionMessageDelivery{
			ID:            d.ID,
			MessageID:     d.Edges.Message.ID,
			PublishedAt:   d.Edges.Message.PublishedAt,
			NextAttemptAt: now.Add(nominalDelay),
			fuzzDelay:     fuzzedDelay - nominalDelay,
			OrderKey:      d.Edges.Message.OrderKey,
			NumAttempts:   d.Attempts + 1,
			Payload:       d.Edges.Message.Payload,
			Attributes:    d.Edges.Message.Attributes,
		}

		results.Deliveries = append(results.Deliveries, delivery)
		bytes += len(d.Edges.Message.Payload)

		if i+1 >= a.params.MaxMessages {
			// just in case, but should never get here due to limit in query
			break
		}
		// bytes check is at the top of the loop
	}

	// workaround for https://github.com/ent/ent/issues/358: avoid deadlocks by
	// touching deliveries in sorted order by uuid
	// TODO: some tests expect low throughput stuff to come in order, so we have
	// to do this on a copy of the array; those tests should be fixed, but doing
	// so is non-trivial.
	sortedDeliveries := make([]*SubscriptionMessageDelivery, len(results.Deliveries))
	copy(sortedDeliveries, results.Deliveries)
	sort.Slice(
		sortedDeliveries,
		func(i, j int) bool { return parse.UUIDLess(deliveries[i].ID, deliveries[j].ID) },
	)

	for _, d := range sortedDeliveries {
		if err := tx.Delivery.UpdateOneID(d.ID).
			SetLastAttemptedAt(now).
			AddAttempts(1).
			SetAttemptAt(d.NextAttemptAt.Add(d.fuzzDelay)).
			Exec(ctx); err != nil {
			return err
		}
	}

	a.results = results
	return nil
}

// defaultMinDelay is the default ack timeout for subscriptions that don't
// configure a backoff. This is currently 10 seconds, because the Google client
// will immediately mod-ack everything it receives up to that, so there's no
// point in having it set lower here.
const defaultMinDelay = 10 * time.Second

const (
	defaultMaxDelay    = 10 * time.Minute
	retryBackoffFactor = 1.1
)

// delay after N attempts = floor(max, min * factor^N), AKA after first attempt
// delay is min, after each further attempt delay *= factor, until delay hits
// max
func NextDelayFor(sub *ent.Subscription, attempts int) (nominalDelay, fuzzedDelay time.Duration) {
	min, max := defaultMinDelay, defaultMaxDelay
	if sub.MinBackoff != nil && *sub.MinBackoff > 0 {
		min = time.Duration(*sub.MinBackoff)
	}
	if sub.MaxBackoff != nil && *sub.MaxBackoff > 0 {
		max = time.Duration(*sub.MaxBackoff)
	}
	delay := math.Pow(retryBackoffFactor, float64(attempts)) * min.Seconds()
	if delay > max.Seconds() {
		delay = max.Seconds()
	}
	var fuzzNanos uint32
	// add up to 1 second of fuzz if we are doing at least 0.5 seconds of nominal
	// delay
	if delay > 0.5 {
		// be stochastic, but deterministic about fuzz
		h := crc32.NewIEEE()
		h.Write(sub.ID[:])
		attempts32 := int32(attempts)
		h.Write((*(*[4]byte)(unsafe.Pointer(&attempts32)))[:])
		fuzzNanos = h.Sum32() % uint32(time.Second)
	}

	nominalDelay = time.Duration(delay * float64(time.Second))
	fuzzedDelay = nominalDelay + time.Duration(fuzzNanos)
	return
}
