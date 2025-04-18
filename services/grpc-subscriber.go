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
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.6river.tech/mmmbbb/actions"
	"go.6river.tech/mmmbbb/db/postgres"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/predicate"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/ent/topic"
	"go.6river.tech/mmmbbb/filter"
	"go.6river.tech/mmmbbb/grpc"
	"go.6river.tech/mmmbbb/grpc/pubsubpb"
	"go.6river.tech/mmmbbb/internal/sqltypes"
	"go.6river.tech/mmmbbb/logging"
	"go.6river.tech/mmmbbb/parse"
)

type subscriberServer struct {
	pubsubpb.UnimplementedSubscriberServer

	client *ent.Client
}

func (s *subscriberServer) CreateSubscription(
	ctx context.Context,
	req *pubsubpb.Subscription,
) (*pubsubpb.Subscription, error) {
	if !isValidSubscriptionName(req.Name) {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Unsupported project / subscription path %s",
			req.Name,
		)
	}

	if req.Detached {
		return nil, status.Error(codes.InvalidArgument, "Cannot create detached subscription")
	}

	if req.PushConfig != nil {
		if len(req.PushConfig.Attributes) != 0 {
			return nil, status.Error(
				codes.Unimplemented,
				"Advanced features not supported (PushConfig.Attributes)",
			)
		}
		if req.PushConfig.AuthenticationMethod != nil {
			return nil, status.Error(
				codes.Unimplemented,
				"Advanced features not supported (PushConfig.AuthenticationMethod)",
			)
		}
		if req.PushConfig.Wrapper != nil {
			if _, ok := req.PushConfig.Wrapper.(*pubsubpb.PushConfig_PubsubWrapper_); !ok {
				return nil, status.Error(
					codes.Unimplemented,
					"Advanced features not supported (PushConfig.Wrapper unwrapped)",
				)
			}
		}
	}

	params := actions.CreateSubscriptionParams{
		Name:            req.Name,
		TopicName:       req.Topic,
		TTL:             req.ExpirationPolicy.GetTtl().AsDuration(),
		MessageTTL:      req.MessageRetentionDuration.AsDuration(),
		OrderedDelivery: req.EnableMessageOrdering,
		Labels:          req.Labels,
		Filter:          req.Filter,
	}
	if params.TTL == 0 {
		params.TTL = defaultSubscriptionTTL
	}
	if params.MessageTTL == 0 {
		params.MessageTTL = defaultSubscriptionMessageTTL
	}
	if req.RetryPolicy != nil {
		params.MinBackoff = req.RetryPolicy.MinimumBackoff.AsDuration()
		params.MaxBackoff = req.RetryPolicy.MaximumBackoff.AsDuration()
	}
	if req.DeadLetterPolicy != nil {
		params.MaxDeliveryAttempts = req.DeadLetterPolicy.MaxDeliveryAttempts
		if params.MaxDeliveryAttempts == 0 {
			params.MaxDeliveryAttempts = defaultDeadLetterMaxAttempts
		}
		params.DeadLetterTopic = req.DeadLetterPolicy.DeadLetterTopic
	}
	if req.PushConfig != nil {
		params.PushEndpoint = req.PushConfig.PushEndpoint
	}
	action := actions.NewCreateSubscription(params)
	err := s.client.DoCtxTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable}, action.Execute)
	if err != nil {
		if isNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "Topic not found: %s", req.Topic)
		}
		if errors.Is(err, actions.ErrExists) {
			return nil, status.Error(codes.AlreadyExists, "Subscription already exists")
		}
		if _, ok := postgres.IsPostgreSQLErrorCode(err, postgres.SerializationFailure); ok {
			return nil, status.Error(codes.Aborted, err.Error())
		}
		return nil, grpc.AsStatusError(err)
	}
	results, _ := action.Results()
	return entSubscriptionToGrpc(results.Sub, params.TopicName, params.DeadLetterTopic), nil
}

