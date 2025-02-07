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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"go.6river.tech/mmmbbb/actions"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/enttest"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/internal/testutil"
	"go.6river.tech/mmmbbb/logging"
)

func assertEqualJSON(t testing.TB, expected any, actual []byte) bool {
	// JSON normalization differs between PG & SQLite, we don't care about that,
	// we just want to assert on the value equality.
	var actualValue any
	if !assert.NoError(t, json.Unmarshal(actual, &actualValue)) {
		return false
	}
	return assert.EqualValues(t, expected, actualValue)
}

// TODO: a bunch of these tests should be moved into `actions`
func TestHttpPush(t *testing.T) {
	logging.ConfigureDefaultLogging()

	type test struct {
		name          string
		initialEnable bool
		sender        func(t *testing.T, client *ent.Client, endpoint string) error
		responder     pushResponder
		receiver      func(t *testing.T, msgs chan *actions.PushRequest) error
		after         func(t *testing.T, s *httpPusher)
	}
	tests := []test{
		{
			"single message",
			true,
			func(t *testing.T, client *ent.Client, endpoint string) error {
				return publishMarker(t, client, 0)
			},
			nil,
			func(t *testing.T, msgs chan *actions.PushRequest) error {
				m := <-msgs
				assert.Equal(t, safeName(t), m.Subscription)
				decoded, err := base64.StdEncoding.DecodeString(m.Message.Data)
				assert.NoError(t, err, "message data should be base64 decodable")
				assertEqualJSON(t, map[string]any{"sequence": 0.0}, decoded)
				assert.Empty(t, m.Message.Attributes)
				assert.Empty(t, m.Message.OrderingKey)
				assertNoMore(t, msgs)
				return nil
			},
			nil,
		},
		{
			"delayed disable",
			true,
			func(t *testing.T, client *ent.Client, endpoint string) error {
				// this message should be received by the push endpoint
				if err := publishMarker(t, client, 0); err != nil {
					return err
				}
				// sadly we just need to delay a bit to know the message went through
				<-time.After(25 * time.Millisecond)
				if err := client.DoCtxTx(testutil.Context(t), nil, func(ctx context.Context, tx *ent.Tx) error {
					sub, err := tx.Subscription.Query().
						Where(subscription.Name(safeName(t))).
						Only(ctx)
					if !assert.NoError(t, err) {
						return err
					}
					if err = sub.Update().
						ClearPushEndpoint().
						Exec(ctx); !assert.NoError(t, err) {
						return err
					}
					actions.NotifyModifySubscription(tx, sub.ID, sub.Name)
					return nil
				}); err != nil {
					return err
				}
				// sadly we just need to delay a bit to know the sub change took effect
				<-time.After(100 * time.Millisecond)
				// this message should not be received by the push endpoint
				if err := publishMarker(t, client, 1); err != nil {
					return err
				}
				return nil
			},
			nil,
			func(t *testing.T, msgs chan *actions.PushRequest) error {
				m := <-msgs
				assert.Equal(t, safeName(t), m.Subscription)
				decoded, err := base64.StdEncoding.DecodeString(m.Message.Data)
				assert.NoError(t, err, "message data should be base64 decodable")
				assertEqualJSON(t, map[string]any{"sequence": 0.0}, decoded)
				assert.Empty(t, m.Message.Attributes)
				assert.Empty(t, m.Message.OrderingKey)
				assertNoMore(t, msgs)
				return nil
			},
			nil,
		},
		{
			"delayed enable",
			false,
			func(t *testing.T, client *ent.Client, endpoint string) error {
				// this message should not be received by the push endpoint
				if err := publishMarker(t, client, 0); err != nil {
					return err
				}
				if err := client.DoCtxTx(testutil.Context(t), nil, func(ctx context.Context, tx *ent.Tx) error {
					sub, err := tx.Subscription.Query().
						Where(subscription.Name(safeName(t))).
						Only(ctx)
					if !assert.NoError(t, err) {
						return err
					}
					// we need to ack the message sent before the enable
					if acked, err := tx.Delivery.Update().
						Where(delivery.HasSubscriptionWith(subscription.ID(sub.ID))).
						SetCompletedAt(time.Now()).
						Save(ctx); !assert.NoError(t, err) {
						return err
					} else {
						assert.Equal(t, 1, acked)
					}
					if err = sub.Update().
						SetPushEndpoint(endpoint).
						Exec(ctx); !assert.NoError(t, err) {
						return err
					}
					actions.NotifyModifySubscription(tx, sub.ID, sub.Name)
					return nil
				}); err != nil {
					return err
				}
				// don't need to delay in this case
				// this message should be received by the push endpoint
				if err := publishMarker(t, client, 1); err != nil {
					return err
				}
				return nil
			},
			nil,
			func(t *testing.T, msgs chan *actions.PushRequest) error {
				m := <-msgs
				assert.Equal(t, safeName(t), m.Subscription)
				decoded, err := base64.StdEncoding.DecodeString(m.Message.Data)
				assert.NoError(t, err, "message data should be base64 decodable")
				assertEqualJSON(t, map[string]any{"sequence": 1.0}, decoded)
				assert.Empty(t, m.Message.Attributes)
				assert.Empty(t, m.Message.OrderingKey)
				assertNoMore(t, msgs)
				return nil
			},
			nil,
		},
		{
			"accelerating flow control",
			true,
			func(t *testing.T, client *ent.Client, endpoint string) error {
				for i := 0; i < 100; i++ {
					if err := publishMarker(t, client, i); err != nil {
						return err
					}
				}
				return nil
			},
			nil,
			func(t *testing.T, msgs chan *actions.PushRequest) error {
				got := map[int]struct{}{}
				for len(got) < 100 {
					m := <-msgs
					assert.Equal(t, safeName(t), m.Subscription)
					var mm struct {
						Sequence int `json:"sequence"`
					}
					decoded, err := base64.StdEncoding.DecodeString(m.Message.Data)
					assert.NoError(t, err, "message data should be base64 decodable")
					assert.NoError(t, json.Unmarshal(decoded, &mm))
					assert.GreaterOrEqual(t, mm.Sequence, 0)
					assert.Less(t, mm.Sequence, 100)
					assert.Empty(t, m.Message.Attributes)
					assert.Empty(t, m.Message.OrderingKey)
					assert.NotContains(t, got, mm.Sequence, "should not get duplicate deliveries")
					got[mm.Sequence] = struct{}{}
				}
				assertNoMore(t, msgs)
				return nil
			},
			func(t *testing.T, s *httpPusher) {
				assert.Len(t, s.pushers, 1)
				for _, p := range s.pushers {
					fc := p.CurrentFlowControl()
					assert.Equal(t, 101, fc.MaxMessages, "flow control should have incremented one for every message")
				}
				if incomplete, err := s.client.Delivery.Query().
					Where(
						delivery.CompletedAtIsNil(),
						delivery.HasSubscriptionWith(subscription.Name(safeName(t))),
					).
					Count(testutil.Context(t)); assert.NoError(t, err) {
					assert.Zero(t, incomplete, "all messages should have been delivered and ACKed")
				}
			},
		},
		{
			// TODO: this test flaps a bit when double-delivery happens due to timing
			// issues
			"decelerating flow control",
			true,
			func(t *testing.T, client *ent.Client, endpoint string) error {
				for i := 0; i < 4; i++ {
					if err := publishMarker(t, client, i); err != nil {
						return err
					}
				}
				return nil
			},
			func(t testing.TB, w http.ResponseWriter, r *http.Request, pr *actions.PushRequest) {
				var mm struct {
					Sequence int `json:"sequence"`
				}
				decoded, err := base64.StdEncoding.DecodeString(pr.Message.Data)
				assert.NoError(t, err, "message data should be base64 decodable")
				assert.NoError(t, json.Unmarshal(decoded, &mm))
				slow := mm.Sequence%2 == 1
				if slow {
					// TODO: this makes this test slow, but this time value is part of the
					// API spec, so we can't speed it up
					time.Sleep(1010 * time.Millisecond)
				}
				w.WriteHeader(http.StatusNoContent)
			},
			func(t *testing.T, msgs chan *actions.PushRequest) error {
				got := map[int]struct{}{}
				for len(got) < 4 {
					m := <-msgs
					assert.Equal(t, safeName(t), m.Subscription)
					var mm struct {
						Sequence int `json:"sequence"`
					}
					decoded, err := base64.StdEncoding.DecodeString(m.Message.Data)
					assert.NoError(t, err, "message data should be base64 decodable")
					assert.NoError(t, json.Unmarshal(decoded, &mm))
					assert.GreaterOrEqual(t, mm.Sequence, 0)
					assert.Less(t, mm.Sequence, 4)
					assert.Empty(t, m.Message.Attributes)
					assert.Empty(t, m.Message.OrderingKey)
					assert.NotContains(t, got, mm.Sequence, "shouldn't double deliver")
					got[mm.Sequence] = struct{}{}
				}
				assert.Len(t, got, 4, "should get everything")
				assertNoMore(t, msgs)
				return nil
			},
			func(t *testing.T, s *httpPusher) {
				assert.Len(t, s.pushers, 1)
				for _, p := range s.pushers {
					fc := p.CurrentFlowControl()
					assert.Equal(t, 1, fc.MaxMessages, "FlowControl should decelerate to 1 message at a time on NACKs")
				}
			},
		},
		{
			"nack flow control",
			true,
			func(t *testing.T, client *ent.Client, endpoint string) error {
				for i := 0; i < 11; i++ {
					if err := publishMarker(t, client, i); err != nil {
						return err
					}
				}
				return nil
			},
			func(t testing.TB, w http.ResponseWriter, r *http.Request, pr *actions.PushRequest) {
				var mm struct {
					Sequence int `json:"sequence"`
				}
				decoded, err := base64.StdEncoding.DecodeString(pr.Message.Data)
				assert.NoError(t, err, "message data should be base64 decodable")
				assert.NoError(t, json.Unmarshal(decoded, &mm))
				// push messages don't get a DeliveryAttempt, so we just nack it once
				// and be done with it
				if mm.Sequence == 10 {
					// make sure concurrent messages ack
					time.Sleep(50 * time.Millisecond)
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					w.WriteHeader(http.StatusNoContent)
				}
			},
			func(t *testing.T, msgs chan *actions.PushRequest) error {
				got := map[int]struct{}{}
				for len(got) < 11 {
					m := <-msgs
					assert.Equal(t, safeName(t), m.Subscription)
					var mm struct {
						Sequence int `json:"sequence"`
					}
					decoded, err := base64.StdEncoding.DecodeString(m.Message.Data)
					assert.NoError(t, err, "message data should be base64 decodable")
					assert.NoError(t, json.Unmarshal(decoded, &mm))
					assert.GreaterOrEqual(t, mm.Sequence, 0)
					assert.Less(t, mm.Sequence, 11)
					assert.Empty(t, m.Message.Attributes)
					assert.Empty(t, m.Message.OrderingKey)
					assert.NotContains(t, got, mm.Sequence)
					got[mm.Sequence] = struct{}{}
				}
				assert.Len(t, got, 11)
				// no call to assertNoMore here, as we aren't interested in re-delivery
				// of the last message; however we do need to delay to ensure the nack
				// completes
				time.Sleep(100 * time.Millisecond)
				return nil
			},
			func(t *testing.T, s *httpPusher) {
				assert.Len(t, s.pushers, 1)
				for _, p := range s.pushers {
					fc := p.CurrentFlowControl()
					assert.Equal(t, 1, fc.MaxMessages, "flow control should be at minimum after nacks")
				}
				// there should be one incomplete delivery on the sub, with attempts=1
				if del, err := s.client.Delivery.Query().
					Where(
						delivery.CompletedAtIsNil(),
						delivery.HasSubscriptionWith(subscription.Name(safeName(t))),
					).
					Only(testutil.Context(t)); assert.NoError(t, err) {
					assert.Equal(t, 1, del.Attempts)
					// should be in the future, too, by at least ~50% of the min backoff
					assert.Greater(
						t,
						del.AttemptAt.UnixNano(),
						time.Now().Add(50*time.Millisecond).UnixNano(),
						"next attempt should be in the future after NACKs",
					)
				}
			},
		},
		{
			"nack redlivery",
			true,
			func(t *testing.T, client *ent.Client, endpoint string) error {
				if err := publishMarker(t, client, 0); err != nil {
					return err
				}
				return nil
			},
			func(t testing.TB, w http.ResponseWriter, r *http.Request, pr *actions.PushRequest) {
				var mm struct {
					Sequence int `json:"sequence"`
				}
				decoded, err := base64.StdEncoding.DecodeString(pr.Message.Data)
				assert.NoError(t, err, "message data should be base64 decodable")
				assert.NoError(t, json.Unmarshal(decoded, &mm))
				w.WriteHeader(http.StatusInternalServerError)
			},
			func(t *testing.T, msgs chan *actions.PushRequest) error {
				for i := 0; i < 2; i++ {
					m := <-msgs
					assert.Equal(t, safeName(t), m.Subscription)
					var mm struct {
						Sequence int `json:"sequence"`
					}
					decoded, err := base64.StdEncoding.DecodeString(m.Message.Data)
					assert.NoError(t, err, "message data should be base64 decodable")
					assert.NoError(t, json.Unmarshal(decoded, &mm))
					assert.Equal(t, mm.Sequence, 0)
					assert.Empty(t, m.Message.Attributes)
					assert.Empty(t, m.Message.OrderingKey)
				}
				// no call to assertNoMore here, as we aren't interested in re-delivery
				// of the last message; however we do need to delay to ensure the nack
				// completes
				time.Sleep(100 * time.Millisecond)
				return nil
			},
			func(t *testing.T, s *httpPusher) {
				assert.Len(t, s.pushers, 1)
				for _, p := range s.pushers {
					fc := p.CurrentFlowControl()
					assert.Equal(t, 1, fc.MaxMessages, "flow control should be at minimum after nacks")
				}
				// there should be one incomplete delivery on the sub, with attempts=1
				if del, err := s.client.Delivery.Query().
					Where(
						delivery.CompletedAtIsNil(),
						delivery.HasSubscriptionWith(subscription.Name(safeName(t))),
					).
					Only(testutil.Context(t)); assert.NoError(t, err) {
					assert.Equal(t, 2, del.Attempts)
					// should be in the future, too, by at least ~50% of the min backoff
					assert.Greater(
						t,
						del.AttemptAt.UnixNano(),
						time.Now().Add(50*time.Millisecond).UnixNano(),
						"next attempt should be in the future after NACKs",
					)
				}
			},
		},
	}
	client := enttest.ClientForTest(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			ctx := testutil.Context(t)

			receiver, addr, msgs, serverWG := newPushReceiver(t, tt.responder)
			t.Cleanup(func() {
				// in case we panic/skip out of the test
				receiver.Close()
				serverWG.Wait()
			})

			require.NoError(t, client.DoCtxTx(ctx, nil, actions.NewCreateTopic(actions.CreateTopicParams{
				Name: safeName(t),
			}).Execute))
			initialEndpoint := ""
			pushEndpoint := fmt.Sprintf("http://%s/", addr.String())
			if tt.initialEnable {
				initialEndpoint = pushEndpoint
			}
			require.NoError(
				t,
				client.DoCtxTx(ctx, nil, actions.NewCreateSubscription(actions.CreateSubscriptionParams{
					TopicName:    safeName(t),
					Name:         safeName(t),
					PushEndpoint: initialEndpoint,
					TTL:          time.Minute,
					MessageTTL:   time.Minute,
					MinBackoff:   200 * time.Millisecond,
					MaxBackoff:   2000 * time.Millisecond,
				}).Execute),
			)

			s := &httpPusher{}
			require.NoError(t, s.Initialize(ctx, client))
			defer func() { assert.NoError(t, s.Cleanup(ctx)) }()

			ctx2, cancel := context.WithCancel(ctx)
			defer cancel()
			eg, egCtx := errgroup.WithContext(ctx2)
			ready := make(chan struct{})
			eg.Go(func() error { return s.Start(egCtx, ready) })
			eg.Go(func() error { return tt.sender(t, client, pushEndpoint) })

			// wait for it to start the pusher
			<-ready

			eg.Go(func() error {
				defer cancel()
				return tt.receiver(t, msgs)
			})

			// OK to either return cancellation error or nil
			assert.Contains(t, []error{nil, context.Canceled}, eg.Wait())
			receiver.Close()
			serverWG.Wait()
			close(msgs)

			if tt.after != nil {
				tt.after(t, s)
			}
		})
	}
}

