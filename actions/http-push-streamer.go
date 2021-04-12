package actions

import (
	"bytes"
	"context"
	"encoding/json"
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
	// TODO: include topic info
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
	isChanged = c.failing != newValue
	c.failing = newValue
	c.mu.Unlock()
	return
}

func (c *httpPushStreamConn) Send(ctx context.Context, del *SubscriptionMessageDelivery) error {
	bodyObject := pubsub.PushRequest{
		Message: pubsub.PushMessage{
			Attributes: del.Attributes,
			Data:       string(del.Payload),
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
			if c.nowFailing(true) {
				c.logger.Warn().
					Err(err).
					Msg("Failed to contact push endpoint")
			}
			httpPushFailures.WithLabelValues("error").Inc()
			q = c.nackQueue
		} else {
			var metric *prometheus.CounterVec
			switch resp.StatusCode {
			case http.StatusProcessing, http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent:
				// all good, leave q as ack queue
				// TODO: emulate google's insistence that no-content has no content
				c.nowFailing(false)
				metric = httpPushSuccesses
			default:
				if c.nowFailing(true) {
					c.logger.Info().
						Int("code", resp.StatusCode).
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
