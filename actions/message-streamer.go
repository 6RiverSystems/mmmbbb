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
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/logging"
)

// TODO: this isn't actually an Action, but it doesn't really have a better
// place to live right now

type MessageStreamer struct {
	Client           *ent.Client
	Logger           *logging.Logger
	SubscriptionID   *uuid.UUID
	SubscriptionName string
	AutomaticNack    bool
}

type StreamConnection interface {
	Close() error
	Receive(context.Context) (*MessageStreamRequest, error)
	Send(context.Context, *SubscriptionMessageDelivery) error
}
type StreamConnectionBatchSend interface {
	SendBatch(context.Context, []*SubscriptionMessageDelivery) error
}

type MessageStreamRequest struct {
	FlowControl *FlowControl `json:"flowControl"`
	Ack         []uuid.UUID  `json:"ack"`
	Nack        []uuid.UUID  `json:"nack"`
	Delay       []uuid.UUID  `json:"delay"`
	// do this as a number to avoid issues & cross-platform/language
	// inconsistencies with duration serialization
	DelaySeconds float64 `json:"delaySeconds"`
}

type pendingMessage struct {
	bytes         int
	nextAttemptAt time.Time
}

func (ms *MessageStreamer) Go(ctx context.Context, conn StreamConnection) error {
	if ms.SubscriptionID == nil {
		id, err := ms.Client.Subscription.Query().
			Where(
				subscription.Name(ms.SubscriptionName),
				subscription.DeletedAtIsNil(),
			).
			OnlyID(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		ms.SubscriptionID = &id
	} else if ms.SubscriptionName == "" {
		s, err := ms.Client.Subscription.Query().
			Where(
				subscription.ID(*ms.SubscriptionID),
				subscription.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		ms.SubscriptionName = s.Name
	}
	if ms.Logger == nil {
		ms.Logger = logging.GetLoggerWith("actions/message-streamer", func(c zerolog.Context) zerolog.Context {
			return c.Str("subscription", ms.SubscriptionName)
		})
	}

	eg, ctx := errgroup.WithContext(ctx)

	var mu sync.Mutex
	// until the client tells us otherwise, use the most conservative possible
	// flow control settings that will still result in messages being delivered
	fc := FlowControl{
		MaxMessages: 1,
		MaxBytes:    1,
	}
	// holds pointers so we can modify the pending state in-place more easily
	pending := map[uuid.UUID]*pendingMessage{}
	wakeSend := make(chan struct{}, 1)
	tryWake := func() {
		select {
		case wakeSend <- struct{}{}:
		default:
		}
	}
	defer close(wakeSend)

	// close the connection when the context is canceled, so the reader exits
	eg.Go(func() error {
		<-ctx.Done()
		conn.Close()
		tryWake()
		return ctx.Err()
	})

	// message reader
	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				tryWake()
				return ctx.Err()
			default:
			}
			var msg *MessageStreamRequest
			var err error
			if msg, err = conn.Receive(ctx); err != nil {
				// connections ending with an error is normal, no reason to log here
				return err
			}
			if msg.FlowControl != nil {
				mu.Lock()
				fc = *msg.FlowControl
				tryWake()
				mu.Unlock()
			}
			if len(msg.Ack) != 0 || len(msg.Nack) != 0 {
				if err := ms.doAcksNacks(ctx, msg.Ack, msg.Nack); err != nil {
					return err
				}
				mu.Lock()
				for _, id := range msg.Ack {
					delete(pending, id)
				}
				for _, id := range msg.Nack {
					delete(pending, id)
				}
				tryWake()
				mu.Unlock()
			}
			if len(msg.Delay) != 0 {
				if _, err := ms.doDelay(ctx, msg.Delay, time.Duration(msg.DelaySeconds*float64(time.Second))); err != nil {
					return err
				}
			}
		}
	})

	// message retriever / sender
	eg.Go(func() error {
		pubNotify := PublishAwaiter(*ms.SubscriptionID)
		defer func() { CancelPublishAwaiter(*ms.SubscriptionID, pubNotify) }()
		for {
			var curFc FlowControl
			anyPending := false
			for {
				// need to re-check context cancellation every time
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				mu.Lock()
				curFc = fc
				// compute how much of the flow limits is used up
				for _, pm := range pending {
					anyPending = true
					curFc.MaxMessages--
					curFc.MaxBytes -= pm.bytes
				}
				mu.Unlock()
				if curFc.MaxBytes > 0 && curFc.MaxMessages > 0 {
					// OK to send at least one message
					break
				}
				ms.Logger.Trace().Interface("flowControl", curFc).Msg("Flow control full, waiting for wakeup")
				waitTimer := prometheus.NewTimer(messageStreamerFlowControlWait)
				// need to wait for some acks before we can send
				select {
				case <-wakeSend:
				case <-pubNotify:
					// notifiers are single-use, so get a new one before we proceed
					pubNotify = PublishAwaiter(*ms.SubscriptionID)
				}
				waitTimer.ObserveDuration()
			}

			ms.Logger.Trace().Interface("flowControl", curFc).Msg("Flow control ready, fetching messages")

			p := GetSubscriptionMessagesParams{
				ID:          ms.SubscriptionID,
				Name:        ms.SubscriptionName,
				MaxMessages: curFc.MaxMessages,
				MaxBytes:    curFc.MaxBytes,
				// GSM will always try to give us one message, even if it exceeds
				// MaxBytes. For one-shot retrievals, this is desirable. But for
				// streaming like this, we want a "strict" mode, once we have at least
				// one message outstanding
				MaxBytesStrict: anyPending,
			}
			// avoid pulling too many at a time, as it causes thundering herd and head
			// of line problems
			if p.MaxMessages > 100 {
				p.MaxMessages = 100
			}
			getter := NewGetSubscriptionMessages(p)
			if err := getter.ExecuteClient(ctx, ms.Client); err != nil {
				return err
			}

			results, _ := getter.Results()
			mu.Lock()
			for _, del := range results.Deliveries {
				pending[del.ID] = &pendingMessage{
					bytes:         len(del.Payload),
					nextAttemptAt: del.NextAttemptAt,
				}
			}
			mu.Unlock()

			if len(results.Deliveries) != 0 {
				ms.Logger.Trace().
					// Interface("delivery", results.Deliveries[0]).
					Int("numDeliveries", len(results.Deliveries)).
					Msg("Streaming some deliveries")
				if sb, ok := conn.(StreamConnectionBatchSend); ok {
					if err := sb.SendBatch(ctx, results.Deliveries); err != nil {
						return err
					}
				} else {
					for _, d := range results.Deliveries {
						if err := conn.Send(ctx, d); err != nil {
							return err
						}
					}
				}
			}
		}
	})

	// refresh pending map from DB when things happen
	eg.Go(func() error {
		ids := []uuid.UUID{}
		pubNotify := PublishAwaiter(*ms.SubscriptionID)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-pubNotify:
				// single use, refresh it
				pubNotify = PublishAwaiter(*ms.SubscriptionID)
			}
			// we expect external ack/nack, so we need to rebuild pending from the
			// db state when we get a notification
			now := time.Now()
			ids = ids[:0]
			mu.Lock()
			for id := range pending {
				ids = append(ids, id)
			}
			mu.Unlock()
			if len(ids) == 0 {
				// wait for the next wakeup
				continue
			}
			stillPendingDeliveries, err := ms.Client.Delivery.Query().
				Where(
					delivery.IDIn(ids...),
					delivery.CompletedAtIsNil(),
					delivery.ExpiresAtGTE(now),
				).
				All(ctx)
			if err != nil {
				return err
			}
			deliveryMap := make(map[uuid.UUID]*ent.Delivery, len(stillPendingDeliveries))
			for _, del := range stillPendingDeliveries {
				deliveryMap[del.ID] = del
			}
			mu.Lock()
			removedPending := false
			for _, id := range ids {
				if del, ok := deliveryMap[id]; !ok {
					delete(pending, id)
					removedPending = true
				} else if p, ok := pending[id]; ok && p.nextAttemptAt.Before(del.AttemptAt) {
					p.nextAttemptAt = del.AttemptAt
				}
			}
			mu.Unlock()
			if removedPending {
				tryWake()
			}
		}
	})

	if !ms.AutomaticNack {
		// auto-delays: try to automatically delay messages client is still working on
		// before they become eligible for retry. this is best effort and not
		// guaranteed to always catch it in time under load
		eg.Go(func() error {
			// TODO: this won't notice if sub backoff params change mid-stream
			var delayAmount time.Duration
			if s, err := ms.Client.Subscription.Get(ctx, *ms.SubscriptionID); err != nil {
				return err
			} else if s.MinBackoff != nil {
				delayAmount = time.Duration(*s.MinBackoff) / 2
			} else {
				delayAmount = defaultMinDelay / 2
			}
			checkInterval := delayAmount * 9 / 10
			if delayAmount < time.Second {
				// set a min for this, but _after_ we compute checkInterval
				delayAmount = time.Second
			}
			ticker := time.NewTicker(checkInterval)
			ids := []uuid.UUID{}
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
					// break & fall through
				}
				now := time.Now()
				ids = ids[:0]
				mu.Lock()
				nextCheck := now.Add(checkInterval)
				for id, pm := range pending {
					// renew (delay) anything that will expire before we next check, not
					// just anything that is expired already
					if pm.nextAttemptAt.Before(nextCheck) {
						ids = append(ids, id)
					}
				}
				mu.Unlock()
				if len(ids) == 0 {
					// loop back to ticker wait
					continue
				}

				if _, err := ms.doDelay(ctx, ids, delayAmount); err != nil {
					return err
				}
				messageStreamerAutoDelays.Add(float64(len(ids)))
				// update our in-mem state with the (approximate) next attempt. need to be
				// aware that some of these may have been acked or nacked in the meantime
				nextAttempt := now.Add(delayAmount)
				mu.Lock()
				// handling the case where some but not all were externally ACKed is
				// handled in the flow control section
				for _, id := range ids {
					if pm, ok := pending[id]; ok && pm.nextAttemptAt.Before(nextAttempt) {
						pm.nextAttemptAt = nextAttempt
					}
				}
				mu.Unlock()
			}
		})
	}

	err := eg.Wait()
	if err != nil && errors.Is(err, io.EOF) {
		// not really a problem
		return nil
	}
	return err
}

