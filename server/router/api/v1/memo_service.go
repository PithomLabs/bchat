package v1

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lithammer/shortuuid/v4"
	"github.com/pkg/errors"
	"github.com/usememos/gomark/ast"
	"github.com/usememos/gomark/parser"
	"github.com/usememos/gomark/parser/tokenizer"
	"github.com/usememos/gomark/renderer"
	"github.com/usememos/gomark/restore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/usememos/memos/plugin/webhook"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/server/runner/memopayload"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) CreateMemo(ctx context.Context, request *v1pb.CreateMemoRequest) (*v1pb.Memo, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user")
	}

	create := &store.Memo{
		UID:        shortuuid.New(),
		CreatorID:  user.ID,
		Content:    request.Memo.Content,
		Visibility: convertVisibilityToStore(request.Memo.Visibility),
	}
	if !isSuperUser(user) {
		create.Visibility = store.Private
	}
	workspaceMemoRelatedSetting, err := s.Store.GetWorkspaceMemoRelatedSetting(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get workspace memo related setting")
	}
	if workspaceMemoRelatedSetting.DisallowPublicVisibility && create.Visibility == store.Public {
		return nil, status.Errorf(codes.PermissionDenied, "disable public memos system setting is enabled")
	}
	contentLengthLimit, err := s.getContentLengthLimit(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get content length limit")
	}
	if len(create.Content) > contentLengthLimit {
		return nil, status.Errorf(codes.InvalidArgument, "content too long (max %d characters)", contentLengthLimit)
	}
	if err := memopayload.RebuildMemoPayload(create); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to rebuild memo payload: %v", err)
	}
	if request.Memo.Location != nil {
		create.Payload.Location = convertLocationToStore(request.Memo.Location)
	}

	memo, err := s.Store.CreateMemo(ctx, create)
	if err != nil {
		return nil, err
	}
	if len(request.Memo.Resources) > 0 {
		_, err := s.SetMemoResources(ctx, &v1pb.SetMemoResourcesRequest{
			Name:      fmt.Sprintf("%s%s", MemoNamePrefix, memo.UID),
			Resources: request.Memo.Resources,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to set memo resources")
		}
	}
	if len(request.Memo.Relations) > 0 {
		_, err := s.SetMemoRelations(ctx, &v1pb.SetMemoRelationsRequest{
			Name:      fmt.Sprintf("%s%s", MemoNamePrefix, memo.UID),
			Relations: request.Memo.Relations,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to set memo relations")
		}
	}

	memoMessage, err := s.convertMemoFromStore(ctx, memo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert memo")
	}
	// Try to dispatch webhook when memo is created.
	if err := s.DispatchMemoCreatedWebhook(ctx, memoMessage); err != nil {
		slog.Warn("Failed to dispatch memo created webhook", slog.Any("err", err))
	}

	// Dispatch mentions
	if err := s.dispatchMemoMentions(ctx, memo); err != nil {
		slog.Warn("Failed to dispatch memo mentions", slog.Any("err", err))
	}

	if !isSuperUser(user) {
		isEscalated := s.handleAutoTicketCreation(ctx, memo, user)
		if !isEscalated {
			go s.handleTicketAIResponse(context.Background(), memo.UID, user.ID, memo.Content)
		}
	}

	return memoMessage, nil
}

func (s *APIV1Service) ListMemos(ctx context.Context, request *v1pb.ListMemosRequest) (*v1pb.ListMemosResponse, error) {
	memoFind := &store.FindMemo{
		// Exclude comments by default.
		ExcludeComments: true,
	}
	if err := s.buildMemoFindWithFilter(ctx, memoFind, request.OldFilter); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to build find memos with filter: %v", err)
	}
	if request.Parent != "" && request.Parent != "users/-" {
		userID, err := ExtractUserIDFromName(request.Parent)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid parent: %v", err)
		}
		memoFind.CreatorID = &userID
		memoFind.OrderByPinned = true
	}
	if request.State == v1pb.State_ARCHIVED {
		state := store.Archived
		memoFind.RowStatus = &state
	} else {
		state := store.Normal
		memoFind.RowStatus = &state
	}
	if request.Direction == v1pb.Direction_ASC {
		memoFind.OrderByTimeAsc = true
	}
	if request.Filter != "" {
		if err := s.validateFilter(ctx, request.Filter); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid filter: %v", err)
		}
		memoFind.Filter = &request.Filter
	}

	currentUser, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user")
	}
	if currentUser == nil {
		memoFind.VisibilityList = []store.Visibility{store.Public}
	} else {
		if memoFind.CreatorID == nil {
			if !isSuperUser(currentUser) {
				internalFilter := fmt.Sprintf(`creator_id == %d || visibility in ["PUBLIC", "PROTECTED"]`, currentUser.ID)
				if memoFind.Filter != nil {
					filter := fmt.Sprintf("(%s) && (%s)", *memoFind.Filter, internalFilter)
					memoFind.Filter = &filter
				} else {
					memoFind.Filter = &internalFilter
				}
			}
		} else if *memoFind.CreatorID != currentUser.ID {
			if !isSuperUser(currentUser) {
				memoFind.VisibilityList = []store.Visibility{store.Public, store.Protected}
			}
		}
	}

	workspaceMemoRelatedSetting, err := s.Store.GetWorkspaceMemoRelatedSetting(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get workspace memo related setting")
	}
	if workspaceMemoRelatedSetting.DisplayWithUpdateTime {
		memoFind.OrderByUpdatedTs = true
	}

	var limit, offset int
	if request.PageToken != "" {
		var pageToken v1pb.PageToken
		if err := unmarshalPageToken(request.PageToken, &pageToken); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid page token: %v", err)
		}
		limit = int(pageToken.Limit)
		offset = int(pageToken.Offset)
	} else {
		limit = int(request.PageSize)
	}
	if limit <= 0 {
		limit = DefaultPageSize
	}
	limitPlusOne := limit + 1
	memoFind.Limit = &limitPlusOne
	memoFind.Offset = &offset
	memos, err := s.Store.ListMemos(ctx, memoFind)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list memos: %v", err)
	}

	memoMessages := []*v1pb.Memo{}
	nextPageToken := ""
	if len(memos) == limitPlusOne {
		memos = memos[:limit]
		nextPageToken, err = getPageToken(limit, offset+limit)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get next page token, error: %v", err)
		}
	}
	for _, memo := range memos {
		memoMessage, err := s.convertMemoFromStore(ctx, memo)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert memo")
		}
		memoMessages = append(memoMessages, memoMessage)
	}

	response := &v1pb.ListMemosResponse{
		Memos:         memoMessages,
		NextPageToken: nextPageToken,
	}
	return response, nil
}

