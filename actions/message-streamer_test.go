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
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/enttest"
	"go.6river.tech/mmmbbb/internal/testutil"
	"go.6river.tech/mmmbbb/logging"
)

type mockInbound struct {
	req *MessageStreamRequest
	err error
}
type mockConn struct {
	t              *testing.T
	closeError     error
	closed         bool
	inbound        chan mockInbound
	outboundErrors chan error
	outbound       chan *SubscriptionMessageDelivery
}

func (c *mockConn) Close() error {
	c.closed = true
	// make receive & send fail
	c.inbound <- mockInbound{err: net.ErrClosed}
	c.outboundErrors <- net.ErrClosed
	return c.closeError
}

func (c *mockConn) Receive(ctx context.Context) (*MessageStreamRequest, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case i := <-c.inbound:
		return i.req, i.err
	}
}

func (c *mockConn) Send(ctx context.Context, del *SubscriptionMessageDelivery) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-c.outboundErrors:
		return err
	case c.outbound <- del:
		return nil
	}
}

func TestMessageStreamer_Go(t *testing.T) {
	logging.ConfigureDefaultLogging()

	type params struct {
		subscriptionID   uuid.UUID
		subscriptionName string
		automaticNack    bool
	}
	type test struct {
		name       string
		before     func(t *testing.T, ctx context.Context, client *ent.Client, tt *test)
		params     params
		concurrent func(
			t *testing.T,
			ctx context.Context,
			client *ent.Client,
			tt *test,
			conn *mockConn,
			cancel context.CancelFunc,
		)
		assertion assert.ErrorAssertionFunc
		after     func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, conn *mockConn)
	}
	tests := []test{
		{
			"err no sub by name",
			nil,
			params{},
			nil,
			func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, ErrNotFound)
			},
			nil,
		},
		{
			"err no sub by id",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				tt.params.subscriptionID = uuid.New()
			},
			params{ /* filled in before */ },
			nil,
			func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, ErrNotFound)
			},
			nil,
		},
		{
			"err no sub by id+name",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				sub1 := createSubscriptionClient(
					t,
					ctx,
					client,
					topic,
					0,
					func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
						return sc.SetName(t.Name() + ":1")
					},
				)
				sub2 := createSubscriptionClient(
					t,
					ctx,
					client,
					topic,
					1,
					func(sc *ent.SubscriptionCreate) *ent.SubscriptionCreate {
						return sc.SetName(t.Name() + ":2")
					},
				)
				tt.params.subscriptionName = sub2.Name
				tt.params.subscriptionID = sub1.ID
			},
			params{ /* filled in before */ },
			nil,
			func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, ErrNotFound)
			},
			nil,
		},
		{
			"cancel with no messages",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				createSubscriptionClient(t, ctx, client, topic, 0)
			},
			params{},
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, conn *mockConn, cancel context.CancelFunc) {
				cancel()
			},
			func(tt assert.TestingT, err error, i ...interface{}) bool {
				var se *sqlite.Error
				if errors.As(err, &se) {
					// TODO: this is a temporary bodge until discussions with
					// modernc.org/sqlite are sorted out
					return assert.Equal(t, sqlite3.SQLITE_INTERRUPT, se.Code())
				}
				return assert.ErrorIs(t, err, context.Canceled)
			},
			nil,
		},
		{
			"close error with no messages",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				createSubscriptionClient(t, ctx, client, topic, 0)
			},
			params{},
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, conn *mockConn, cancel context.CancelFunc) {
				conn.inbound <- mockInbound{err: net.ErrClosed}
			},
			func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, net.ErrClosed)
			},
			nil,
		},
		{
			"simple sequence, no acks",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				createSubscriptionClient(t, ctx, client, topic, 0)
			},
			params{},
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, conn *mockConn, cancel context.CancelFunc) {
				// setup flow control
				conn.inbound <- mockInbound{req: &MessageStreamRequest{
					FlowControl: &FlowControl{
						MaxMessages: 100,
						MaxBytes:    10_000_000,
					},
				}}
				topic := client.Topic.GetX(ctx, xID(t, &ent.Topic{}, 0))
				sub := client.Subscription.GetX(ctx, xID(t, &ent.Subscription{}, 0))
				// send a short sequence of messages, make sure they are received
				for m := range 2 {
					var msgID, delID uuid.UUID
					assert.NoError(t, client.DoCtxTx(ctx, nil, func(ctx context.Context, tx *ent.Tx) error {
						msg := createMessage(t, ctx, tx, topic, m)
						del := createDelivery(t, ctx, tx, sub, msg, m)
						msgID = msg.ID
						delID = del.ID
						notifyPublish(tx, sub.ID)
						return nil
					}))
					select {
					case <-ctx.Done():
						assert.NoError(t, ctx.Err(), "context canceled waiting for delivery")
					case del := <-conn.outbound:
						assert.NotNil(t, del)
						assert.Equal(t, delID, del.ID)
						assert.Equal(t, msgID, del.MessageID)
					}
				}
				// end the session
				cancel()
			},
			func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, context.Canceled)
			},
			nil,
		},
		{
			"flow control constraint, no acks",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				createSubscriptionClient(t, ctx, client, topic, 0)
			},
			params{},
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, conn *mockConn, cancel context.CancelFunc) {
				// setup flow control
				conn.inbound <- mockInbound{req: &MessageStreamRequest{
					FlowControl: &FlowControl{
						MaxMessages: 2,
						MaxBytes:    10_000_000,
					},
				}}
				topic := client.Topic.GetX(ctx, xID(t, &ent.Topic{}, 0))
				sub := client.Subscription.GetX(ctx, xID(t, &ent.Subscription{}, 0))
				assert.NoError(t, client.DoCtxTx(ctx, nil, func(ctx context.Context, tx *ent.Tx) error {
					// send one more message than FC will allow
					for m := range 3 {
						msg := createMessage(t, ctx, tx, topic, m)
						createDelivery(t, ctx, tx, sub, msg, m)
					}
					notifyPublish(tx, sub.ID)
					return nil
				}))
				// receive up to flow control limit
				for m := range 2 {
					select {
					case <-ctx.Done():
						assert.NoError(t, ctx.Err(), "context canceled waiting for delivery")
					case del := <-conn.outbound:
						assert.NotNil(t, del)
						assert.Equal(t, xID(t, &ent.Delivery{}, m), del.ID)
						assert.Equal(t, xID(t, &ent.Message{}, m), del.MessageID)
					}
				}
				// verify we get nothing now
				select {
				case <-ctx.Done():
					assert.Fail(t, "Should not abort")
				case <-time.After(100 * time.Millisecond):
					// continue
				case del := <-conn.outbound:
					assert.Fail(t, "Should not receive delivery", del)
				}
				// raise the FC limit
				conn.inbound <- mockInbound{req: &MessageStreamRequest{
					FlowControl: &FlowControl{
						MaxMessages: 3,
						MaxBytes:    10_000_000,
					},
				}}
				del := <-conn.outbound
				assert.NotNil(t, del)
				assert.Equal(t, xID(t, &ent.Delivery{}, 2), del.ID)
				assert.Equal(t, xID(t, &ent.Message{}, 2), del.MessageID)
				// end the session
				cancel()
			},
			func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, context.Canceled)
			},
			nil,
		},
		{
			// this is how the google client works right now
			"flow control wake, external acks",
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test) {
				topic := createTopicClient(t, ctx, client, 0)
				createSubscriptionClient(t, ctx, client, topic, 0)
			},
			params{
				automaticNack: true,
			},
			func(t *testing.T, ctx context.Context, client *ent.Client, tt *test, conn *mockConn, cancel context.CancelFunc) {
				// setup flow control
				conn.inbound <- mockInbound{req: &MessageStreamRequest{
					FlowControl: &FlowControl{
						MaxMessages: 2,
						MaxBytes:    10_000_000,
					},
				}}
				topic := client.Topic.GetX(ctx, xID(t, &ent.Topic{}, 0))
				sub := client.Subscription.GetX(ctx, xID(t, &ent.Subscription{}, 0))
				assert.NoError(t, client.DoCtxTx(ctx, nil, func(ctx context.Context, tx *ent.Tx) error {
					// send many more messages than FC will allow
					for m := range 10 {
						msg := createMessage(t, ctx, tx, topic, m)
						createDelivery(t, ctx, tx, sub, msg, m)
					}
					notifyPublish(tx, sub.ID)
					return nil
				}))
				l := logging.GetLogger(t.Name())
				// receive, ack in batches sized based on the flow control
				acks := []uuid.UUID{}
				lastDelivery := time.Now()
				for m := range 10 {
					del := <-conn.outbound
					now := time.Now()
					delay := now.Sub(lastDelivery)
					assert.LessOrEqual(t, delay.Seconds(), 0.1)
					lastDelivery = now
					assert.NotNil(t, del)
					assert.Equal(t, xID(t, &ent.Delivery{}, m), del.ID)
					assert.Equal(t, xID(t, &ent.Message{}, m), del.MessageID)
					acks = append(acks, del.ID)
					l.Trace().Stringer("deliveryID", del.ID).Msg("Got a delivery")
					if len(acks) >= 2 {
						l.Trace().Int("numAcks", len(acks)).Msg("Sending acks")
						a := NewAckDeliveries(acks...)
						assert.NoError(t, client.DoCtxTx(ctx, nil, a.Execute))
						results, ok := a.Results()
						if assert.True(t, ok) {
							assert.Equal(t, len(acks), results.NumAcked)
							l.Trace().Int("numAcks", results.NumAcked).Msg("Sent acks")
						}
						acks = acks[:0]
					}
				}
				// end the session
				cancel()
			},
			func(tt assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, context.Canceled)
			},
			nil,
		},
	}
	client := enttest.ClientForTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			ctx := testutil.Context(t)
			if tt.before != nil {
				tt.before(t, ctx, client, &tt)
			}
			if tt.params.subscriptionName == "" && tt.params.subscriptionID == uuid.Nil {
				tt.params.subscriptionName = nameFor(t, 0)
			}
			a := &MessageStreamer{
				Client:           client,
				Logger:           logging.GetLogger(t.Name()),
				SubscriptionName: tt.params.subscriptionName,
				AutomaticNack:    tt.params.automaticNack,
			}
			a.Logger.Level(zerolog.Disabled)
			if tt.params.subscriptionID != uuid.Nil {
				a.SubscriptionID = &tt.params.subscriptionID
			}
			runCtx, cancel := context.WithCancel(ctx)
			eg, egCtx := errgroup.WithContext(runCtx)
			conn := &mockConn{
				t:              t,
				inbound:        make(chan mockInbound, 1),
				outboundErrors: make(chan error, 1),
				outbound:       make(chan *SubscriptionMessageDelivery, 1),
			}
			eg.Go(func() error {
				if tt.concurrent != nil {
					tt.concurrent(t, egCtx, client, &tt, conn, cancel)
				}
				return nil
			})
			err := a.Go(runCtx, conn)
			assert.NoError(t, eg.Wait())
			tt.assertion(t, err)
			if tt.after != nil {
				tt.after(t, ctx, client, &tt, conn)
			}
		})
	}
}