func (s *subscriberServer) GetSubscription(
	ctx context.Context,
	req *pubsubpb.GetSubscriptionRequest,
) (*pubsubpb.Subscription, error) {
	if !isValidSubscriptionName(req.Subscription) {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Unsupported project / subscription path %s",
			req.Subscription,
		)
	}

	var resp *pubsubpb.Subscription
	err := s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		s, err := tx.Subscription.Query().
			Where(
				subscription.Name(req.Subscription),
				subscription.DeletedAtIsNil(),
			).
			WithTopic().
			WithDeadLetterTopic().
			Only(ctx)
		if err != nil {
			return err
		}
		resp = entSubscriptionToGrpc(s, "", "")
		return nil
	})
	if err != nil {
		if isNotFound(err) {
			return nil, status.Errorf(
				codes.NotFound,
				"Subscription not found: %s",
				req.Subscription,
			)
		}
		return nil, grpc.AsStatusError(err)
	}
	return resp, nil
}

func (s *subscriberServer) UpdateSubscription(
	ctx context.Context,
	req *pubsubpb.UpdateSubscriptionRequest,
) (*pubsubpb.Subscription, error) {
	if !isValidSubscriptionName(req.Subscription.Name) {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Unsupported project / subscription path %s",
			req.Subscription.Name,
		)
	}

	var resp *pubsubpb.Subscription
	err := s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		sub, err := tx.Subscription.Query().
			Where(
				subscription.Name(req.Subscription.Name),
				subscription.DeletedAtIsNil(),
			).
			WithTopic().
			WithDeadLetterTopic().
			Only(ctx)
		if err != nil {
			if isNotFound(err) {
				return status.Errorf(codes.NotFound, "Subscription not found: %s", req.Subscription)
			}
			return grpc.AsStatusError(err)
		}
		// getting the saved sub is going to lose cached edges, so cache values from
		// them here
		topicName := sub.Edges.Topic.Name
		deadLetterTopicName := ""
		if sub.Edges.DeadLetterTopic != nil {
			deadLetterTopicName = sub.Edges.DeadLetterTopic.Name
		}

		subUpdate := tx.Subscription.UpdateOne(sub)
		for _, p := range req.UpdateMask.GetPaths() {
			// NOTE: `p` is the protobuf property path, not the Go one the accessible
			// version of the generator package that knows how to convert from the
			// protobuf field to the Go field name generates deprecation warnings, so,
			// we just use the protobuf name
			switch p {
			case "name":
				return status.Error(codes.InvalidArgument, "Subscriptions cannot be renamed")
			case "topic":
				return status.Error(
					codes.InvalidArgument,
					"Subscriptions cannot be moved between topics",
				)
			case "labels":
				subUpdate.SetLabels(req.Subscription.Labels)
			case "expiration_policy":
				ttl := req.Subscription.GetExpirationPolicy().GetTtl().AsDuration()
				if ttl == 0 {
					ttl = defaultSubscriptionTTL
				}
				subUpdate.SetTTL(sqltypes.Interval(ttl))
				subUpdate.SetExpiresAt(time.Now().Add(ttl))
			case "message_retention_duration":
				messageTTL := req.Subscription.MessageRetentionDuration.AsDuration()
				if messageTTL == 0 {
					messageTTL = defaultSubscriptionMessageTTL
				}
				subUpdate.SetMessageTTL(sqltypes.Interval(messageTTL))
			case "enable_message_ordering":
				// NOTE: Google does not support changing this on the fly, even though
				// we (sort of) do
				subUpdate.SetOrderedDelivery(req.Subscription.EnableMessageOrdering)
			case "retry_policy":
				rp := req.Subscription.GetRetryPolicy()
				min := rp.GetMinimumBackoff()
				max := rp.GetMaximumBackoff()
				if min == nil {
					subUpdate.ClearMinBackoff()
				} else {
					subUpdate.SetMinBackoff(sqltypes.IntervalPtr(min.AsDuration()))
				}
				if max == nil {
					subUpdate.ClearMaxBackoff()
				} else {
					subUpdate.SetMaxBackoff(sqltypes.IntervalPtr(max.AsDuration()))
				}
			case "push_config":
				if err := validatePushConfig(req.Subscription.PushConfig); err != nil {
					return err
				}
				applyPushConfig(subUpdate, req.Subscription.PushConfig)
			case "filter":
				// NOTE: Google doesn't permit updating sub filters on the fly. This
				// implementation will only apply the new filter to future messages, any
				// past deliveries will not be updated to include/exclude based on the
				// new filter.
				if req.Subscription.Filter == "" {
					// clear the filter
					subUpdate.ClearMessageFilter()
				} else {
					// validate the filter
					if _, err := filter.Parser.ParseString(sub.Name, req.Subscription.Filter); err != nil {
						return status.Errorf(codes.InvalidArgument, "Invalid filter: %v", err)
					}
					subUpdate.SetMessageFilter(req.Subscription.Filter)
				}
			case "dead_letter_policy":
				dlp := req.Subscription.GetDeadLetterPolicy()
				if deadLetterTopicName = dlp.GetDeadLetterTopic(); deadLetterTopicName == "" {
					subUpdate.ClearDeadLetterTopicID().ClearMaxDeliveryAttempts()
				} else {
					dlt, err := tx.Topic.Query().
						Where(topic.Name(deadLetterTopicName),
							topic.DeletedAtIsNil()).
						Only(ctx)
					if err != nil {
						if isNotFound(err) {
							return status.Errorf(codes.NotFound, "Dead letter topic not found: %s", deadLetterTopicName)
						}
						return grpc.AsStatusError(err)
					}
					subUpdate.SetDeadLetterTopic(dlt)
					deadLetterTopicName = dlt.Name
					if dlp.MaxDeliveryAttempts != 0 {
						subUpdate.SetMaxDeliveryAttempts(dlp.MaxDeliveryAttempts)
					} else {
						subUpdate.SetMaxDeliveryAttempts(defaultDeadLetterMaxAttempts)
					}
				}
			case "ack_deadline_seconds", "retain_acked_messages", "detached":
				// these are valid paths, we just don't support changing them
				return status.Errorf(
					codes.InvalidArgument,
					"Modifying Subscription.%s is not supported",
					p,
				)
			default:
				return status.Errorf(
					codes.InvalidArgument,
					"Modifying Subscription.%s is not a recognized subscription property",
					p,
				)
			}
		}

		// don't bother issuing the save if it's a no-op
		if len(subUpdate.Mutation().Fields()) == 0 &&
			len(subUpdate.Mutation().ClearedFields()) == 0 &&
			len(subUpdate.Mutation().AddedFields()) == 0 {
			return nil
		}

		if sub, err = subUpdate.Save(ctx); err != nil {
			return status.Error(codes.Unknown, err.Error())
		}

		actions.NotifyModifySubscription(tx, sub.ID, sub.Name)

		resp = entSubscriptionToGrpc(sub, topicName, deadLetterTopicName)

		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *subscriberServer) ListSubscriptions(
	ctx context.Context,
	req *pubsubpb.ListSubscriptionsRequest,
) (*pubsubpb.ListSubscriptionsResponse, error) {
	var pageSize int32 = 100
	if req.PageSize > 0 && req.PageSize < pageSize {
		pageSize = req.PageSize
	}

	var resp *pubsubpb.ListSubscriptionsResponse
	err := s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		predicates := []predicate.Subscription{
			subscription.NameHasPrefix(projectSubscriptionPrefix(req.Project)),
			subscription.DeletedAtIsNil(),
		}
		if req.PageToken != "" {
			pageID, err := uuid.Parse(req.PageToken)
			if err != nil {
				return status.Error(codes.InvalidArgument, err.Error())
			}
			predicates = append(predicates, subscription.IDGT(pageID))
		}

		subs, err := tx.Subscription.Query().
			Where(predicates...).
			WithTopic().
			WithDeadLetterTopic().
			Order(ent.Asc(subscription.FieldID)).
			Limit(int(pageSize)).
			All(ctx)
		if err != nil {
			return grpc.AsStatusError(err)
		}
		grpcSubscriptions := make([]*pubsubpb.Subscription, len(subs))
		for i, sub := range subs {
			grpcSubscriptions[i] = entSubscriptionToGrpc(sub, "", "")
		}
		var nextPageToken string
		if len(subs) >= int(pageSize) {
			nextPageToken = subs[len(subs)-1].ID.String()
		}
		resp = &pubsubpb.ListSubscriptionsResponse{
			Subscriptions: grpcSubscriptions,
			NextPageToken: nextPageToken,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *subscriberServer) DeleteSubscription(
	ctx context.Context,
	req *pubsubpb.DeleteSubscriptionRequest,
) (*emptypb.Empty, error) {
	if !isValidSubscriptionName(req.Subscription) {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Unsupported project / subscription path %s",
			req.Subscription,
		)
	}

	action := actions.NewDeleteSubscription(req.Subscription)
	err := s.client.DoCtxTx(ctx, nil, action.Execute)
	if err != nil {
		if isNotFound(err) {
			return nil, status.Errorf(
				codes.NotFound,
				"Subscription does not exist: %s",
				req.Subscription,
			)
		}
		return nil, grpc.AsStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *subscriberServer) ModifyAckDeadline(
	ctx context.Context,
	req *pubsubpb.ModifyAckDeadlineRequest,
) (*emptypb.Empty, error) {
	if !isValidSubscriptionName(req.Subscription) {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Unsupported project / subscription path %s",
			req.Subscription,
		)
	}

	// NOTE: other than the format validation, we ignore req.Subscription, as it
	// is not needed to resolve the AckIDs

	var err error
	p := actions.DelayDeliveriesParams{
		Delay: time.Duration(req.AckDeadlineSeconds) * time.Second,
	}
	if p.IDs, err = parse.UUIDsFromStrings(req.AckIds); err != nil {
		return nil, grpc.AsStatusError(err)
	}
	action := actions.NewDelayDeliveries(p)
	if err = s.client.DoCtxTxRetry(
		ctx,
		nil,
		action.Execute,
		postgres.RetryOnErrorCode(postgres.DeadlockDetected),
	); err != nil {
		// TODO: map error properly, skipped for now because DelayDeliveries isn't
		// expected to return any mappable errors
		return nil, grpc.AsStatusError(err)
	}

	return &emptypb.Empty{}, nil
}