func (s *APIV1Service) GetMemo(ctx context.Context, request *v1pb.GetMemoRequest) (*v1pb.Memo, error) {
	memoUID, err := ExtractMemoUIDFromName(request.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
	}
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{
		UID: &memoUID,
	})
	if err != nil {
		return nil, err
	}
	if memo == nil {
		return nil, status.Errorf(codes.NotFound, "memo not found")
	}
	if memo.Visibility != store.Public {
		user, err := s.GetCurrentUser(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get user")
		}
		if user == nil {
			return nil, status.Errorf(codes.PermissionDenied, "permission denied")
		}
		if memo.Visibility == store.Private && memo.CreatorID != user.ID && !isSuperUser(user) {
			return nil, status.Errorf(codes.PermissionDenied, "permission denied")
		}
	}

	memoMessage, err := s.convertMemoFromStore(ctx, memo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert memo")
	}
	return memoMessage, nil
}

func (s *APIV1Service) UpdateMemo(ctx context.Context, request *v1pb.UpdateMemoRequest) (*v1pb.Memo, error) {
	memoUID, err := ExtractMemoUIDFromName(request.Memo.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
	}
	if request.UpdateMask == nil || len(request.UpdateMask.Paths) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "update mask is required")
	}

	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil {
		return nil, err
	}
	if memo == nil {
		return nil, status.Errorf(codes.NotFound, "memo not found")
	}

	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user")
	}
	// Only the creator or admin can update the memo.
	if memo.CreatorID != user.ID && !isSuperUser(user) {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	update := &store.UpdateMemo{
		ID: memo.ID,
	}
	for _, path := range request.UpdateMask.Paths {
		if path == "content" {
			contentLengthLimit, err := s.getContentLengthLimit(ctx)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get content length limit")
			}
			if len(request.Memo.Content) > contentLengthLimit {
				return nil, status.Errorf(codes.InvalidArgument, "content too long (max %d characters)", contentLengthLimit)
			}
			memo.Content = request.Memo.Content
			if err := memopayload.RebuildMemoPayload(memo); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to rebuild memo payload: %v", err)
			}
			update.Content = &memo.Content
			update.Payload = memo.Payload
		} else if path == "visibility" {
			workspaceMemoRelatedSetting, err := s.Store.GetWorkspaceMemoRelatedSetting(ctx)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get workspace memo related setting")
			}
			visibility := convertVisibilityToStore(request.Memo.Visibility)
			if workspaceMemoRelatedSetting.DisallowPublicVisibility && visibility == store.Public {
				return nil, status.Errorf(codes.PermissionDenied, "disable public memos system setting is enabled")
			}
			update.Visibility = &visibility
		} else if path == "pinned" {
			update.Pinned = &request.Memo.Pinned
		} else if path == "state" {
			rowStatus := convertStateToStore(request.Memo.State)
			update.RowStatus = &rowStatus
		} else if path == "create_time" {
			createdTs := request.Memo.CreateTime.AsTime().Unix()
			update.CreatedTs = &createdTs
		} else if path == "update_time" {
			updatedTs := time.Now().Unix()
			if request.Memo.UpdateTime != nil {
				updatedTs = request.Memo.UpdateTime.AsTime().Unix()
			}
			update.UpdatedTs = &updatedTs
		} else if path == "display_time" {
			displayTs := request.Memo.DisplayTime.AsTime().Unix()
			memoRelatedSetting, err := s.Store.GetWorkspaceMemoRelatedSetting(ctx)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get workspace memo related setting")
			}
			if memoRelatedSetting.DisplayWithUpdateTime {
				update.UpdatedTs = &displayTs
			} else {
				update.CreatedTs = &displayTs
			}
		} else if path == "location" {
			payload := memo.Payload
			payload.Location = convertLocationToStore(request.Memo.Location)
			update.Payload = payload
		} else if path == "resources" {
			_, err := s.SetMemoResources(ctx, &v1pb.SetMemoResourcesRequest{
				Name:      request.Memo.Name,
				Resources: request.Memo.Resources,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to set memo resources")
			}
		} else if path == "relations" {
			_, err := s.SetMemoRelations(ctx, &v1pb.SetMemoRelationsRequest{
				Name:      request.Memo.Name,
				Relations: request.Memo.Relations,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to set memo relations")
			}
		}
	}

	if err = s.Store.UpdateMemo(ctx, update); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update memo")
	}

	memo, err = s.Store.GetMemo(ctx, &store.FindMemo{
		ID: &memo.ID,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get memo")
	}
	memoMessage, err := s.convertMemoFromStore(ctx, memo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert memo")
	}
	// Try to dispatch webhook when memo is updated.
	if err := s.DispatchMemoUpdatedWebhook(ctx, memoMessage); err != nil {
		slog.Warn("Failed to dispatch memo updated webhook", slog.Any("err", err))
	}

	return memoMessage, nil
}