type pushResponder func(testing.TB, http.ResponseWriter, *http.Request, *actions.PushRequest)

func newPushReceiver(
	t testing.TB,
	responder pushResponder,
) (*http.Server, *net.TCPAddr, chan *actions.PushRequest, *sync.WaitGroup) {
	ctx := testutil.Context(t)

	received := make(chan *actions.PushRequest, 1)
	wg := &sync.WaitGroup{}

	server := &http.Server{
		BaseContext: func(net.Listener) context.Context { return ctx },
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wg.Add(1)
			assert.Equal(t, http.MethodPost, r.Method)
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			var m actions.PushRequest
			err = json.Unmarshal(body, &m)
			assert.NoError(t, err)
			if responder != nil {
				responder(t, w, r, &m)
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
			select {
			case received <- &m:
				// OK
			case <-time.After(time.Second):
				assert.Fail(t, "unable to deliver received message to receiver channel")
			}
			wg.Done()
		}),
	}

	lc := net.ListenConfig{}
	l, err := lc.Listen(ctx, "tcp", "localhost:0")
	addr := l.Addr().(*net.TCPAddr)
	// t.Logf("Push receiver listening on %v", addr)
	require.NoError(t, err)
	go func() {
		err := server.Serve(l)
		if err != nil {
			assert.ErrorIs(t, err, http.ErrServerClosed)
		}
	}()
	return server, addr, received, wg
}

func assertNoMore(t testing.TB, msgs chan *actions.PushRequest) {
	assert.NotNil(t, msgs)
	for {
		select {
		case msg, ok := <-msgs:
			if ok {
				assert.Failf(
					t,
					"receive on expected-open-empty channel should not receive a value",
					"got unexpected message: %#v",
					msg,
				)
			} else {
				assert.Fail(t, "receive on expected-open-empty channel should not confirm closed")
				return
			}
		case <-time.After(100 * time.Millisecond):
			// PASS
			return
		}
	}
}

func publishMarker(t testing.TB, client *ent.Client, sequence int) error {
	err := client.DoCtxTx(testutil.Context(t), nil, actions.NewPublishMessage(actions.PublishMessageParams{
		TopicName: safeName(t),
		Payload:   json.RawMessage(fmt.Sprintf(`{"sequence": %d}`, sequence)),
	}).Execute)
	assert.NoError(t, err)
	return err
}