func (s *subscriberServer) Acknowledge(
	ctx context.Context,
	req *pubsubpb.AcknowledgeRequest,
) (*emptypb.Empty, error) {
	if !isValidSubscriptionName(req.Subscription) {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Unsupported project / subscription path %s",
			req.Subscription,
		)
	}

	// NOTE: other than the format validation, we ignore req.Subscription, as it
	// is not needed to resolve the AckIDs

	var err error
	var ids []uuid.UUID
	if ids, err = parse.UUIDsFromStrings(req.AckIds); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	action := actions.NewAckDeliveries(ids...)
	if err = s.client.DoCtxTxRetry(
		ctx,
		nil,
		action.Execute,
		postgres.RetryOnErrorCode(postgres.DeadlockDetected),
	); err != nil {
		// TODO: map error properly, skipped for now because AckDeliveries isn't
		// expected to return any mappable errors
		return nil, grpc.AsStatusError(err)
	}

	return &emptypb.Empty{}, nil
}

func (s *subscriberServer) Pull(
	ctx context.Context,
	req *pubsubpb.PullRequest,
) (*pubsubpb.PullResponse, error) {
	if !isValidSubscriptionName(req.Subscription) {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Unsupported project / subscription path %s",
			req.Subscription,
		)
	}

	p := actions.GetSubscriptionMessagesParams{
		Name:        req.Subscription,
		MaxMessages: int(req.MaxMessages),
		MaxBytes:    10 * 1024 * 1024,
	}
	if req.ReturnImmediately { // nolint:staticcheck // deprecated
		// MaxWait = 0 means use a default, so we set the smallest possible non-zero
		// wait here. The action won't check this until after the first retrieval
		// attempt, so it doesn't matter if it's smaller than the time it takes to
		// check for messages.
		p.MaxWait = time.Nanosecond
	}
	action := actions.NewGetSubscriptionMessages(p)
	err := action.ExecuteClient(ctx, s.client)
	if err != nil {
		if isNotFound(err) {
			return nil, status.Errorf(
				codes.NotFound,
				"Subscription not found: %s",
				req.Subscription,
			)
		}
		return nil, grpc.AsStatusError(err)
	}
	results, _ := action.Results()
	msgs := make([]*pubsubpb.ReceivedMessage, len(results.Deliveries))
	for i, d := range results.Deliveries {
		msgs[i] = entDeliveryToGrpc(d)
	}

	return &pubsubpb.PullResponse{ReceivedMessages: msgs}, nil
}