func (s *APIV1Service) DeleteMemo(ctx context.Context, request *v1pb.DeleteMemoRequest) (*emptypb.Empty, error) {
	memoUID, err := ExtractMemoUIDFromName(request.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
	}
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{
		UID: &memoUID,
	})
	if err != nil {
		return nil, err
	}
	if memo == nil {
		return nil, status.Errorf(codes.NotFound, "memo not found")
	}

	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user")
	}
	// Only the creator or admin can update the memo.
	if memo.CreatorID != user.ID && !isSuperUser(user) {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	if memoMessage, err := s.convertMemoFromStore(ctx, memo); err == nil {
		// Try to dispatch webhook when memo is deleted.
		if err := s.DispatchMemoDeletedWebhook(ctx, memoMessage); err != nil {
			slog.Warn("Failed to dispatch memo deleted webhook", slog.Any("err", err))
		}
	}

	if err = s.Store.DeleteMemo(ctx, &store.DeleteMemo{ID: memo.ID}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete memo")
	}

	// Delete memo relation
	if err := s.Store.DeleteMemoRelation(ctx, &store.DeleteMemoRelation{MemoID: &memo.ID}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete memo relations")
	}

	// Delete related resources.
	resources, err := s.Store.ListResources(ctx, &store.FindResource{MemoID: &memo.ID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list resources")
	}
	for _, resource := range resources {
		if err := s.Store.DeleteResource(ctx, &store.DeleteResource{ID: resource.ID}); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to delete resource")
		}
	}

	// Delete memo comments
	commentType := store.MemoRelationComment
	relations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{RelatedMemoID: &memo.ID, Type: &commentType})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list memo comments")
	}
	for _, relation := range relations {
		if err := s.Store.DeleteMemo(ctx, &store.DeleteMemo{ID: relation.MemoID}); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to delete memo comment")
		}
	}

	// Delete memo references
	referenceType := store.MemoRelationReference
	if err := s.Store.DeleteMemoRelation(ctx, &store.DeleteMemoRelation{RelatedMemoID: &memo.ID, Type: &referenceType}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete memo references")
	}

	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) CreateMemoComment(ctx context.Context, request *v1pb.CreateMemoCommentRequest) (*v1pb.Memo, error) {
	memoUID, err := ExtractMemoUIDFromName(request.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
	}
	relatedMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get memo")
	}

	// Create the memo comment first.
	if request.Comment.Visibility == v1pb.Visibility_VISIBILITY_UNSPECIFIED {
		request.Comment.Visibility = v1pb.Visibility_PUBLIC
	}
	memoComment, err := s.CreateMemo(ctx, &v1pb.CreateMemoRequest{Memo: request.Comment})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create memo")
	}
	memoUID, err = ExtractMemoUIDFromName(memoComment.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
	}
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get memo")
	}

	// Build the relation between the comment memo and the original memo.
	_, err = s.Store.UpsertMemoRelation(ctx, &store.MemoRelation{
		MemoID:        memo.ID,
		RelatedMemoID: relatedMemo.ID,
		Type:          store.MemoRelationComment,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create memo relation")
	}
	creatorID, err := ExtractUserIDFromName(memoComment.Creator)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid memo creator")
	}
	if memoComment.Visibility != v1pb.Visibility_PRIVATE && creatorID != relatedMemo.CreatorID {
		activity, err := s.Store.CreateActivity(ctx, &store.Activity{
			CreatorID: creatorID,
			Type:      store.ActivityTypeMemoComment,
			Level:     store.ActivityLevelInfo,
			Payload: &storepb.ActivityPayload{
				MemoComment: &storepb.ActivityMemoCommentPayload{
					MemoId:        memo.ID,
					RelatedMemoId: relatedMemo.ID,
				},
			},
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create activity")
		}
		if _, err := s.Store.CreateInbox(ctx, &store.Inbox{
			SenderID:   creatorID,
			ReceiverID: relatedMemo.CreatorID,
			Status:     store.UNREAD,
			Message: &storepb.InboxMessage{
				Type:       storepb.InboxMessage_MEMO_COMMENT,
				ActivityId: &activity.ID,
			},
		}); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create inbox")
		}
	}

	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &creatorID})
	if err == nil && user != nil && !isSuperUser(user) {
		go s.handleTicketAIResponse(context.Background(), memo.UID, user.ID, request.Comment.Content)
	}

	return memoComment, nil
}

