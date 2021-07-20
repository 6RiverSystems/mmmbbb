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
	"encoding/json"

	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"go.6river.tech/gosix/db/postgres"
	"go.6river.tech/gosix/grpc"
	"go.6river.tech/mmmbbb/actions"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/predicate"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/ent/topic"
	"go.6river.tech/mmmbbb/grpc/pubsub"
)

type publisherServer struct {
	pubsub.UnimplementedPublisherServer

	client *ent.Client
}

func (s *publisherServer) ListTopics(ctx context.Context, req *pubsub.ListTopicsRequest) (*pubsub.ListTopicsResponse, error) {
	var pageSize int32 = 100
	if req.PageSize > 0 && req.PageSize < pageSize {
		pageSize = req.PageSize
	}

	var resp *pubsub.ListTopicsResponse
	err := s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		predicates := []predicate.Topic{
			topic.NameHasPrefix(projectTopicPrefix(req.Project)),
			topic.DeletedAtIsNil(),
		}
		if req.PageToken != "" {
			pageID, err := uuid.Parse(req.PageToken)
			if err != nil {
				return status.Error(codes.InvalidArgument, err.Error())
			}
			predicates = append(predicates, topic.IDGT(pageID))
		}
		topics, err := tx.Topic.Query().
			Where(predicates...).
			Order(ent.Asc(topic.FieldID)).
			Limit(int(pageSize)).
			All(ctx)
		if err != nil {
			return grpc.AsStatusError(err)
		}
		grpcTopics := make([]*pubsub.Topic, len(topics))
		for i, t := range topics {
			grpcTopics[i] = entTopicToGrpc(t)
		}
		var nextPageToken string
		if len(topics) >= int(pageSize) {
			nextPageToken = topics[len(topics)-1].ID.String()
		}
		resp = &pubsub.ListTopicsResponse{Topics: grpcTopics, NextPageToken: nextPageToken}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *publisherServer) CreateTopic(ctx context.Context, req *pubsub.Topic) (*pubsub.Topic, error) {
	if !isValidTopicName(req.Name) {
		return nil, status.Errorf(codes.InvalidArgument, "Unsupported project / topic path %s", req.Name)
	}

	if req.MessageStoragePolicy != nil || req.KmsKeyName != "" || req.SchemaSettings != nil {
		return nil, status.Error(codes.Unimplemented, "Advanced features not supported")
	}

	// SatisfiesPzs is documented as something only returned from the server,
	// should be ignored in requests. What it does isn't documented anyways,
	// clearly a reserved field for some unreleased feature as of 2021-02-04

	action := actions.NewCreateTopic(actions.CreateTopicParams{
		Name:   req.Name,
		Labels: req.Labels,
	})
	err := s.client.DoCtxTx(ctx, nil, action.Execute)
	if err != nil {
		if errors.Is(err, actions.ErrExists) {
			return nil, status.Errorf(codes.AlreadyExists, "Topic already exists: %s", req.Name)
		}
		if _, ok := postgres.IsPostgreSQLErrorCode(err, postgres.SerializationFailure); ok {
			return nil, status.Error(codes.Aborted, err.Error())
		}
		return nil, grpc.AsStatusError(err)
	}
	resp := proto.Clone(req).(*pubsub.Topic)
	return resp, nil
}

func (s *publisherServer) GetTopic(ctx context.Context, req *pubsub.GetTopicRequest) (*pubsub.Topic, error) {
	if !isValidTopicName(req.Topic) {
		return nil, status.Errorf(codes.InvalidArgument, "Unsupported project / topic path %s", req.Topic)
	}

	var resp *pubsub.Topic
	err := s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		t, err := tx.Topic.Query().
			Where(
				topic.Name(req.Topic),
				topic.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return err
		}
		resp = entTopicToGrpc(t)
		return nil
	})
	if err != nil {
		if isNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "Topic not found: %s", req.Topic)
		}
		return nil, grpc.AsStatusError(err)
	}
	return resp, nil
}