func (s *subscriberServer) Seek(
	ctx context.Context,
	req *pubsubpb.SeekRequest,
) (*pubsubpb.SeekResponse, error) {
	if !isValidSubscriptionName(req.Subscription) {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Unsupported project / subscription path %s",
			req.Subscription,
		)
	}

	switch target := req.Target.(type) {
	case *pubsubpb.SeekRequest_Time:
		// FUTURE: do we want to bound how far in the future or past the target can be?
		action := actions.NewSeekSubscriptionToTime(actions.SeekSubscriptionToTimeParams{
			Name: req.Subscription,
			Time: target.Time.AsTime(),
		})
		if err := s.client.DoCtxTxRetry(
			ctx,
			nil,
			action.Execute,
			postgres.RetryOnErrorCode(postgres.DeadlockDetected),
		); err != nil {
			if isNotFound(err) {
				return nil, status.Errorf(codes.NotFound, "Subscription not found: %s", req.Subscription)
			}
			return nil, grpc.AsStatusError(err)
		}
		return &pubsubpb.SeekResponse{}, nil
	case *pubsubpb.SeekRequest_Snapshot:
		action := actions.NewSeekSubscriptionToSnapshot(actions.SeekSubscriptionToSnapshotParams{
			SubscriptionName: req.Subscription,
			SnapshotName:     target.Snapshot,
		})
		if err := s.client.DoCtxTxRetry(
			ctx,
			nil,
			action.Execute,
			postgres.RetryOnErrorCode(postgres.DeadlockDetected),
		); err != nil {
			if isNotFound(err) {
				return nil, status.Errorf(codes.NotFound,
					"Subscription or snapshot not found: %s / %s",
					req.Subscription, target.Snapshot,
				)
			}
			return nil, grpc.AsStatusError(err)
		}
		return &pubsubpb.SeekResponse{}, nil
	default:
		return nil, status.Errorf(codes.InvalidArgument, "Invalid type for SeekRequest.Target")
	}
}

