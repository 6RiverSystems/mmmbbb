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

	"go.6river.tech/gosix/db/postgres"
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
	Waiting chan<- struct{}
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
	deliveries []*SubscriptionMessageDelivery
}
type GetSubscriptionMessages struct {
	params  GetSubscriptionMessagesParams
	results *getSubscriptionMessagesResults
}

var _ Action = (*GetSubscriptionMessages)(nil)

func NewGetSubscriptionMessages(params GetSubscriptionMessagesParams) *GetSubscriptionMessages {
	if params.MaxMessages < 1 {
		panic(errors.New("MaxMessages must be > 0"))
	}
	if params.MaxBytes < 1 {
		panic(errors.New("MaxBytes must be > 0"))
	}
	if params.MaxWait < 0 {
		// we allow 0 here as "use the default"
		panic(errors.New("MaxWait must be >= 0"))
	}
	if params.Name == "" && params.ID == nil {
		panic(errors.New("Must provide Name or ID"))
	}
	return &GetSubscriptionMessages{
		params: params,
	}
}

var getSubscriptionMessagesCounter, getSubscriptionMessagesHistogram = actionMetrics("get_subscription_messages", "messages", "delivered")

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

RETRY:
	for {
		var sub *ent.Subscription
		var next *time.Time

		// initialize the publish awaiter before we query, to avoid a race where a
		// publish happens between receiving empty query results and initializing
		// this waiter. IMPORTANT: this has to be _before_ we potentially start a
		// txn, at least for SQLite. Which means we need an extra Tx the first time
		// around.
		if a.params.ID == nil {
			if err := runTx(func(tx *ent.Tx) error {
				_, err := a.verifySub(ctx, tx)
				return err
			}); err != nil {
				return err
			}
		}
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
			if err = a.acquireLock(ctx, tx); err != nil {
				return err
			}

			deliveries, err := a.queryDeliveriesOnce(ctx, tx, sub)
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
				timer.Succeeded(func() { getSubscriptionMessagesCounter.Add(float64(len(a.results.deliveries))) })
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

func (a *GetSubscriptionMessages) acquireLock(
	ctx context.Context,
	tx *ent.Tx,
) error {
	if tx.Dialect() != dialect.Postgres {
		return nil
	}

	// use the top 16 and bottom 48 bits (4 & 12 nibbles=hex-chars) of the UUID
	// as the hash for advisory locks. we have to cast this back down to signed
	// int64 for compat with pg
	subIDHash := int64(
		uint64(*(*uint16)((unsafe.Pointer)(&a.params.ID[0])))<<48 |
			(*(*uint64)((unsafe.Pointer)(&a.params.ID[8])) & 0x0000ffff_ffffffff))

	// acquire an advisory lock on the subscription we're monitoring, so that
	// concurrent delivery fetchers for it won't pull the same messages
	if _, err := tx.DBTx().ExecContext(ctx, "select pg_advisory_xact_lock($1)", subIDHash); err != nil {
		return err
	}
	return nil
}

func (a *GetSubscriptionMessages) queryDeliveriesOnce(
	ctx context.Context,
	tx *ent.Tx,
	sub *ent.Subscription,
) ([]*ent.Delivery, error) {
	// fresh now for every retry
	now := time.Now()

	deliveries, err := a.buildDeliveryQuery(tx, sub, delivery.AttemptAtLTE(now)).
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
		// this avoids a useless join against the subscriptions table
		predicate.Delivery(func(s *sql.Selector) {
			s.Where(sql.EQ(s.C(delivery.SubscriptionColumn), sub.ID))
		}),
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
		// TODO: ent.Asc doesn't work right for SQLite here due to the self join,
		// see: https://github.com/ent/ent/issues/1265
		// Order(ent.Asc(delivery.FieldAttemptAt)).
		Order(func(s *sql.Selector, f func(string) bool) {
			sql.Asc(delivery.Table + "." + delivery.FieldAttemptAt)
		}).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	nextExpires, err := a.buildDeliveryQuery(tx, sub).
		// TODO: ent.Asc doesn't work right for SQLite here due to the self join,
		// see: https://github.com/ent/ent/issues/1265
		// Order(ent.Asc(delivery.FieldAttemptAt)).
		Order(func(s *sql.Selector, f func(string) bool) {
			sql.Asc(delivery.Table + "." + delivery.FieldAttemptAt)
		}).
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
		SetExpiresAt(now.Add(sub.TTL.Duration)).
		Exec(ctx)
	if err != nil {
		return err
	}

	results := &getSubscriptionMessagesResults{
		deliveries: make([]*SubscriptionMessageDelivery, 0, len(deliveries)),
	}

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

		results.deliveries = append(results.deliveries, delivery)
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
	sortedDeliveries := make([]*SubscriptionMessageDelivery, len(results.deliveries))
	copy(sortedDeliveries, results.deliveries)
	sort.Slice(sortedDeliveries, func(i, j int) bool { return parse.UUIDLess(deliveries[i].ID, deliveries[j].ID) })

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

func (a *GetSubscriptionMessages) Deliveries() []*SubscriptionMessageDelivery {
	return a.results.deliveries
}

func (a *GetSubscriptionMessages) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"name":        a.params.Name,
		"id":          a.params.ID,
		"maxMessages": a.params.MaxMessages,
		"maxBytes":    a.params.MaxBytes,
	}
}

func (a *GetSubscriptionMessages) HasResults() bool {
	return a.results != nil
}

func (a *GetSubscriptionMessages) Results() map[string]interface{} {
	return map[string]interface{}{
		"deliveries": a.results.deliveries,
	}
}

// defaultMinDelay is the default ack timeout for subscriptions that don't
// configure a backoff. This is currently 10 seconds, because the Google client
// will immediately modack everything it receives up to that, so there's no
// point in having it set lower here.
const defaultMinDelay = 10 * time.Second
const defaultMaxDelay = 10 * time.Minute
const retryBackoffFactor = 1.1

// delay after N attempts = floor(max, min * factor^N), AKA after first attempt
// delay is min, after each further attempt delay *= factor, until delay hits
// max
func NextDelayFor(sub *ent.Subscription, attempts int) (nominalDelay, fuzzedDelay time.Duration) {
	min, max := defaultMinDelay, defaultMaxDelay
	if sub.MinBackoff != nil && sub.MinBackoff.Duration != nil && *sub.MinBackoff.Duration > 0 {
		min = *sub.MinBackoff.Duration
	}
	if sub.MaxBackoff != nil && sub.MaxBackoff.Duration != nil && *sub.MaxBackoff.Duration > 0 {
		max = *sub.MaxBackoff.Duration
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
