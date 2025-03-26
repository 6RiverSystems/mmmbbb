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
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/sync/errgroup"

	"go.6river.tech/mmmbbb/actions"
	"go.6river.tech/mmmbbb/db/postgres"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/logging"
	"go.6river.tech/mmmbbb/version"
)

// subPublishedChannelName is the postgres notify/listen channel name to which
// we send the subscription uuid when a message is published to that
// subscription
const subPublishedChannelName = version.AppName + "_published_to_sub"

// topicModifiedChannelName is the postgres notify/listen channel name to which
// we send the topic uuid and name when that topic is modified
const topicModifiedChannelName = version.AppName + "_modified_topic"

// subModifiedChannelName is the postgres notify/listen channel name to which we
// send the subscription uuid and name when that subscription is modified
const subModifiedChannelName = version.AppName + "_modified_sub"

type pgNotifier struct {
	db     *sql.DB
	cancel context.CancelFunc
	done   chan struct{}
	logger *logging.Logger

	publishHook     actions.PublishHookHandle
	topicModifyHook actions.TopicModifyHookHandle
	subModifyHook   actions.SubscriptionModifyHookHandle
}

func (n *pgNotifier) Name() string {
	return "pg-notifier"
}

func (n *pgNotifier) Initialize(ctx context.Context, client *ent.Client) error {
	if client.Dialect() != dialect.Postgres {
		n.db = nil
		return nil
	}

	n.db = client.DB()
	n.logger = logging.GetLogger("services/pg-notifier")

	return nil
}

func (n *pgNotifier) Start(ctx context.Context, ready chan<- struct{}) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	n.cancel = cancel
	n.done = make(chan struct{})
	defer close(n.done)

	if n.db == nil {
		// not postgres
		close(ready)
		return nil
	}

	// TODO: detect if we are using pgBouncer running with transaction-level
	// pooling (which we generally use) and cancel this, as it relies on
	// session-level pooling. that is currently difficult without external
	// support, see: https://github.com/pgbouncer/pgbouncer/issues/249

	// batching notifies saves overhead in connection leasing at high throughput.
	// publishes want to be able to buffer a lot for throughput, topic & sub
	// modifies should be rarer and don't need so much.
	pubNotifies := make(chan uuid.UUID, 100)
	type idName struct {
		id   uuid.UUID
		name string
	}
	topicNotifies := make(chan idName, 10)
	subNotifies := make(chan idName, 10)

	n.publishHook = actions.AddPublishHook(func(subID uuid.UUID) {
		if pubNotifies != nil {
			pubNotifies <- subID
		}
	})
	// TODO: can generalize these two
	n.topicModifyHook = actions.AddTopicModifyHook(func(topicID uuid.UUID, topicName string) {
		if topicNotifies != nil {
			topicNotifies <- idName{topicID, topicName}
		}
	})
	n.subModifyHook = actions.AddSubscriptionModifyHook(func(subID uuid.UUID, subName string) {
		if subNotifies != nil {
			subNotifies <- idName{subID, subName}
		}
	})

	eg, egCtx := errgroup.WithContext(ctx)

	// connection retry loop
	eg.Go(func() error {
		for {
			if err := n.runOnce(egCtx, &ready); err != nil {
				if isContextError(err, egCtx) {
					// context was canceled
					return nil
				}
				n.logger.Warn().Err(err).Msg("pg connection failed, retrying")
				// don't retry instantly, causes spam
				retryDelay := time.After(time.Second)
				select {
				case <-retryDelay:
					continue
				case <-egCtx.Done():
					return nil
				}
			}
		}
	})

	// queue processor loop
	eg.Go(func() error {
		const bufferSize = 100
		pn := make(map[uuid.UUID]struct{}, bufferSize)
		tn := make(map[idName]struct{}, bufferSize)
		sn := make(map[idName]struct{}, bufferSize)

		for {

			// block waiting for the first one
			select {
			case p := <-pubNotifies:
				pn[p] = struct{}{}
			case t := <-topicNotifies:
				tn[t] = struct{}{}
			case s := <-subNotifies:
				sn[s] = struct{}{}
			case <-egCtx.Done():
				return nil
			}
			// then get as much as is buffered up to their cap without blocking
		DRAIN:
			for {
				select {
				case p := <-pubNotifies:
					pn[p] = struct{}{}
					if len(pn) >= bufferSize {
						break DRAIN
					}
				case t := <-topicNotifies:
					tn[t] = struct{}{}
					if len(tn) >= bufferSize {
						break DRAIN
					}
				case s := <-subNotifies:
					sn[s] = struct{}{}
					if len(sn) >= bufferSize {
						break DRAIN
					}
				case <-egCtx.Done():
					return nil
				default:
					break DRAIN
				}
			}
			// and then process them
			func() {
				conn, err := n.db.Conn(egCtx)
				if err != nil {
					// yikes, we're going to fail to send some notifies, things waiting for them may hang!
					n.logger.Error().Err(err).Msg("pg connection failed for notify send")
					return
				}
				defer conn.Close()
				for id := range pn {
					delete(pn, id)
					_, err := conn.ExecContext(
						egCtx,
						`SELECT pg_notify($1, $2)`,
						subPublishedChannelName,
						id.String(),
					)
					if err != nil {
						n.logger.Error().Err(err).Msg("Failed to send notify for sub publish")
					}
				}
				for idn := range tn {
					_, err := conn.ExecContext(
						egCtx,
						`SELECT pg_notify($1, $2 || ' ' || $3)`,
						topicModifiedChannelName,
						idn.id.String(),
						idn.name,
					)
					if err != nil {
						n.logger.Error().Err(err).Msg("Failed to send notify for topic modify")
					}
				}
				for idn := range sn {
					_, err := conn.ExecContext(
						egCtx,
						`SELECT pg_notify($1, $2 || ' ' || $3)`,
						subModifiedChannelName,
						idn.id.String(),
						idn.name,
					)
					if err != nil {
						n.logger.Error().Err(err).Msg("Failed to send notify for sub modify")
					}
				}
			}()
		}
	})

	return eg.Wait()
}

