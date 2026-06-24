package v1

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/usememos/memos/plugin/storage/s3"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

func (s *APIV1Service) SetMemoResources(ctx context.Context, request *v1pb.SetMemoResourcesRequest) (*emptypb.Empty, error) {
	memoUID, err := ExtractMemoUIDFromName(request.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
	}
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get memo")
	}
	if memo == nil {
		return nil, status.Errorf(codes.NotFound, "memo not found")
	}

	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthorized access")
	}

	// Verify write permission to the target memo
	if err := s.checkMemoWriteAccess(ctx, memo); err != nil {
		return nil, err
	}

	// Identify SQL dialect / driver
	driver := "sqlite"
	if s.Profile != nil && s.Profile.Driver != "" {
		driver = s.Profile.Driver
	}

	// Start database transaction
	sqlDB := s.Store.GetDriver().GetDB()
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Lock the target memo row to prevent concurrent deletion or modification without mutating user-visible metadata
	lockQuery := replacePlaceholders(driver, "UPDATE memo SET row_status = row_status WHERE id = ? AND row_status = 'NORMAL'")
	res, err := tx.ExecContext(ctx, lockQuery, memo.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to lock memo: %v", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check locked memo: %v", err)
	}
	if rowsAffected == 0 {
		return nil, status.Errorf(codes.NotFound, "memo not found or already deleted")
	}

	// Get resources currently linked to the memo inside the transaction
	selectQuery := replacePlaceholders(driver, "SELECT id, uid, creator_id, memo_id, storage_type, reference, payload FROM resource WHERE memo_id = ?")
	rows, err := tx.QueryContext(ctx, selectQuery, memo.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query existing resources: %v", err)
	}
	defer rows.Close()

	var existingResources []*store.Resource
	for rows.Next() {
		var r store.Resource
		var rMemoID int32
		var storageType string
		var reference string
		var payloadBytes []byte
		if err := rows.Scan(&r.ID, &r.UID, &r.CreatorID, &rMemoID, &storageType, &reference, &payloadBytes); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to scan resource: %v", err)
		}
		r.MemoID = &rMemoID
		r.StorageType = storepb.ResourceStorageType(storepb.ResourceStorageType_value[storageType])
		r.Reference = reference
		payload := &storepb.ResourcePayload{}
		if err := protojson.Unmarshal(payloadBytes, payload); err == nil {
			r.Payload = payload
		}
		existingResources = append(existingResources, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, status.Errorf(codes.Internal, "error iterating resources: %v", err)
	}

	// Identify resources to delete and validate deletion permission
	var resourcesToDelete []*store.Resource
	for _, resource := range existingResources {
		found := false
		for _, requestResource := range request.Resources {
			requestResourceUID, err := ExtractResourceUIDFromName(requestResource.Name)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid resource name: %v", err)
			}
			if resource.UID == requestResourceUID {
				found = true
				break
			}
		}
		if !found {
			resourcesToDelete = append(resourcesToDelete, resource)
		}
	}

	for _, resource := range resourcesToDelete {
		if err := s.checkResourceAccess(ctx, resource, ActionDelete); err != nil {
			return nil, err
		}
	}

	// Identify incoming resources and validate write/edit permission
	type resourceWithUID struct {
		Resource    *store.Resource
		ResourceUID string
		OrigMemoID  *int32
	}
	var incomingResources []resourceWithUID
	for _, requestResource := range request.Resources {
		resourceUID, err := ExtractResourceUIDFromName(requestResource.Name)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid resource name: %v", err)
		}

		// Fetch current resource state within transaction
		var resID int32
		var resCreatorID int32
		var resMemoID sql.NullInt32
		fetchQuery := replacePlaceholders(driver, "SELECT id, creator_id, memo_id FROM resource WHERE uid = ?")
		err = tx.QueryRowContext(ctx, fetchQuery, resourceUID).Scan(&resID, &resCreatorID, &resMemoID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, status.Errorf(codes.NotFound, "resource not found")
			}
			return nil, status.Errorf(codes.Internal, "failed to get resource: %v", err)
		}

		tempResource := &store.Resource{
			ID:        resID,
			UID:       resourceUID,
			CreatorID: resCreatorID,
		}
		var origMemoID *int32
		if resMemoID.Valid {
			origMemoID = &resMemoID.Int32
			tempResource.MemoID = origMemoID
		}

		// Only check ActionWrite if the resource is not already linked to the target memo
		if origMemoID == nil || *origMemoID != memo.ID {
			if err := s.checkResourceAccess(ctx, tempResource, ActionWrite); err != nil {
				return nil, err
			}
		}

		incomingResources = append(incomingResources, resourceWithUID{
			Resource:    tempResource,
			ResourceUID: resourceUID,
			OrigMemoID:  origMemoID,
		})
	}

	// Perform deletions inside transaction, conditioned on both ID and expected memo ID
	deleteQuery := replacePlaceholders(driver, "DELETE FROM resource WHERE id = ? AND memo_id = ?")
	for _, resource := range resourcesToDelete {
		resDel, err := tx.ExecContext(ctx, deleteQuery, resource.ID, memo.ID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to delete resource: %v", err)
		}
		delRows, err := resDel.RowsAffected()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to check rows affected: %v", err)
		}
		if delRows == 0 {
			return nil, status.Errorf(codes.Aborted, "concurrent modification: resource already reassigned or deleted")
		}
	}

	// Perform updates in reverse order
	slices.Reverse(incomingResources)
	for index, item := range incomingResources {
		updatedTs := time.Now().Unix() + int64(index)
		var queryStr string
		if driver == "mysql" {
			if item.OrigMemoID != nil {
				queryStr = "UPDATE resource SET memo_id = ?, updated_ts = FROM_UNIXTIME(?) WHERE id = ? AND memo_id = ?"
			} else {
				queryStr = "UPDATE resource SET memo_id = ?, updated_ts = FROM_UNIXTIME(?) WHERE id = ? AND memo_id IS NULL"
			}
		} else {
			if item.OrigMemoID != nil {
				queryStr = "UPDATE resource SET memo_id = ?, updated_ts = ? WHERE id = ? AND memo_id = ?"
			} else {
				queryStr = "UPDATE resource SET memo_id = ?, updated_ts = ? WHERE id = ? AND memo_id IS NULL"
			}
		}

		updateQuery := replacePlaceholders(driver, queryStr)
		var resUpd sql.Result
		if item.OrigMemoID != nil {
			resUpd, err = tx.ExecContext(ctx, updateQuery, memo.ID, updatedTs, item.Resource.ID, *item.OrigMemoID)
		} else {
			resUpd, err = tx.ExecContext(ctx, updateQuery, memo.ID, updatedTs, item.Resource.ID)
		}
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update resource: %v", err)
		}
		updRows, err := resUpd.RowsAffected()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to check rows affected: %v", err)
		}
		if updRows == 0 {
			return nil, status.Errorf(codes.Aborted, "concurrent modification: resource already reassigned or deleted")
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to commit transaction: %v", err)
	}

	// After transaction commits successfully, clean up backing files/S3 objects to avoid storage leaks
	for _, resource := range resourcesToDelete {
		s.deleteBackingResourceFile(ctx, resource)
	}

	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) deleteBackingResourceFile(ctx context.Context, resource *store.Resource) {
	if resource.StorageType == storepb.ResourceStorageType_LOCAL {
		p := filepath.FromSlash(resource.Reference)
		if !filepath.IsAbs(p) {
			if s.Profile != nil {
				p = filepath.Join(s.Profile.Data, p)
			}
		}
		if err := os.Remove(p); err != nil {
			slog.Warn("Failed to delete local file", slog.String("path", p), slog.Any("error", err))
		}
	} else if resource.StorageType == storepb.ResourceStorageType_S3 {
		s3ObjectPayload := resource.Payload.GetS3Object()
		if s3ObjectPayload != nil {
			workspaceStorageSetting, err := s.Store.GetWorkspaceStorageSetting(ctx)
			if err == nil {
				s3Config := s3ObjectPayload.S3Config
				if s3Config == nil {
					s3Config = workspaceStorageSetting.S3Config
				}
				if s3Config != nil {
					s3Client, err := s3.NewClient(ctx, s3Config)
					if err == nil {
						if err := s3Client.DeleteObject(ctx, s3ObjectPayload.Key); err != nil {
							slog.Warn("Failed to delete s3 object", slog.String("key", s3ObjectPayload.Key), slog.Any("error", err))
						}
					}
				}
			}
		}
	}
}

func (s *APIV1Service) ListMemoResources(ctx context.Context, request *v1pb.ListMemoResourcesRequest) (*v1pb.ListMemoResourcesResponse, error) {
	memoUID, err := ExtractMemoUIDFromName(request.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
	}
	memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get memo: %v", err)
	}
	if memo == nil {
		return nil, status.Errorf(codes.NotFound, "memo not found")
	}

	// Validate read visibility to parent memo
	if err := s.checkMemoReadAccess(ctx, memo); err != nil {
		return nil, err
	}

	resources, err := s.Store.ListResources(ctx, &store.FindResource{
		MemoID: &memo.ID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list resources: %v", err)
	}

	response := &v1pb.ListMemoResourcesResponse{
		Resources: []*v1pb.Resource{},
	}
	for _, resource := range resources {
		response.Resources = append(response.Resources, s.convertResourceFromStore(ctx, resource))
	}
	return response, nil
}

func replacePlaceholders(driver string, query string) string {
	if driver != "postgres" {
		return query
	}
	var builder strings.Builder
	paramIndex := 1
	for _, r := range query {
		if r == '?' {
			builder.WriteString(fmt.Sprintf("$%d", paramIndex))
			paramIndex++
		} else {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