func (s *APIV1Service) ListMemoComments(ctx context.Context, request *v1pb.ListMemoCommentsRequest) (*v1pb.ListMemoCommentsResponse, error) {
	memoUID, err := ExtractMemoUIDFromName(request.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
	}
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get memo")
	}

	currentUser, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user")
	}
	var memoFilter *string
	if currentUser == nil {
		filterStr := `visibility == "PUBLIC"`
		memoFilter = &filterStr
	} else if !isSuperUser(currentUser) {
		filterStr := fmt.Sprintf(`creator_id == %d || visibility in ["PUBLIC", "PROTECTED"]`, currentUser.ID)
		memoFilter = &filterStr
	}
	memoRelationComment := store.MemoRelationComment
	memoRelations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
		RelatedMemoID: &memo.ID,
		Type:          &memoRelationComment,
		MemoFilter:    memoFilter,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list memo relations")
	}

	var memos []*v1pb.Memo
	for _, memoRelation := range memoRelations {
		memo, err := s.Store.GetMemo(ctx, &store.FindMemo{
			ID: &memoRelation.MemoID,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get memo")
		}
		if memo != nil {
			memoMessage, err := s.convertMemoFromStore(ctx, memo)
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert memo")
			}
			memos = append(memos, memoMessage)
		}
	}

	response := &v1pb.ListMemoCommentsResponse{
		Memos: memos,
	}
	return response, nil
}