func (s *subscriberServer) StreamingPull(stream pubsubpb.Subscriber_StreamingPullServer) error {
	// we require an initial message before we can start doing anything
	initial, err := stream.Recv()
	if err != nil {
		return grpc.WrapError(err, "Error receiving StreamingPull message")
	}

	if !isValidSubscriptionName(initial.Subscription) {
		return status.Errorf(
			codes.InvalidArgument,
			"Unsupported project / subscription path %s",
			initial.Subscription,
		)
	}

	ctx := stream.Context()

	var sub *ent.Subscription
	err = s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		sub, err = tx.Subscription.Query().
			Where(
				subscription.Name(initial.Subscription),
				subscription.DeletedAtIsNil(),
			).
			Only(ctx)
		return err
	})
	if err != nil {
		if isNotFound(err) {
			return status.Errorf(codes.NotFound, "Subscription not found: %s", initial.Subscription)
		}
		return grpc.AsStatusError(err)
	}

	// StreamAckDeadlineSeconds is required in the initial message, but we don't
	// use it, we auto-renew messages instead on the server side

	// We don't use ClientId, because we don't store any state across streams

	l := logging.GetLoggerWith("grpc/subscriber", func(c zerolog.Context) zerolog.Context {
		return c.
			Str("subscription", sub.Name).
			Str("clientId", initial.ClientId)
	})
	l.Debug().Msg("Starting streamer")

	streamer := &actions.MessageStreamer{
		Client:           s.client,
		Logger:           l,
		SubscriptionID:   &sub.ID,
		SubscriptionName: sub.Name,
		AutomaticNack:    true,
	}

	w := &streamWrapper{stream: stream, initial: initial}
	w.init()
	if err = streamer.Go(ctx, w); err == nil {
		l.Debug().Msg("Streamer ended normally")
		return nil
	}
	var stat *status.Status
	var ok bool
	if stat, ok = status.FromError(err); !ok {
		stat = status.FromContextError(err)
	}
	// these are context termination, and are boring
	if stat.Code() == codes.Canceled || stat.Code() == codes.DeadlineExceeded {
		l.Debug().Msg("Streamer ended with context termination")
	} else {
		if isNotFound(err) {
			// this happens when subscription is deleted mid-stream
			stat = status.New(codes.NotFound, err.Error())
		}
		l.Warn().Err(err).Stringer("code", stat.Code()).Msg("Streamer ended with error")
	}
	return stat.Err()
}