func (n *pgNotifier) runOnce(ctx context.Context, ready *chan<- struct{}) error {
	c, err := n.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	return c.Raw(func(driverConn interface{}) error {
		pgc := driverConn.(*stdlib.Conn).Conn()

		for _, c := range []string{subPublishedChannelName, topicModifiedChannelName, subModifiedChannelName} {
			_, err := pgc.Exec(ctx, fmt.Sprintf(`LISTEN %s`, postgres.QuoteIdentifier(c)))
			if err != nil {
				return err
			}
			defer func() {
				// if we are exiting due to context cancellation, we don't want this to
				// be canceled, so we use the background ctx here
				_, err := pgc.Exec(
					context.Background(),
					fmt.Sprintf(`UNLISTEN %s`, postgres.QuoteIdentifier(c)),
				)
				if err != nil {
					n.logger.Error().Err(err).Msg("Unable to UNLISTEN")
				}
			}()
		}

		// once we wire up all the PG listeners, we need to wake up all the internal
		// listeners, as we don't know what we might have missed
		actions.WakeAllInternal()

		received := make(chan *pgconn.Notification, 1)
		eg, egCtx := errgroup.WithContext(ctx)
		eg.Go(func() error {
			for {
				notification, err := pgc.WaitForNotification(ctx)
				if err != nil {
					if isContextError(err, egCtx, ctx) {
						// context ended, stop
						return nil
					} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
						// this also happens on context termination
						return nil
					}
					// pg error, log and stop
					n.logger.Error().Err(err).Msg("notifier pg connection failed")
					return err
				}
				received <- notification
			}
		})

		if *ready != nil {
			close(*ready)
			*ready = nil
		}

	LOOP:
		for {
			select {
			case <-egCtx.Done():
				// stop
				break LOOP
			case notification := <-received:
				switch notification.Channel {
				case subPublishedChannelName:
					subID, err := uuid.Parse(notification.Payload)
					if err != nil {
						n.logger.Error().Err(err).Msg("Bad UUID on sub notifier channel")
						break
					}
					go actions.WakePublishListeners(true, subID)
				case topicModifiedChannelName:
					id, name, err := parseUUIDName(notification.Payload)
					if err != nil {
						n.logger.Error().Err(err).Msg("Bad format on topic modification notifier channel")
						break
					}
					go actions.WakeTopicListeners(true, id, name)
				case subModifiedChannelName:
					id, name, err := parseUUIDName(notification.Payload)
					if err != nil {
						n.logger.Error().Err(err).Msg("Bad format on subscription modification notifier channel")
						break
					}
					go actions.WakeSubscriptionListeners(true, id, name)
					// wake publish listeners too, as this likely affects them
					go actions.WakePublishListeners(true, id)
				default:
					n.logger.Error().
						Str("channelName", notification.Channel).
						Msg("Got LISTEN notification from unexpected channel")
				}
			}
		}

		return eg.Wait()
	})
}

func parseUUIDName(s string) (uuid.UUID, string, error) {
	// expected format is "uuid name", noting that name may contain whitespace and
	// other data, so we simply split on the first space char, everything before
	// that is the uuid and everything after is the name
	space := strings.IndexRune(s, ' ')
	if space < 0 {
		return uuid.UUID{}, "", errors.New("no space found")
	}
	name := s[space+1:]
	id, err := uuid.Parse(s[:space])
	return id, name, err
}

func (n *pgNotifier) Cleanup(context.Context) error {
	if n.cancel != nil {
		n.cancel()
	}
	if n.done != nil {
		<-n.done
	}

	if n.publishHook != nil {
		actions.RemovePublishHook(n.publishHook)
		n.publishHook = nil
	}
	if n.topicModifyHook != nil {
		actions.RemoveTopicModifyHook(n.topicModifyHook)
		n.topicModifyHook = nil
	}
	if n.subModifyHook != nil {
		actions.RemoveSubscriptionModifyHook(n.subModifyHook)
		n.subModifyHook = nil
	}

	n.db = nil
	return nil
}

func init() {
	defaultServices = append(defaultServices, &pgNotifier{})
}

func isContextError(err error, ctxs ...context.Context) bool {
	for _, ctx := range ctxs {
		ce := ctx.Err()
		if ce != nil && errors.Is(err, ce) {
			return true
		}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return false
}