func (s *APIV1Service) RenameMemoTag(ctx context.Context, request *v1pb.RenameMemoTagRequest) (*emptypb.Empty, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user")
	}

	memoFind := &store.FindMemo{
		CreatorID:       &user.ID,
		PayloadFind:     &store.FindMemoPayload{TagSearch: []string{request.OldTag}},
		ExcludeComments: true,
	}
	if (request.Parent) != "memos/-" {
		memoUID, err := ExtractMemoUIDFromName(request.Parent)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
		}
		memoFind.UID = &memoUID
	}

	memos, err := s.Store.ListMemos(ctx, memoFind)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list memos")
	}

	for _, memo := range memos {
		nodes, err := parser.Parse(tokenizer.Tokenize(memo.Content))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse memo: %v", err)
		}
		memopayload.TraverseASTNodes(nodes, func(node ast.Node) {
			if tag, ok := node.(*ast.Tag); ok && tag.Content == request.OldTag {
				tag.Content = request.NewTag
			}
		})
		memo.Content = restore.Restore(nodes)
		if err := memopayload.RebuildMemoPayload(memo); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to rebuild memo payload: %v", err)
		}
		if err := s.Store.UpdateMemo(ctx, &store.UpdateMemo{
			ID:      memo.ID,
			Content: &memo.Content,
			Payload: memo.Payload,
		}); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update memo: %v", err)
		}
	}

	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) DeleteMemoTag(ctx context.Context, request *v1pb.DeleteMemoTagRequest) (*emptypb.Empty, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user")
	}

	memoFind := &store.FindMemo{
		CreatorID:       &user.ID,
		PayloadFind:     &store.FindMemoPayload{TagSearch: []string{request.Tag}},
		ExcludeContent:  true,
		ExcludeComments: true,
	}
	if request.Parent != "memos/-" {
		memoUID, err := ExtractMemoUIDFromName(request.Parent)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
		}
		memoFind.UID = &memoUID
	}

	memos, err := s.Store.ListMemos(ctx, memoFind)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list memos")
	}

	for _, memo := range memos {
		if request.DeleteRelatedMemos {
			err := s.Store.DeleteMemo(ctx, &store.DeleteMemo{ID: memo.ID})
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to delete memo")
			}
		} else {
			archived := store.Archived
			err := s.Store.UpdateMemo(ctx, &store.UpdateMemo{
				ID:        memo.ID,
				RowStatus: &archived,
			})
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to update memo")
			}
		}
	}

	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) getContentLengthLimit(ctx context.Context) (int, error) {
	workspaceMemoRelatedSetting, err := s.Store.GetWorkspaceMemoRelatedSetting(ctx)
	if err != nil {
		return 0, status.Errorf(codes.Internal, "failed to get workspace memo related setting")
	}
	return int(workspaceMemoRelatedSetting.ContentLengthLimit), nil
}

// DispatchMemoCreatedWebhook dispatches webhook when memo is created.
func (s *APIV1Service) DispatchMemoCreatedWebhook(ctx context.Context, memo *v1pb.Memo) error {
	return s.dispatchMemoRelatedWebhook(ctx, memo, "memos.memo.created")
}

// DispatchMemoUpdatedWebhook dispatches webhook when memo is updated.
func (s *APIV1Service) DispatchMemoUpdatedWebhook(ctx context.Context, memo *v1pb.Memo) error {
	return s.dispatchMemoRelatedWebhook(ctx, memo, "memos.memo.updated")
}

// DispatchMemoDeletedWebhook dispatches webhook when memo is deleted.
func (s *APIV1Service) DispatchMemoDeletedWebhook(ctx context.Context, memo *v1pb.Memo) error {
	return s.dispatchMemoRelatedWebhook(ctx, memo, "memos.memo.deleted")
}

func (s *APIV1Service) dispatchMemoRelatedWebhook(ctx context.Context, memo *v1pb.Memo, activityType string) error {
	creatorID, err := ExtractUserIDFromName(memo.Creator)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid memo creator")
	}
	webhooks, err := s.Store.ListWebhooks(ctx, &store.FindWebhook{
		CreatorID: &creatorID,
	})
	if err != nil {
		return err
	}
	for _, hook := range webhooks {
		payload, err := convertMemoToWebhookPayload(memo)
		if err != nil {
			return errors.Wrap(err, "failed to convert memo to webhook payload")
		}
		payload.ActivityType = activityType
		payload.Url = hook.URL

		// Use asynchronous webhook dispatch
		webhook.PostAsync(payload)
	}
	return nil
}

