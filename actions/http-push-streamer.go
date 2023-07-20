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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"go.6river.tech/gosix/logging"
	"go.6river.tech/gosix/pubsub"
	"go.6river.tech/mmmbbb/ent"
)

type HttpPushStreamer struct {
	conn *httpPushStreamConn
	ms   *MessageStreamer
}

func NewHttpPusher(
	subscriptionName string,
	subscriptionID uuid.UUID,
	endpoint string,
	httpClient *http.Client,
	entClient *ent.Client,
) *HttpPushStreamer {
	conn := newHttpPushConn(subscriptionName, subscriptionID, endpoint, httpClient)
	streamer := &MessageStreamer{
		Client:           entClient,
		Logger:           conn.logger,
		SubscriptionID:   &conn.subscriptionID,
		SubscriptionName: conn.subscriptionName,
		// we want automatic renew for http push delivery
		AutomaticNack: false,
	}
	return &HttpPushStreamer{conn, streamer}
}

func (p *HttpPushStreamer) Go(ctx context.Context) error {
	return p.ms.Go(ctx, p.conn)
}

// TODO: this is only exposed for acceptance testing, which is bad, refactor
// tests to not need this
func (p *HttpPushStreamer) CurrentFlowControl() FlowControl {
	return FlowControl{
		MaxMessages: p.conn.maxMessages,
		MaxBytes:    p.conn.maxBytes,
	}
}

func (p *HttpPushStreamer) LogContexter(c zerolog.Context) zerolog.Context {
	return c.
		Str("subscriptionName", p.conn.subscriptionName).
		Stringer("subscriptionID", p.conn.subscriptionID)
	// TODO: include topic info in log context
}

type httpPushStreamConn struct {
	logger           *logging.Logger
	mu               sync.Mutex
	subscriptionName string    // needed to construct the delivery messages
	subscriptionID   uuid.UUID // for logging
	endpoint         string
	client           *http.Client
	wg               sync.WaitGroup
	fastAckQueue     chan uuid.UUID
	slowAckQueue     chan uuid.UUID
	nackQueue        chan uuid.UUID
	maxBytes         int
	maxMessages      int
	failing          bool
	lastFail         time.Time
}

var _ StreamConnection = &httpPushStreamConn{}

func newHttpPushConn(
	subscriptionName string,
	subscriptionID uuid.UUID,
	endpoint string,
	client *http.Client,
) *httpPushStreamConn {
	if client == nil {
		client = http.DefaultClient
	}
	if endpoint == "" {
		panic("empty endpoint")
	}
	return &httpPushStreamConn{
		logger: logging.GetLoggerWith("actions/http-streamer", func(c zerolog.Context) zerolog.Context {
			return c.
				Str("subscriptionName", subscriptionName).
				Stringer("subscriptionID", subscriptionID).
				Str("endpoint", endpoint)
		}),
		subscriptionName: subscriptionName,
		subscriptionID:   subscriptionID,
		endpoint:         endpoint,
		client:           client,
		fastAckQueue:     make(chan uuid.UUID, 10),
		slowAckQueue:     make(chan uuid.UUID, 10),
		nackQueue:        make(chan uuid.UUID, 10),
		maxBytes:         10_000_000,
		maxMessages:      1,
	}
}

func (c *httpPushStreamConn) Close() error {
	// Wait for any in-progress sends to complete. Having canceled the context
	// should cause them to do so quickly.
	c.wg.Wait()

	return nil
}

func drainIds(dest *[]uuid.UUID, src <-chan uuid.UUID) {
	for {
		select {
		case id := <-src:
			*dest = append(*dest, id)
		default:
			return
		}
	}
}