func (ms *MessageStreamer) doAcksNacks(ctx context.Context, ackIDs, nackIDs []uuid.UUID) error {
	if len(ackIDs) == 0 && len(nackIDs) == 0 {
		return nil
	}

	ack := NewAckDeliveries(ackIDs...)
	nack := NewNackDeliveries(nackIDs...)
	err := ms.Client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		if err := ack.Execute(ctx, tx); err != nil {
			return err
		}
		if err := nack.Execute(ctx, tx); err != nil {
			return err
		}
		return nil
	})
	evt := ms.Logger.Trace()
	if err != nil {
		evt = ms.Logger.Error().Err(err)
	}
	if ackRes, ok := ack.Results(); ok && len(ackIDs) != 0 {
		evt.Int("attemptedAcks", len(ackIDs)).Int("acked", ackRes.NumAcked)
	}
	if results, ok := nack.Results(); ok && len(nackIDs) != 0 {
		evt.Int("attemptedNacks", len(nackIDs)).Int("nacked", results.NumNacked)
	}
	evt.Msg("Processed ACK/NACK request")
	return err
}

func (ms *MessageStreamer) doDelay(ctx context.Context, ids []uuid.UUID, delay time.Duration) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	dd := NewDelayDeliveries(DelayDeliveriesParams{
		IDs:   ids,
		Delay: delay,
	})
	err := ms.Client.DoCtxTx(ctx, nil, dd.Execute)
	evt := ms.Logger.Trace()
	if err != nil {
		evt = ms.Logger.Error().Err(err)
	}
	evt = evt.Int("attempted", len(ids))
	var numDelayed int
	if results, ok := dd.Results(); ok {
		numDelayed = results.NumDelayed
		evt = evt.Int("delayed", numDelayed)
	}
	evt.Msg("Processed delays")
	return numDelayed, err
}