func (s *APIV1Service) dispatchMemoMentions(ctx context.Context, memo *store.Memo) error {
	// Analyze mentions in the memo content
	// Regexp to find all @nickname
	// Matches @nickname followed by space or end of line
	// Nickname can contain alphanumeric, underscore, dot, dash.
	usernameRegexp := regexp.MustCompile(`@([a-zA-Z0-9_.-]+)`)
	matches := usernameRegexp.FindAllStringSubmatch(memo.Content, -1)

	mentionedUserIDs := make(map[int32]bool)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		nickname := match[1]
		user, err := s.Store.GetUser(ctx, &store.FindUser{
			Username: &nickname,
		})
		if err != nil {
			// Ignore if user not found or error
			continue
		}
		if user == nil {
			// Fallback to nickname
			user, err = s.Store.GetUser(ctx, &store.FindUser{
				Nickname: &nickname,
			})
			if err != nil {
				continue
			}
		}
		if user != nil {
			mentionedUserIDs[user.ID] = true
		}
	}

	// For comments, we want the "RelatedMemo" in the notification to point to the Parent Memo (the Ticket).
	// This ensures clicking the notification takes the user to the Ticket View, not the isolated comment view.
	relatedMemoID := memo.ID
	// Check if this memo is a comment (has a parent relation)
	relationType := store.MemoRelationComment
	relations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
		MemoID: &memo.ID,
		Type:   &relationType,
	})
	if err == nil && len(relations) > 0 {
		// It is a comment, point to the parent.
		// relationships in store/memo_relation.go are usually RelatedMemoID (int32)
		if relations[0].RelatedMemoID != 0 {
			relatedMemoID = relations[0].RelatedMemoID
		}
	}

	for userID := range mentionedUserIDs {
		// Don't notify self
		if userID == memo.CreatorID {
			continue
		}

		// Create Activity
		activityPayload := &storepb.ActivityPayload{
			MemoComment: &storepb.ActivityMemoCommentPayload{
				MemoId:        memo.ID,
				RelatedMemoId: relatedMemoID,
			},
		}
		activity, err := s.Store.CreateActivity(ctx, &store.Activity{
			CreatorID: memo.CreatorID,
			Type:      store.ActivityTypeMemoComment,
			Level:     store.ActivityLevelInfo,
			Payload:   activityPayload,
		})
		if err != nil {
			return errors.Wrap(err, "failed to create activity")
		}

		// Create Inbox for the mentioned user
		// Find the correct ticket ID to use in the notification URL
		ticketURL := "/m/" + memo.UID // Default fallback to memo URL

		// If this is a comment, we try to find the ticket associated with the parent memo
		parentMemo, _ := s.Store.GetMemo(ctx, &store.FindMemo{ID: &relatedMemoID})
		if parentMemo != nil {
			descriptionLink := "/m/" + parentMemo.UID
			tickets, _ := s.Store.ListTickets(ctx, &store.FindTicket{Description: &descriptionLink})
			if len(tickets) > 0 {
				ticketURL = "/tickets/" + strconv.Itoa(int(tickets[0].ID))
			}
		}

		// Create Notification
		_, err = s.Store.CreateNotification(ctx, &store.Notification{
			InitiatorID: memo.CreatorID,
			ReceiverID:  userID,
			TicketURL:   ticketURL,
			CreatedTs:   time.Now().Unix(),
			IsRead:      false,
		})
		if err != nil {
			return errors.Wrap(err, "failed to create notification")
		}

		// Push real-time notification via SSE
		senderUser, _ := s.Store.GetUser(ctx, &store.FindUser{ID: &memo.CreatorID})
		senderName := "Someone"
		if senderUser != nil {
			if senderUser.Nickname != "" {
				senderName = senderUser.Nickname
			} else {
				senderName = senderUser.Username
			}
		}

		fmt.Printf("SSE: Preparing to notify userID: %d about memo mention\n", userID)
		GetNotificationHub().NotifyUser(userID, Notification{
			ID:         activity.ID,
			Type:       "MEMO_COMMENT",
			SenderName: senderName,
			SenderID:   memo.CreatorID,
			MemoName:   fmt.Sprintf("memos/%s", memo.UID),
			Timestamp:  time.Now(),
		})
	}

	return nil
}

func convertMemoToWebhookPayload(memo *v1pb.Memo) (*v1pb.WebhookRequestPayload, error) {
	creatorID, err := ExtractUserIDFromName(memo.Creator)
	if err != nil {
		return nil, errors.Wrap(err, "invalid memo creator")
	}
	return &v1pb.WebhookRequestPayload{
		Creator:    fmt.Sprintf("%s%d", UserNamePrefix, creatorID),
		CreateTime: timestamppb.New(time.Now()),
		Memo:       memo,
	}, nil
}

