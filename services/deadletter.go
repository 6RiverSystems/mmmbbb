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
	"math/rand"
	"time"

	"go.6river.tech/gosix/logging"
	"go.6river.tech/gosix/registry"
	"go.6river.tech/mmmbbb/actions"
	"go.6river.tech/mmmbbb/ent"
)

// this is _almost_ a pruning service, except it doesn't have an age parameter

type DeadLetterSettings struct {
	actions.DeadLetterDeliveriesParams
	Interval time.Duration
	Fuzz     time.Duration
	Backoff  time.Duration
}

var dlDefaults = DeadLetterSettings{
	Interval: time.Minute,
	Fuzz:     10 * time.Second,
	Backoff:  100 * time.Millisecond,
	// TODO: should action param defaults be here or there?
	DeadLetterDeliveriesParams: actions.DeadLetterDeliveriesParams{
		MaxDeliveries: 100,
	},
}

func (s *DeadLetterSettings) ApplyDefaults() {
	if s.MaxDeliveries == 0 {
		s.MaxDeliveries = dlDefaults.MaxDeliveries
	}
	if s.Interval == 0 {
		s.Interval = dlDefaults.Interval
	}
	if s.Fuzz == 0 {
		s.Fuzz = dlDefaults.Fuzz
	}
	if s.Backoff == 0 {
		s.Backoff = dlDefaults.Backoff
	}
}

type deadLetter struct {
	// config

	settings DeadLetterSettings

	// state

	logger *logging.Logger
	client *ent.Client
}

func (s *deadLetter) Name() string {
	return "dead-letter"
}

func (s *deadLetter) Initialize(_ context.Context, _ *registry.Registry, client *ent.Client) error {
	s.settings.ApplyDefaults()

	s.logger = logging.GetLogger("services/" + s.Name())
	s.client = client

	s.logger.Trace().Msg("Service initialized")

	return nil
}

func (s *deadLetter) Start(ctx context.Context, ready chan<- struct{}) error {
	// do the initial run soon after startup
	ticker := time.NewTicker(s.settings.Backoff)
	s.logger.Trace().Msg("Service started")
	close(ready)

LOOP:
	for {
		select {
		case <-ticker.C:
			action := actions.NewDeadLetterDeliveries(s.settings.DeadLetterDeliveriesParams)
			err := s.client.DoCtxTx(ctx, nil, action.Execute)
			var n int
			if action.HasResults() {
				evt := s.logger.Trace()
				n = action.NumDeadLettered()
				if n > 0 {
					evt = s.logger.Info()
				}
				evt.Int("numDeleted", n).Msg("Action results")
			}
			if err != nil {
				// log errors and go back to max interval, but don't give up
				s.logger.Error().Err(err).Msgf("Error dead-lettering")
				// apply a bit of fuzz to the interval here so that colliding services
				// separate out
				ticker.Reset(s.settings.Interval + time.Duration(rand.Int63n(int64(s.settings.Fuzz))))
			} else if n <= 0 {
				// we have caught up, restore normal timing, with some fuzz
				ticker.Reset(s.settings.Interval + time.Duration(rand.Int63n(int64(s.settings.Fuzz))))
			} else if n >= s.settings.MaxDeliveries {
				// we are "behind", run again sooner than normal
				ticker.Reset(s.settings.Backoff)
			}
		case <-ctx.Done():
			break LOOP
		}
	}

	s.logger.Trace().Msg("Service stopped")
	return nil
}

func (s *deadLetter) Cleanup(context.Context, *registry.Registry) error {
	logger := s.logger
	// these aren't really necessary
	s.client = nil
	s.logger = nil
	if logger != nil {
		logger.Trace().Msg("Service cleaned up")
	}
	return nil
}

func init() {
	defaultServices = append(defaultServices, &deadLetter{})
}