func (s *publisherServer) UpdateTopic(ctx context.Context, req *pubsub.UpdateTopicRequest) (*pubsub.Topic, error) {
	if !isValidTopicName(req.Topic.Name) {
		return nil, status.Errorf(codes.InvalidArgument, "Unsupported project / topic path %s", req.Topic.Name)
	}

	var resp *pubsub.Topic
	err := s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		t, err := tx.Topic.Query().
			Where(
				topic.Name(req.Topic.Name),
				topic.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			if isNotFound(err) {
				return status.Errorf(codes.NotFound, "Topic not found: %s", req.Topic)
			}
			return grpc.AsStatusError(err)
		}

		topicUpdate := tx.Topic.UpdateOne(t)
		for _, p := range req.UpdateMask.GetPaths() {
			// NOTE: `p` is the protobuf property path, not the Go one the accessible
			// version of the generator package that knows how to convert from the
			// protobuf field to the Go field name generates deprecation warnings, so,
			// we just use the protobuf name
			switch p {
			case "name":
				return status.Error(codes.InvalidArgument, "Topics cannot be renamed")
			case "labels":
				topicUpdate.SetLabels(req.Topic.Labels)
			case "message_storage_policy", "kms_key_name", "schema_settings", "satisfies_pzs":
				// these are valid paths, we just don't support changing them
				return status.Errorf(codes.Unimplemented, "Modifying Topic.%s is not supported", p)
			default:
				return status.Errorf(codes.InvalidArgument, "Modifying Topic.%s is not a recognized topic property", p)
			}
		}

		// don't bother issuing the save if it's a no-op
		if len(topicUpdate.Mutation().Fields()) == 0 {
			return nil
		}

		if t, err = topicUpdate.Save(ctx); err != nil {
			return status.Errorf(codes.Unknown, err.Error())
		}

		actions.NotifyModifyTopic(tx, t.ID, t.Name)

		resp = entTopicToGrpc(t)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *publisherServer) DeleteTopic(ctx context.Context, req *pubsub.DeleteTopicRequest) (*emptypb.Empty, error) {
	if !isValidTopicName(req.Topic) {
		return nil, status.Errorf(codes.InvalidArgument, "Unsupported project / topic path %s", req.Topic)
	}

	action := actions.NewDeleteTopic(req.Topic)
	err := s.client.DoCtxTx(ctx, nil, action.Execute)
	if err != nil {
		if isNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "Topic not found: %s", req.Topic)
		}
		return nil, grpc.AsStatusError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *publisherServer) ListTopicSubscriptions(
	ctx context.Context,
	req *pubsub.ListTopicSubscriptionsRequest,
) (*pubsub.ListTopicSubscriptionsResponse, error) {
	if !isValidTopicName(req.Topic) {
		return nil, status.Errorf(codes.InvalidArgument, "Unsupported project / topic path %s", req.Topic)
	}

	var pageSize int32 = 100
	if req.PageSize > 0 && req.PageSize < pageSize {
		pageSize = req.PageSize
	}

	var resp *pubsub.ListTopicSubscriptionsResponse
	err := s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		topic, err := tx.Topic.Query().
			Where(
				topic.Name(req.Topic),
				topic.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			if isNotFound(err) {
				return status.Errorf(codes.NotFound, "Topic not found: %s", req.Topic)
			}
			return grpc.AsStatusError(err)
		}
		subPredicates := []predicate.Subscription{
			subscription.DeletedAtIsNil(),
		}
		if req.PageToken != "" {
			pageID, err := uuid.Parse(req.PageToken)
			if err != nil {
				return status.Error(codes.InvalidArgument, err.Error())
			}
			subPredicates = append(subPredicates, subscription.IDGT(pageID))
		}

		subs, err := topic.QuerySubscriptions().
			Where(subPredicates...).
			Order(ent.Asc(subscription.FieldID)).
			Limit(int(pageSize)).
			All(ctx)
		if err != nil {
			return grpc.AsStatusError(err)
		}

		subNames := make([]string, len(subs))
		for i, s := range subs {
			subNames[i] = s.Name
		}
		var nextPageToken string
		if len(subs) >= int(pageSize) {
			nextPageToken = subs[len(subs)-1].ID.String()
		}
		resp = &pubsub.ListTopicSubscriptionsResponse{Subscriptions: subNames, NextPageToken: nextPageToken}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *publisherServer) Publish(ctx context.Context, req *pubsub.PublishRequest) (*pubsub.PublishResponse, error) {
	if !isValidTopicName(req.Topic) {
		return nil, status.Errorf(codes.InvalidArgument, "Unsupported project / topic path %s", req.Topic)
	}

	var resp *pubsub.PublishResponse
	err := s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		t, err := tx.Topic.Query().
			Where(
				topic.Name(req.Topic),
				topic.DeletedAtIsNil(),
			).
			WithSubscriptions(func(sq *ent.SubscriptionQuery) {
				sq.Where(subscription.DeletedAtIsNil())
			}).
			Only(ctx)
		if err != nil {
			if isNotFound(err) {
				return status.Errorf(codes.NotFound, "Topic not found: %s", req.Topic)
			}
			return grpc.AsStatusError(err)
		}

		resp = &pubsub.PublishResponse{MessageIds: make([]string, len(req.Messages))}

		for i, m := range req.Messages {
			// TODO: we don't expect to receive an m.PublishTime or m.MessageId, right?

			action := actions.NewPublishMessage(actions.PublishMessageParams{
				TopicID:    &t.ID,
				TopicName:  t.Name,
				Payload:    json.RawMessage(m.Data),
				Attributes: m.Attributes,
				OrderKey:   m.OrderingKey,
			})
			err := action.Execute(ctx, tx)
			if err != nil {
				// no expected logic errors here, only db weirdness
				return grpc.AsStatusError(err)
			}
			resp.MessageIds[i] = action.MessageID().String()
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Don't intend to implement: ListTopicSnapshots

// Don't intend to implement: DetachSubscription

func entTopicToGrpc(topic *ent.Topic) *pubsub.Topic {
	return &pubsub.Topic{
		Name:   topic.Name,
		Labels: topic.Labels,
	}
}