func getMemoContentSnippet(content string) (string, error) {
	nodes, err := parser.Parse(tokenizer.Tokenize(content))
	if err != nil {
		return "", errors.Wrap(err, "failed to parse content")
	}

	plainText := renderer.NewStringRenderer().Render(nodes)
	if len(plainText) > 64 {
		return substring(plainText, 64) + "...", nil
	}
	return plainText, nil
}

func substring(s string, length int) string {
	if length <= 0 {
		return ""
	}

	runeCount := 0
	byteIndex := 0
	for byteIndex < len(s) {
		_, size := utf8.DecodeRuneInString(s[byteIndex:])
		byteIndex += size
		runeCount++
		if runeCount == length {
			break
		}
	}

	return s[:byteIndex]
}

func (s *APIV1Service) handleAutoTicketCreation(ctx context.Context, memo *store.Memo, user *store.User) bool {
	// Extract title
	title := memo.Content
	if idx := strings.Index(title, "\n"); idx > 0 {
		title = title[:idx]
	}
	title = strings.TrimSpace(strings.TrimLeft(title, "#* \t"))
	if len(title) > 80 {
		title = title[:80] + "..."
	}
	if title == "" {
		title = "Support Request"
	}

	// Parse tags in content to set Priority and Type
	priority := store.TicketPriorityMedium
	ticketType := "SUPPORT"
	contentLower := strings.ToLower(memo.Content)

	if strings.Contains(contentLower, "#high") || strings.Contains(contentLower, "#urgent") {
		priority = store.TicketPriorityHigh
	} else if strings.Contains(contentLower, "#low") {
		priority = store.TicketPriorityLow
	}

	if strings.Contains(contentLower, "#bug") {
		ticketType = "BUG"
	} else if strings.Contains(contentLower, "#feature") {
		ticketType = "FEATURE"
	}

	tags := []string{}
	isEscalated := false
	// If user tagged `#staff` or `#human` or `#escalated`, mark ticket as escalated from start!
	if strings.Contains(contentLower, "#staff") || strings.Contains(contentLower, "#human") || strings.Contains(contentLower, "#escalated") {
		tags = append(tags, "escalated")
		priority = store.TicketPriorityHigh // Auto escalate is high priority
		isEscalated = true
	}

	ticket := &store.Ticket{
		Title:       title,
		Description: "/m/" + memo.UID,
		Status:      store.TicketStatusOpen,
		Priority:    priority,
		Type:        ticketType,
		Tags:        tags,
		CreatorID:   user.ID,
		CreatedTs:   time.Now().Unix(),
		UpdatedTs:   time.Now().Unix(),
	}

	_, err := s.Store.CreateTicket(ctx, ticket)
	if err != nil {
		slog.Error("failed to create automatic support ticket for memo", "memoUID", memo.UID, "error", err)
		return isEscalated
	}

	slog.Info("Successfully created automatic support ticket for customer memo", "memoUID", memo.UID, "ticket_title", title, "priority", priority, "type", ticketType)
	return isEscalated
}