type streamWrapper struct {
	mu       sync.Mutex
	stream   pubsubpb.Subscriber_StreamingPullServer
	initial  *pubsubpb.StreamingPullRequest
	closed   chan struct{}
	receives chan streamReceiveItem
}
type streamReceiveItem struct {
	msg *pubsubpb.StreamingPullRequest
	err error
}

func (w *streamWrapper) init() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed == nil {
		w.closed = make(chan struct{})
	}
	if w.receives == nil {
		w.receives = make(chan streamReceiveItem)
	}
}

func (w *streamWrapper) Close() error {
	close(w.closed)
	// we don't close the receives channel to avoid potential panic on send to a
	// closed channel
	w.mu.Lock()
	w.receives = nil
	w.mu.Unlock()
	return nil
}

func (w *streamWrapper) Receive(context.Context) (*actions.MessageStreamRequest, error) {
	if w.initial != nil {
		ret, err := w.adaptIn(w.initial)
		w.initial = nil
		return ret, err
	}
	// we can't interrupt receive without killing the stream, and we can't kill
	// the stream until we return the final state to the client, so we have to
	// fire it into the background and not directly wait for it
	go func() {
		w.mu.Lock()
		r := w.receives
		w.mu.Unlock()
		m, err := w.stream.Recv()
		select {
		case <-w.closed:
			// abandon
		case r <- streamReceiveItem{m, err}:
			// notified
		}
	}()
	w.mu.Lock()
	r := w.receives
	w.mu.Unlock()
	select {
	case <-w.closed:
		// this is equivalent to context cancellation
		return nil, context.Canceled
	case rm := <-r:
		if rm.err != nil {
			return nil, rm.err
		}
		return w.adaptIn(rm.msg)
	}
}

func (w *streamWrapper) Send(ctx context.Context, m *actions.SubscriptionMessageDelivery) error {
	return w.stream.Send(&pubsubpb.StreamingPullResponse{
		ReceivedMessages: []*pubsubpb.ReceivedMessage{entDeliveryToGrpc(m)},
	})
}

func (w *streamWrapper) SendBatch(
	ctx context.Context,
	ms []*actions.SubscriptionMessageDelivery,
) error {
	rm := make([]*pubsubpb.ReceivedMessage, len(ms))
	for i, m := range ms {
		rm[i] = entDeliveryToGrpc(m)
	}
	return w.stream.Send(&pubsubpb.StreamingPullResponse{
		ReceivedMessages: rm,
	})
}

var _ actions.StreamConnectionBatchSend = (*streamWrapper)(nil)