func (c *httpPushStreamConn) Receive(ctx context.Context) (*MessageStreamRequest, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case id := <-c.fastAckQueue:
		// fast ack, increase message limit
		c.mu.Lock()
		defer c.mu.Unlock()
		ret := &MessageStreamRequest{
			Ack: []uuid.UUID{id},
		}
		drainIds(&ret.Ack, c.fastAckQueue)
		if c.maxMessages < 1000 {
			mm := c.maxMessages + len(ret.Ack)
			if mm > 1000 {
				mm = 1000
			}
			c.maxMessages = mm
			ret.FlowControl = &FlowControl{
				MaxMessages: c.maxMessages,
				MaxBytes:    c.maxBytes,
			}
		}
		return ret, nil
	case id := <-c.slowAckQueue:
		// fast ack, decrease message limit
		c.mu.Lock()
		defer c.mu.Unlock()
		ret := &MessageStreamRequest{
			Ack: []uuid.UUID{id},
		}
		drainIds(&ret.Ack, c.slowAckQueue)
		if c.maxMessages > 1 {
			mm := c.maxMessages - len(ret.Ack)
			if mm < 1 {
				mm = 1
			}
			c.maxMessages = mm
			ret.FlowControl = &FlowControl{
				MaxMessages: c.maxMessages,
				MaxBytes:    c.maxBytes,
			}
		}
		return ret, nil
	case id := <-c.nackQueue:
		// nack, major decrease in message limit
		c.mu.Lock()
		defer c.mu.Unlock()
		ret := &MessageStreamRequest{
			Nack: []uuid.UUID{id},
		}
		drainIds(&ret.Nack, c.nackQueue)
		if c.maxMessages > 1 {
			mm := c.maxMessages - 10*len(ret.Nack)
			if mm < 1 {
				mm = 1
			}
			c.maxMessages = mm
			ret.FlowControl = &FlowControl{
				MaxMessages: c.maxMessages,
				MaxBytes:    c.maxBytes,
			}
		}
		return ret, nil
	}
}

func (c *httpPushStreamConn) nowFailing(newValue bool) (isChanged bool) {
	c.mu.Lock()
	isChanged = c.failing != newValue || time.Since(c.lastFail) > time.Minute
	c.failing = newValue
	if isChanged && newValue {
		c.lastFail = time.Now()
	}
	c.mu.Unlock()
	return
}

func (c *httpPushStreamConn) Send(ctx context.Context, del *SubscriptionMessageDelivery) error {
	// payload must be base64 encoded since the base API supports binary payloads
	payload64 := base64.StdEncoding.EncodeToString(del.Payload)
	bodyObject := pubsub.PushRequest{
		Message: pubsub.PushMessage{
			Attributes: del.Attributes,
			Data:       payload64,
			MessageId:  del.MessageID.String(),
			// OrderingKey set below
			// TODO: is this the right format?
			PublishTime: del.PublishedAt.String(),
		},
		Subscription: c.subscriptionName,
	}
	if del.OrderKey != nil {
		bodyObject.Message.OrderingKey = *del.OrderKey
	}
	body, err := json.Marshal(&bodyObject)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/json")

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		start := time.Now()
		resp, err := c.client.Do(req)
		dur := time.Since(start)
		q := c.slowAckQueue
		if dur < time.Second {
			q = c.fastAckQueue
		}
		if err != nil {
			// nack, don't fail the sending
			var evt *zerolog.Event
			if c.nowFailing(true) {
				evt = c.logger.Warn()
			} else {
				evt = c.logger.Trace()
			}
			evt.Err(err).Msg("Failed to contact push endpoint")

			httpPushFailures.WithLabelValues("error").Inc()
			q = c.nackQueue
		} else {
			var metric *prometheus.CounterVec
			defer resp.Body.Close()
			respBody, respBodyErr := io.ReadAll(resp.Body)
			// TODO: if we fail to read the response, should we treat it as a NACK
			// even if the status code was success?
			switch resp.StatusCode {
			case http.StatusProcessing, http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent:
				// all good, leave q as ack queue
				// TODO: emulate google's insistence that no-content has no content
				c.nowFailing(false)
				metric = httpPushSuccesses
				var evt *zerolog.Event
				var msg string
				if respBodyErr != nil {
					evt = c.logger.Error().
						Err(respBodyErr)
					msg = ("failed to read push subscription's response on ACK")
				} else {
					evt = c.logger.Trace().
						Str("response", string(respBody))
					msg = ("successful push")
				}
				evt.
					Int("code", resp.StatusCode).
					Str("pushEndpoint", c.endpoint).
					Str("subscriptionName", c.subscriptionName).
					Msg(msg)
			default:
				var evt *zerolog.Event
				failingChanged := c.nowFailing(true)
				if respBodyErr != nil {
					evt = c.logger.Error()
				} else if failingChanged {
					evt = c.logger.Warn()
				} else {
					evt = c.logger.Trace()
				}
				evt = evt.
					Int("code", resp.StatusCode).
					Str("pushEndpoint", c.endpoint).
					Str("subscriptionName", c.subscriptionName)
				if respBodyErr != nil {
					evt.
						Err(respBodyErr).
						Msg("failed to read response during HTTP error (nack) from push endpoint")
				} else {
					evt.
						Str("response", string(respBody)).
						Msg("HTTP error (nack) from push endpoint")
				}
				metric = httpPushFailures
				q = c.nackQueue
			}
			metric.WithLabelValues(strconv.Itoa(resp.StatusCode)).Inc()
		}
		select {
		case q <- del.ID:
		case <-ctx.Done():
		}
	}()

	return nil
}