func (s *APIV1Service) handleTicketAIResponse(ctx context.Context, memoUID string, creatorID int32, latestMessageContent string) {
	// 1. Sleep slightly (e.g. 500ms) to ensure DB commits/relations are finished
	time.Sleep(500 * time.Millisecond)

	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &creatorID})
	if err != nil || user == nil {
		return
	}

	// 2. Fetch the target memo
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil || memo == nil {
		return
	}

	// 3. Find the parent memo (if this is a comment)
	parentMemo := memo
	commentType := store.MemoRelationComment
	relations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
		MemoID: &memo.ID,
		Type:   &commentType,
	})
	if err == nil && len(relations) > 0 {
		// It is a comment, load parent memo
		pID := relations[0].RelatedMemoID
		pMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: &pID})
		if err == nil && pMemo != nil {
			parentMemo = pMemo
		}
	}

	// 4. Find the ticket linked to the parent memo
	descriptionLink := "/m/" + parentMemo.UID
	tickets, err := s.Store.ListTickets(ctx, &store.FindTicket{Description: &descriptionLink})
	if err != nil || len(tickets) == 0 {
		slog.Warn("AI support: linked ticket not found", "parentMemoUID", parentMemo.UID)
		return
	}
	ticket := tickets[0]

	// 5. Check if the ticket is escalated to human staff (if so, skip AI)
	for _, tag := range ticket.Tags {
		if tag == "escalated" {
			slog.Info("AI support: ticket is escalated to human, skipping auto-reply", "ticketID", ticket.ID)
			return
		}
	}

	// 6. Find the customer's TenantID
	perms, err := s.Store.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{UserID: &ticket.CreatorID})
	var tenantID int32
	if err != nil || len(perms) == 0 {
		// Default fallback: load first active tenant in system
		tenants, err := s.Store.ListAgentTenants(ctx, &store.FindAgentTenant{})
		if err == nil && len(tenants) > 0 {
			tenantID = tenants[0].ID
		} else {
			slog.Error("AI support: no active tenants found in the system")
			return
		}
	} else {
		tenantID = perms[0].TenantID
	}

	tenant, err := s.Store.GetAgentTenant(ctx, &store.FindAgentTenant{ID: &tenantID})
	if err != nil || tenant == nil {
		slog.Error("AI support: failed to get tenant", "tenantID", tenantID)
		return
	}

	// 7. Build conversation history from parent memo + all comments (excluding latest)
	history := []store.AgentMessage{}

	// Root message
	rootCreator, _ := s.Store.GetUser(ctx, &store.FindUser{ID: &parentMemo.CreatorID})
	rootRole := "user"
	if rootCreator != nil && isSuperUser(rootCreator) {
		rootRole = "assistant"
	} else if parentMemo.CreatorID == store.SystemBotID {
		rootRole = "assistant"
	}
	// Only add root to history if it's not the latest message itself
	if parentMemo.UID != memoUID {
		history = append(history, store.AgentMessage{
			Role:      rootRole,
			Content:   parentMemo.Content,
			Timestamp: time.Unix(parentMemo.CreatedTs, 0),
		})
	}

	// Get comments
	commentsRelations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
		RelatedMemoID: &parentMemo.ID,
		Type:          &commentType,
	})
	if err == nil {
		type commentWithTime struct {
			content   string
			creatorID int32
			createdTs int64
			uid       string
		}
		var list []commentWithTime
		for _, r := range commentsRelations {
			cMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: &r.MemoID})
			if err == nil && cMemo != nil {
				list = append(list, commentWithTime{
					content:   cMemo.Content,
					creatorID: cMemo.CreatorID,
					createdTs: cMemo.CreatedTs,
					uid:       cMemo.UID,
				})
			}
		}
		// Sort chronologically by createdTs
		sort.Slice(list, func(i, j int) bool {
			return list[i].createdTs < list[j].createdTs
		})

		for _, c := range list {
			// Skip the latest message itself since it's passed separately
			if c.uid == memoUID {
				continue
			}
			cCreator, _ := s.Store.GetUser(ctx, &store.FindUser{ID: &c.creatorID})
			role := "user"
			if cCreator != nil && isSuperUser(cCreator) {
				role = "assistant"
			} else if c.creatorID == store.SystemBotID {
				role = "assistant"
			}
			history = append(history, store.AgentMessage{
				Role:      role,
				Content:   c.content,
				Timestamp: time.Unix(c.createdTs, 0),
			})
		}
	}

	// 8. Generate AI response
	slog.Info("AI support: generating response", "tenant", tenant.Slug, "ticketID", ticket.ID)
	aiReply, err := s.agentHandler.GetService().ProcessTicketChat(ctx, tenant.Slug, history, latestMessageContent)
	if err != nil {
		slog.Error("AI support: failed to process chat", "error", err)
		return
	}

	// 9. Post AI reply comment
	aiMemo := &store.Memo{
		UID:        shortuuid.New(),
		CreatorID:  store.SystemBotID,
		Content:    aiReply,
		Visibility: store.Protected,
	}
	if err := memopayload.RebuildMemoPayload(aiMemo); err != nil {
		slog.Error("AI support: failed to rebuild payload", "error", err)
		return
	}

	createdReply, err := s.Store.CreateMemo(ctx, aiMemo)
	if err != nil {
		slog.Error("AI support: failed to save reply memo", "error", err)
		return
	}

	// Link AI comment to parent
	_, err = s.Store.UpsertMemoRelation(ctx, &store.MemoRelation{
		MemoID:        createdReply.ID,
		RelatedMemoID: parentMemo.ID,
		Type:          store.MemoRelationComment,
	})
	if err != nil {
		slog.Error("AI support: failed to create comment relation", "error", err)
		return
	}

	slog.Info("AI support: auto-replied successfully to ticket", "ticketID", ticket.ID, "replyUID", createdReply.UID)
}
