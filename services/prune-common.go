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

type PruneCommonSettings struct {
	actions.PruneCommonParams
	Interval time.Duration
	Backoff  time.Duration
}

var pcDefaults = PruneCommonSettings{
	Interval: time.Minute,
	Backoff:  100 * time.Millisecond,
	// TODO: should action param defaults be here or there?
	PruneCommonParams: actions.PruneCommonParams{
		MinAge:    1 * time.Hour,
		MaxDelete: 100,
	},
}

func (s *PruneCommonSettings) ApplyDefaults() {
	if s.MinAge == 0 {
		s.MinAge = pcDefaults.MinAge
	}
	if s.MaxDelete == 0 {
		s.MaxDelete = pcDefaults.MaxDelete
	}
	if s.Interval == 0 {
		s.Interval = pcDefaults.Interval
	}
	if s.Backoff == 0 {
		s.Backoff = pcDefaults.Backoff
	}
}

type pruneAction interface {
	actions.Action
	NumDeleted() int
}

type pruneService struct {
	name          string
	actionbuilder func(actions.PruneCommonParams) pruneAction
	settings      PruneCommonSettings
	logger        *logging.Logger
	action        pruneAction
	client        *ent.Client
}

func pruneServiceFor(
	name string,
	actionBuilder func(actions.PruneCommonParams) pruneAction,
) *pruneService {
	return &pruneService{
		name:          name,
		actionbuilder: actionBuilder,
	}
}

func (s *pruneService) Name() string {
	return s.name
}

func (s *pruneService) Initialize(ctx context.Context, _ *registry.Registry, client *ent.Client) error {
	// TODO: need a consistent way to inject configuration: could get config from
	// the context, but that gets inefficient fast
	s.settings.ApplyDefaults()

	s.logger = logging.GetLogger("services/" + s.name)
	s.client = client
	s.action = s.actionbuilder(s.settings.PruneCommonParams)

	s.logger.Trace().Msg("Service initialized")

	return nil
}

func (s *pruneService) Start(ctx context.Context, ready chan<- struct{}) error {
	// do the initial run soon after startup
	ticker := time.NewTicker(s.settings.Backoff)
	s.logger.Trace().Msg("Service started")
	close(ready)

LOOP:
	for {
		select {
		case <-ticker.C:
			n, err := s.runOnce(ctx)
			if s.action.HasResults() {
				evt := s.logger.Trace()
				if s.action.NumDeleted() > 0 {
					evt = s.logger.Info()
				}
				evt.Int("numDeleted", s.action.NumDeleted()).Msg("Action results")
			}
			if err != nil {
				// log errors and go back to max interval, but don't give up
				s.logger.Error().Err(err).Msgf("Error pruning %s", s.name)
				// apply a bit of fuzz to the interval here so that colliding services
				// separate out
				ticker.Reset(s.settings.Interval + time.Duration(rand.Intn(10000))*time.Millisecond)
			} else if n <= 0 {
				// we have caught up, restore normal timing
				ticker.Reset(s.settings.Interval)
			} else if n >= s.settings.MaxDelete {
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

func (s *pruneService) runOnce(
	ctx context.Context,
) (int, error) {
	tx, err := s.client.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if tx != nil {
			rbErr := tx.Rollback()
			if rbErr != nil {
				// this is nearly fatal, except that the most likely cause is we lost
				// connection with the db, in which case the txn was implicitly rolled
				// back
				s.logger.Error().Err(rbErr).Msg("Failed to rollback transaction?!")
			}
		}
	}()
	err = s.action.Execute(ctx, tx)
	if err == nil {
		// no error, commit
		err = tx.Commit()
		tx = nil
	}
	if s.action.HasResults() {
		return s.action.NumDeleted(), err
	}
	return 0, err
}

func (s *pruneService) Cleanup(context.Context, *registry.Registry) error {
	logger := s.logger
	// these aren't really necessary
	s.action = nil
	s.client = nil
	s.logger = nil
	if logger != nil {
		logger.Trace().Msg("Service cleaned up")
	}
	return nil
}

func init() {
	defaultServices = append(defaultServices,
		pruneServiceFor(
			"prune-completed-deliveries",
			func(params actions.PruneCommonParams) pruneAction {
				return actions.NewPruneCompletedDeliveries(params)
			},
		),
		pruneServiceFor(
			"prune-expired-deliveries",
			func(params actions.PruneCommonParams) pruneAction {
				return actions.NewPruneExpiredDeliveries(params)
			},
		),
		pruneServiceFor(
			"prune-completed-messages",
			func(params actions.PruneCommonParams) pruneAction {
				return actions.NewPruneCompletedMessages(params)
			},
		),
		pruneServiceFor(
			"prune-deleted-subscription-deliveries",
			func(params actions.PruneCommonParams) pruneAction {
				return actions.NewPruneDeletedSubscriptionDeliveries(params)
			},
		),
		pruneServiceFor(
			"prune-deleted-subscriptions",
			func(params actions.PruneCommonParams) pruneAction {
				return actions.NewPruneDeletedSubscriptions(params)
			},
		),
		pruneServiceFor(
			"prune-deleted-topics",
			func(params actions.PruneCommonParams) pruneAction {
				return actions.NewPruneDeletedTopics(params)
			},
		),
		pruneServiceFor(
			"delete-expired-subscriptions",
			func(params actions.PruneCommonParams) pruneAction {
				return actions.NewDeleteExpiredSubscriptions(params)
			},
		),
	)
}