func (w *streamWrapper) adaptIn(
	m *pubsubpb.StreamingPullRequest,
) (*actions.MessageStreamRequest, error) {
	ret := &actions.MessageStreamRequest{}
	if m == w.initial {
		// for Google PubSub, the flow control can only be set on the first request
		fc := effectiveFlowControl(m.MaxOutstandingMessages, m.MaxOutstandingBytes)
		ret.FlowControl = &fc
	}
	// NOTE: At least the NodeJS client doesn't seem to send us acks this way, but
	// instead uses the standalone ack method
	if len(m.AckIds) != 0 {
		var err error
		if ret.Ack, err = parse.UUIDsFromStrings(m.AckIds); err != nil {
			return nil, err
		}
	}
	// we could actually skip this for how we work, but better to implement the spec where we can
	if len(m.ModifyDeadlineSeconds) != len(m.ModifyDeadlineAckIds) {
		return nil, status.Error(
			codes.InvalidArgument,
			"Must have same len for ModifyDeadlineSeconds and ModifyDeadlineAckIds",
		)
	}
	if len(m.ModifyDeadlineAckIds) != 0 {
		var err error
		if ret.Delay, err = parse.UUIDsFromStrings(m.ModifyDeadlineAckIds); err != nil {
			return nil, err
		}
		// we don't support per-message delay, so take the max delay of the set
		for _, d := range m.ModifyDeadlineSeconds {
			df := float64(d)
			if ret.DelaySeconds < df {
				ret.DelaySeconds = df
			}
		}
	}
	return ret, nil
}

