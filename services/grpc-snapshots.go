// Copyright (c) 2024 6 River Systems
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

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.6river.tech/mmmbbb/actions"
	"go.6river.tech/mmmbbb/db/postgres"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/predicate"
	"go.6river.tech/mmmbbb/ent/snapshot"
	"go.6river.tech/mmmbbb/grpc"
	"go.6river.tech/mmmbbb/grpc/pubsubpb"
)

func (s *subscriberServer) GetSnapshot(
	ctx context.Context,
	req *pubsubpb.GetSnapshotRequest,
) (*pubsubpb.Snapshot, error) {
	if !isValidSnapshotName(req.Snapshot) {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Unsupported project / snapshot path %s",
			req.Snapshot,
		)
	}

	var resp *pubsubpb.Snapshot
	err := s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		s, err := tx.Snapshot.Query().
			Where(
				snapshot.Name(req.Snapshot),
			).
			WithTopic().
			Only(ctx)
		if err != nil {
			return err
		}
		resp = entSnapshotToGrpc(s, "")
		return nil
	})
	if err != nil {
		if isNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "Snapshot not found: %s", req.Snapshot)
		}
		return nil, grpc.AsStatusError(err)
	}
	return resp, nil
}

func (s *subscriberServer) ListSnapshots(
	ctx context.Context,
	req *pubsubpb.ListSnapshotsRequest,
) (*pubsubpb.ListSnapshotsResponse, error) {
	var pageSize int32 = 100
	if req.PageSize > 0 && req.PageSize < pageSize {
		pageSize = req.PageSize
	}

	var resp *pubsubpb.ListSnapshotsResponse
	err := s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		predicates := []predicate.Snapshot{
			snapshot.NameHasPrefix(projectSubscriptionPrefix(req.Project)),
		}
		if req.PageToken != "" {
			pageID, err := uuid.Parse(req.PageToken)
			if err != nil {
				return status.Error(codes.InvalidArgument, err.Error())
			}
			predicates = append(predicates, snapshot.IDGT(pageID))
		}

		snaps, err := tx.Snapshot.Query().
			Where(predicates...).
			WithTopic().
			Order(ent.Asc(snapshot.FieldID)).
			Limit(int(pageSize)).
			All(ctx)
		if err != nil {
			return grpc.AsStatusError(err)
		}
		grpcSnapshots := make([]*pubsubpb.Snapshot, len(snaps))
		for i, snap := range snaps {
			grpcSnapshots[i] = entSnapshotToGrpc(snap, "")
		}
		var nextPageToken string
		if len(snaps) >= int(pageSize) {
			nextPageToken = snaps[len(snaps)-1].ID.String()
		}
		resp = &pubsubpb.ListSnapshotsResponse{
			Snapshots:     grpcSnapshots,
			NextPageToken: nextPageToken,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *subscriberServer) CreateSnapshot(
	ctx context.Context,
	req *pubsubpb.CreateSnapshotRequest,
) (*pubsubpb.Snapshot, error) {
	if !isValidSnapshotName(req.Name) {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Unsupported project / snapshot path %s",
			req.Name,
		)
	}
	if !isValidSubscriptionName(req.Subscription) {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Unsupported project / subscription path %s",
			req.Subscription,
		)
	}

	params := actions.CreateSnapshotParams{
		SubscriptionName: req.Subscription,
		Name:             req.Name,
		Labels:           req.Labels,
	}
	action := actions.NewCreateSnapshot(params)
	if err := s.client.DoCtxTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable}, action.Execute); err != nil {
		if isNotFound(err) {
			return nil, status.Errorf(
				codes.NotFound,
				"Subscription not found: %s",
				req.Subscription,
			)
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
	return entSnapshotToGrpc(results.Snapshot, results.TopicName), nil
}

func (s *subscriberServer) UpdateSnapshot(
	ctx context.Context,
	req *pubsubpb.UpdateSnapshotRequest,
) (*pubsubpb.Snapshot, error) {
	// TODO: implement this if we need to
	return nil, status.Errorf(codes.Unimplemented, "method UpdateSnapshot not implemented")
}

func (s *subscriberServer) DeleteSnapshot(
	ctx context.Context,
	req *pubsubpb.DeleteSnapshotRequest,
) (*empty.Empty, error) {
	if !isValidSnapshotName(req.Snapshot) {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"Unsupported project / snapshot path %s",
			req.Snapshot,
		)
	}

	// TODO: implement this as an action
	if n, err := s.client.Snapshot.Delete().
		Where(snapshot.Name(req.Snapshot)).
		Exec(ctx); err != nil {
		return nil, grpc.AsStatusError(err)
	} else if n == 0 {
		return nil, status.Errorf(codes.NotFound, "Snapshot not found: %s", req.Snapshot)
	} else {
		return &empty.Empty{}, nil
	}
}

func entSnapshotToGrpc(snapshot *ent.Snapshot, topicName string) *pubsubpb.Snapshot {
	ret := &pubsubpb.Snapshot{
		Name:       snapshot.Name,
		ExpireTime: timestamppb.New(snapshot.ExpiresAt),
		Labels:     snapshot.Labels,
	}
	if snapshot.Edges.Topic != nil {
		if snapshot.Edges.Topic.DeletedAt != nil {
			// should never happen, but just in case
			ret.Topic = deletedTopicName
		} else {
			ret.Topic = snapshot.Edges.Topic.Name
		}
	} else if topicName != "" {
		ret.Topic = topicName
	}
	return ret
}
