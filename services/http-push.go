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

package services

import (
	"context"
	"reflect"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"go.6river.tech/mmmbbb/actions"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/logging"
)

type monitoredPusher struct {
	*errgroup.Group
	context.Context
	*actions.HttpPushStreamer
	cancel func()
}

func monitorPusher(ctx context.Context, pusher *actions.HttpPushStreamer) monitoredPusher {
	ctx, cancel := context.WithCancel(ctx)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error { return pusher.Go(egCtx) })
	return monitoredPusher{eg, egCtx, pusher, cancel}
}

func waitPusherMonitors(
	ctx context.Context,
	timeout time.Duration,
	notifier <-chan struct{},
	mons map[uuid.UUID]monitoredPusher,
) waitMonitorResult {
	selects := make([]reflect.SelectCase, 3, 3+len(mons))
	selects[waitMonitorTimeout] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(time.After(timeout)),
	}
	selects[waitMonitorContextDone] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	}
	selects[waitMonitorNotified] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(notifier),
	}
	for _, mp := range mons {
		selects = append(
			selects,
			reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(mp.Done())},
		)
	}

	chosen, _, _ := reflect.Select(selects)
	switch chosen {
	case waitMonitorTimeout, waitMonitorContextDone, waitMonitorNotified:
		return waitMonitorResult(chosen)
	default:
		// returned value is the context err, not the waitgroup err, we didn't track
		// the wg order and return doesn't say which one, so caller has to figure
		// this out for now. the point of this method is mainly to know when to
		// check the monitors not to know a specific one ended
		return waitMonitorEnded
	}
}

type httpPusher struct {
	cancel  context.CancelFunc
	done    chan struct{}
	client  *ent.Client
	logger  *logging.Logger
	pushers map[uuid.UUID]monitoredPusher
}

var _ Service = &httpPusher{}

func (s *httpPusher) Name() string {
	return "http-pusher"
}

func (s *httpPusher) Initialize(_ context.Context, client *ent.Client) error {
	s.client = client
	s.logger = logging.GetLogger(s.Name())
	s.pushers = map[uuid.UUID]monitoredPusher{}
	return nil
}

func (s *httpPusher) Start(ctx context.Context, ready chan<- struct{}) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.cancel = cancel
	eg, egCtx := errgroup.WithContext(ctx)
	s.done = make(chan struct{})
	defer close(s.done)

	eg.Go(func() error { return s.monitorSubs(egCtx, ready) })

	err := eg.Wait()

	// wait for all the sub watchers to finish before returning
	for _, mon := range s.pushers {
		mErr := mon.Wait()
		// take the first error if we don't have one already
		if err == nil {
			err = mErr
		}
	}

	return err
}

func (s *httpPusher) monitorSubs(ctx context.Context, ready chan<- struct{}) error {
	var subAwaiter actions.AnySubModifiedNotifier
	defer func() { actions.CancelAnySubModifiedAwaiter(subAwaiter) }()
LOOP:
	for {
		// get a fresh notifier every loop, before we query, so that there isn't a
		// race from a change between query and notifier setup
		actions.CancelAnySubModifiedAwaiter(subAwaiter)
		subAwaiter = actions.AnySubModifiedAwaiter()
		if err := s.startPushersOnce(ctx); err != nil {
			s.logger.Error().Err(err).Msg("error starting pushers, will retry")
			// don't retry instantly, causes spam
			retryDelay := time.After(time.Second)
			select {
			case <-retryDelay:
				continue LOOP
			case <-ctx.Done():
				return nil
			}
		}
		if ready != nil {
			close(ready)
			ready = nil
		}
		if r := waitPusherMonitors(ctx, time.Minute, subAwaiter, s.pushers); r == waitMonitorContextDone {
			return nil
		}
	}
}

func (s *httpPusher) startPushersOnce(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	// harvest the dead
	for subID, pusherMon := range s.pushers {
		select {
		case <-pusherMon.Done():
			if err := pusherMon.Wait(); err != nil {
				s.logger.With(pusherMon.LogContexter).Error().Err(err).Msg("HTTP pusher died")
			} else {
				s.logger.With(pusherMon.LogContexter).Debug().Msg("HTTP pusher ended")
			}
			// release resources
			pusherMon.cancel()
			delete(s.pushers, subID)
		default:
		}
	}

	curSet := map[uuid.UUID]struct{}{}
	// find the active push subs and ensure we have a pusher for each
	err := s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		pushSubs, err := tx.Subscription.Query().
			Where(
				subscription.DeletedAtIsNil(),
				subscription.PushEndpointNotNil(),
				subscription.PushEndpointNEQ(""),
			).
			All(ctx)
		if err != nil {
			return err
		}

		for _, sub := range pushSubs {
			curSet[sub.ID] = struct{}{}
			if _, ok := s.pushers[sub.ID]; !ok {
				// need to start a new pusher
				p := actions.NewHttpPusher(sub.Name, sub.ID, *sub.PushEndpoint, nil, s.client)
				s.pushers[sub.ID] = monitorPusher(ctx, p)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// terminate any pushers that didn't map to an actively pushing sub, they'll
	// be harvested in the next round
	for id, p := range s.pushers {
		if _, ok := curSet[id]; !ok {
			p.cancel()
		}
	}
	return nil
}

func (s *httpPusher) Cleanup(context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.done != nil {
		<-s.done
	}
	s.client = nil
	s.logger = nil
	s.pushers = nil
	return nil
}

func init() {
	defaultServices = append(defaultServices, &httpPusher{})
}