func (s *subscriberServer) ModifyPushConfig(
	ctx context.Context,
	req *pubsubpb.ModifyPushConfigRequest,
) (*emptypb.Empty, error) {
	if !isValidSubscriptionName(req.Subscription) {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Unsupported project / subscription path %s",
			req.Subscription,
		)
	}
	if err := validatePushConfig(req.PushConfig); err != nil {
		return nil, err
	}
	err := s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		sub, err := tx.Subscription.Query().
			Where(
				subscription.Name(req.Subscription),
				subscription.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			if isNotFound(err) {
				return status.Errorf(codes.NotFound, "Subscription not found: %s", req.Subscription)
			}
			return grpc.AsStatusError(err)
		}
		mut := applyPushConfig(sub.Update(), req.PushConfig)
		actions.NotifyModifySubscription(tx, sub.ID, sub.Name)
		return mut.Exec(ctx)
	})
	if err != nil {
		return nil, grpc.AsStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

func validatePushConfig(cfg *pubsubpb.PushConfig) error {
	attrs := cfg.GetAttributes()
	for k, v := range attrs {
		if k != "x-goog-version" {
			return status.Errorf(codes.InvalidArgument, "Unsupported attribute %s", k)
		}
		if v != "v1" {
			return status.Errorf(codes.InvalidArgument, "Unsupported 'x-goog-version': %s", v)
		}
	}
	if cfg.AuthenticationMethod != nil {
		return status.Errorf(codes.Unimplemented, "PushConfig.AuthenticationMethod not supported")
	}
	return nil
}

func applyPushConfig(
	mut *ent.SubscriptionUpdateOne,
	cfg *pubsubpb.PushConfig,
) *ent.SubscriptionUpdateOne {
	ep := cfg.GetPushEndpoint()
	if ep == "" {
		mut = mut.ClearPushEndpoint()
	} else {
		mut = mut.SetPushEndpoint(ep)
	}
	return mut
}

func entSubscriptionToGrpc(
	subscription *ent.Subscription,
	topicName, deadLetterTopicName string,
) *pubsubpb.Subscription {
	nominalDelay, _ := actions.NextDelayFor(subscription, 0)
	ret := &pubsubpb.Subscription{
		Name: subscription.Name,
		// TODO: this is a fudge, based on the initial retry backoff (pubsub
		// differentiates ack deadlines vs retry backoffs, we don't)
		AckDeadlineSeconds: int32(nominalDelay.Seconds()),
		// we do retain acked messages, but not in the sense or for the purpose that
		// Google means, esp. not indefinitely
		RetainAckedMessages:      false,
		MessageRetentionDuration: durationpb.New(time.Duration(subscription.MessageTTL)),
		Labels:                   subscription.Labels,
		EnableMessageOrdering:    subscription.OrderedDelivery,
		ExpirationPolicy: &pubsubpb.ExpirationPolicy{
			Ttl: durationpb.New(time.Duration(subscription.TTL)),
		},
		// not supported: Detached
	}
	if subscription.PushEndpoint != nil {
		ret.PushConfig = &pubsubpb.PushConfig{
			PushEndpoint: *subscription.PushEndpoint,
		}
	}
	if subscription.MessageFilter != nil {
		ret.Filter = *subscription.MessageFilter
	}
	if subscription.DeadLetterTopicID != nil {
		ret.DeadLetterPolicy = &pubsubpb.DeadLetterPolicy{}
		if subscription.Edges.DeadLetterTopic != nil {
			if subscription.Edges.DeadLetterTopic.DeletedAt != nil {
				deadLetterTopicName = deletedTopicName
			} else {
				deadLetterTopicName = subscription.Edges.DeadLetterTopic.Name
			}
		}
		ret.DeadLetterPolicy.DeadLetterTopic = deadLetterTopicName
		if subscription.MaxDeliveryAttempts != nil {
			ret.DeadLetterPolicy.MaxDeliveryAttempts = *subscription.MaxDeliveryAttempts
		}
	}
	if subscription.MinBackoff != nil || subscription.MaxBackoff != nil {
		ret.RetryPolicy = &pubsubpb.RetryPolicy{}
		if subscription.MinBackoff != nil {
			ret.RetryPolicy.MinimumBackoff = durationpb.New(time.Duration(*subscription.MinBackoff))
		}
		if subscription.MaxBackoff != nil {
			ret.RetryPolicy.MaximumBackoff = durationpb.New(time.Duration(*subscription.MaxBackoff))
		}
	}
	if subscription.MessageFilter != nil {
		ret.Filter = *subscription.MessageFilter
	}
	if subscription.Edges.Topic != nil {
		if subscription.Edges.Topic.DeletedAt != nil {
			ret.Topic = deletedTopicName
		} else {
			ret.Topic = subscription.Edges.Topic.Name
		}
	} else if topicName != "" {
		ret.Topic = topicName
	}
	return ret
}

func entDeliveryToGrpc(m *actions.SubscriptionMessageDelivery) *pubsubpb.ReceivedMessage {
	ret := &pubsubpb.ReceivedMessage{
		AckId: m.ID.String(),
		Message: &pubsubpb.PubsubMessage{
			MessageId:   m.MessageID.String(),
			PublishTime: timestamppb.New(m.PublishedAt),
			Data:        m.Payload,
			Attributes:  m.Attributes,
		},
		DeliveryAttempt: int32(m.NumAttempts),
	}
	if m.OrderKey != nil {
		ret.Message.OrderingKey = *m.OrderKey
	}
	return ret
}

func effectiveFlowControl(maxMessages, maxBytes int64) actions.FlowControl {
	fc := actions.FlowControl{
		MaxMessages: int(maxMessages),
		MaxBytes:    int(maxBytes),
	}
	const maxInt = int((^uint(0)) >> 1)
	if maxMessages > int64(maxInt) {
		fc.MaxMessages = maxInt
	} else if fc.MaxMessages <= 0 {
		fc.MaxMessages = 1000
	}
	if maxBytes > int64(maxInt) {
		fc.MaxBytes = maxInt
	} else if fc.MaxBytes <= 0 {
		fc.MaxBytes = 10 * 1024 * 1024
	}
	return fc
}

const (
	defaultSubscriptionTTL        = 30 * 24 * time.Hour
	defaultSubscriptionMessageTTL = 7 * 24 * time.Hour
)

const (
	deletedTopicName             = "_deleted-topic_"
	defaultDeadLetterMaxAttempts = 5
)
