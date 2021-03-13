package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
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

type httpPushStreamConn struct {
	logger           *logging.Logger
	mu               sync.Mutex
	subscriptionName string    // needed to construct the delivery messages
	subscriptionID   uuid.UUID // for logging
	endpoint         string
	client           *http.Client
	fastAckQueue     chan uuid.UUID
	slowAckQueue     chan uuid.UUID
	nackQueue        chan uuid.UUID
	maxBytes         int
	maxMessages      int
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
		logger: logging.GetLoggerWith("http-streamer", func(c zerolog.Context) zerolog.Context {
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
	// no-op, closing this is not required to interrupt any operations, just
	// cancelling the context is sufficient
	return nil
}

func (c *httpPushStreamConn) Receive(ctx context.Context) (*MessageStreamRequest, error) {
	// TODO: drain channels before returning a result, for efficiency
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
		if c.maxMessages < 1000 {
			c.maxMessages++
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
		if c.maxMessages > 1 {
			c.maxMessages--
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
			Ack: []uuid.UUID{id},
		}
		if c.maxMessages > 1 {
			if c.maxMessages > 10 {
				c.maxMessages -= 10
			} else {
				c.maxMessages--
			}
			ret.FlowControl = &FlowControl{
				MaxMessages: c.maxMessages,
				MaxBytes:    c.maxBytes,
			}
		}
		return ret, nil
	}
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
	start := time.Now()
	resp, err := c.client.Do(req)
	dur := time.Since(start)
	q := c.slowAckQueue
	if dur < time.Second {
		q = c.fastAckQueue
	}
	if err != nil {
		// nack, don't fail the sending
		// TODO: sample this log invocation
		c.logger.Warn().
			Err(err).
			Msg("Failed to contact push endpoint")
		q = c.nackQueue
	} else {
		switch resp.StatusCode {
		case http.StatusProcessing, http.StatusOK, http.StatusCreated, http.StatusAccepted:
			// all good, leave q as ack queue
		case http.StatusNoContent:
			// also all good, leave q as ack queue
			// TODO: emulate google's insistence that no-content has no content
		default:
			// TODO: sample this log invocation
			c.logger.Info().
				Int("code", resp.StatusCode).
				Msg("HTTP error (nack) from push endpoint")
			q = c.nackQueue
		}
	}
	select {
	case q <- del.ID:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
