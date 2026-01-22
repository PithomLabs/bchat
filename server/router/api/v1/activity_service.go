package v1

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) GetActivity(ctx context.Context, request *v1pb.GetActivityRequest) (*v1pb.Activity, error) {
	activityID, err := ExtractActivityIDFromName(request.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid activity name: %v", err)
	}
	activity, err := s.Store.GetActivity(ctx, &store.FindActivity{
		ID: &activityID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get activity: %v", err)
	}

	activityMessage, err := s.convertActivityFromStore(ctx, activity)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert activity from store: %v", err)
	}
	return activityMessage, nil
}

func (s *APIV1Service) convertActivityFromStore(ctx context.Context, activity *store.Activity) (*v1pb.Activity, error) {
	payload, err := s.convertActivityPayloadFromStore(ctx, activity.Payload)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert activity payload from store: %v", err)
	}
	return &v1pb.Activity{
		Name:       fmt.Sprintf("%s%d", ActivityNamePrefix, activity.ID),
		Creator:    fmt.Sprintf("%s%d", UserNamePrefix, activity.CreatorID),
		Type:       activity.Type.String(),
		Level:      activity.Level.String(),
		CreateTime: timestamppb.New(time.Unix(activity.CreatedTs, 0)),
		Payload:    payload,
	}, nil
}

func (s *APIV1Service) convertActivityPayloadFromStore(ctx context.Context, payload *storepb.ActivityPayload) (*v1pb.ActivityPayload, error) {
	v2Payload := &v1pb.ActivityPayload{}
	if payload.MemoComment != nil {
		memo, err := s.Store.GetMemo(ctx, &store.FindMemo{
			ID:             &payload.MemoComment.MemoId,
			ExcludeContent: true,
		})
		if err != nil {
			// Log the error but don't fail - memo might be deleted or inaccessible
			// This allows other notifications to still work
			// TODO: Add proper logging
			return v2Payload, nil
		}
		if memo == nil {
			// Memo was deleted or doesn't exist - skip this payload
			// Frontend will handle empty payload gracefully (won't display)
			return v2Payload, nil
		}
		relatedMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
			ID:             &payload.MemoComment.RelatedMemoId,
			ExcludeContent: true,
		})
		if err != nil {
			// Log but don't fail
			return v2Payload, nil
		}
		if relatedMemo == nil {
			// Related memo was deleted - skip this payload
			return v2Payload, nil
		}
		v2Payload.MemoComment = &v1pb.ActivityMemoCommentPayload{
			Memo:        fmt.Sprintf("%s%s", MemoNamePrefix, memo.UID),
			RelatedMemo: fmt.Sprintf("%s%s", MemoNamePrefix, relatedMemo.UID),
		}
	} else if payload.TicketComment != nil {
		v2Payload.TicketComment = &v1pb.ActivityTicketCommentPayload{
			TicketId: payload.TicketComment.TicketId,
		}
	}
	return v2Payload, nil
}
