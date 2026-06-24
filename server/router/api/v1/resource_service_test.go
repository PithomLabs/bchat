package v1

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
	teststore "github.com/usememos/memos/store/test"
)

func setupResourceTestStore(t *testing.T, ctx context.Context) (*store.Store, *APIV1Service, map[string]*store.User) {
	db := teststore.NewTestingStore(ctx, t)

	s := &APIV1Service{
		Store:  db,
		Secret: "test-secret",
	}

	users := make(map[string]*store.User)

	// Create users
	roles := map[string]store.Role{
		"host":  store.RoleHost,
		"admin": store.RoleAdmin,
		"userA": store.RoleUser,
		"userB": store.RoleUser,
	}

	for name, role := range roles {
		user, err := db.CreateUser(ctx, &store.User{
			Username:     name,
			Nickname:     name,
			Role:         role,
			PasswordHash: "hash",
		})
		require.NoError(t, err)
		users[name] = user
	}

	return db, s, users
}

func ctxFor(name string) context.Context {
	if name == "" {
		return context.Background()
	}
	return context.WithValue(context.Background(), usernameContextKey, name)
}

func TestResourceReadAccessPublicRoot(t *testing.T) {
	ctx := context.Background()
	db, s, users := setupResourceTestStore(t, ctx)
	defer db.Close()

	// 1. Create a public memo
	memo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "memo-public",
		CreatorID:  users["userA"].ID,
		Content:    "Public content",
		Visibility: store.Public,
	})
	require.NoError(t, err)

	// 2. Create a resource attached to the public memo (created by userA)
	res, err := db.CreateResource(ctx, &store.Resource{
		UID:       "res-public",
		CreatorID: users["userA"].ID,
		Filename:  "file.png",
		Type:      "image/png",
		MemoID:    &memo.ID,
	})
	require.NoError(t, err)

	// 3. Anyone (anonymous, userB, host, admin) should be able to read it
	for _, client := range []string{"", "userB", "host", "admin"} {
		err := s.checkResourceAccess(ctxFor(client), res, ActionRead)
		require.NoError(t, err, "client: %s", client)
	}
}

func TestResourceReadAccessProtectedRoot(t *testing.T) {
	ctx := context.Background()
	db, s, users := setupResourceTestStore(t, ctx)
	defer db.Close()

	memo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "memo-protected",
		CreatorID:  users["userA"].ID,
		Content:    "Protected content",
		Visibility: store.Protected,
	})
	require.NoError(t, err)

	res, err := db.CreateResource(ctx, &store.Resource{
		UID:       "res-protected",
		CreatorID: users["userA"].ID,
		Filename:  "file.png",
		Type:      "image/png",
		MemoID:    &memo.ID,
	})
	require.NoError(t, err)

	// Anonymous should be denied with Unauthenticated
	err = s.checkResourceAccess(ctxFor(""), res, ActionRead)
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	// Authenticated users (userB, host, admin) should be allowed
	for _, client := range []string{"userB", "host", "admin"} {
		err := s.checkResourceAccess(ctxFor(client), res, ActionRead)
		require.NoError(t, err, "client: %s", client)
	}
}

func TestResourceReadAccessPrivateRoot(t *testing.T) {
	ctx := context.Background()
	db, s, users := setupResourceTestStore(t, ctx)
	defer db.Close()

	memo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "memo-private",
		CreatorID:  users["userA"].ID,
		Content:    "Private content",
		Visibility: store.Private,
	})
	require.NoError(t, err)

	res, err := db.CreateResource(ctx, &store.Resource{
		UID:       "res-private",
		CreatorID: users["userA"].ID,
		Filename:  "file.png",
		Type:      "image/png",
		MemoID:    &memo.ID,
	})
	require.NoError(t, err)

	// Anonymous should be Unauthenticated
	err = s.checkResourceAccess(ctxFor(""), res, ActionRead)
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	// Unrelated userB should be PermissionDenied
	err = s.checkResourceAccess(ctxFor("userB"), res, ActionRead)
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	// Owner (userA), Host, and Admin should be allowed
	for _, client := range []string{"userA", "host", "admin"} {
		err := s.checkResourceAccess(ctxFor(client), res, ActionRead)
		require.NoError(t, err, "client: %s", client)
	}
}

func TestResourceReadAccessCommentHierarchy(t *testing.T) {
	ctx := context.Background()
	db, s, users := setupResourceTestStore(t, ctx)
	defer db.Close()

	// 1. Root private ticket created by userA (customer)
	rootMemo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "root-ticket",
		CreatorID:  users["userA"].ID,
		Content:    "Ticket content",
		Visibility: store.Private,
	})
	require.NoError(t, err)

	// 2. Comment created by agent (host)
	commentMemo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "comment-1",
		CreatorID:  users["host"].ID,
		Content:    "Comment response",
		Visibility: store.Private,
	})
	require.NoError(t, err)

	_, err = db.UpsertMemoRelation(ctx, &store.MemoRelation{
		MemoID:        commentMemo.ID,
		RelatedMemoID: rootMemo.ID,
		Type:          store.MemoRelationComment,
	})
	require.NoError(t, err)

	// Refetch to populate ParentIDs
	commentMemo, err = db.GetMemo(ctx, &store.FindMemo{ID: &commentMemo.ID})
	require.NoError(t, err)
	require.NotNil(t, commentMemo.ParentID)
	require.Equal(t, rootMemo.ID, *commentMemo.ParentID)

	// 3. Create a resource attached to the comment memo (created by host)
	res, err := db.CreateResource(ctx, &store.Resource{
		UID:       "res-comment",
		CreatorID: users["host"].ID,
		Filename:  "reply.pdf",
		Type:      "application/pdf",
		MemoID:    &commentMemo.ID,
	})
	require.NoError(t, err)

	// Ticket Owner (userA) should be able to read it
	err = s.checkResourceAccess(ctxFor("userA"), res, ActionRead)
	require.NoError(t, err)

	// Commenter (host) should be able to read it
	err = s.checkResourceAccess(ctxFor("host"), res, ActionRead)
	require.NoError(t, err)

	// Unrelated userB should be denied
	err = s.checkResourceAccess(ctxFor("userB"), res, ActionRead)
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestResourceReadAccessUnattached(t *testing.T) {
	ctx := context.Background()
	db, s, users := setupResourceTestStore(t, ctx)
	defer db.Close()

	res, err := db.CreateResource(ctx, &store.Resource{
		UID:       "res-unattached",
		CreatorID: users["userA"].ID,
		Filename:  "unattached.zip",
		Type:      "application/zip",
		MemoID:    nil,
	})
	require.NoError(t, err)

	// Creator (userA), Host, and Admin should be allowed
	for _, client := range []string{"userA", "host", "admin"} {
		err := s.checkResourceAccess(ctxFor(client), res, ActionRead)
		require.NoError(t, err, "client: %s", client)
	}

	// Anonymous should be Unauthenticated
	err = s.checkResourceAccess(ctxFor(""), res, ActionRead)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	// Unrelated userB should be PermissionDenied
	err = s.checkResourceAccess(ctxFor("userB"), res, ActionRead)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestResourceWriteAccessRestrictions(t *testing.T) {
	ctx := context.Background()
	db, s, users := setupResourceTestStore(t, ctx)
	defer db.Close()

	res, err := db.CreateResource(ctx, &store.Resource{
		UID:       "res-write",
		CreatorID: users["userA"].ID,
		Filename:  "file.png",
		Type:      "image/png",
	})
	require.NoError(t, err)

	// Creator (userA) allowed
	err = s.checkResourceAccess(ctxFor("userA"), res, ActionWrite)
	require.NoError(t, err)

	// Host, Admin, unrelated userB, anonymous should be Denied
	for _, client := range []string{"host", "admin", "userB"} {
		err = s.checkResourceAccess(ctxFor(client), res, ActionWrite)
		require.Equal(t, codes.PermissionDenied, status.Code(err), "client: %s", client)
	}

	err = s.checkResourceAccess(ctxFor(""), res, ActionWrite)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestResourceDeleteAccessRestrictions(t *testing.T) {
	ctx := context.Background()
	db, s, users := setupResourceTestStore(t, ctx)
	defer db.Close()

	res, err := db.CreateResource(ctx, &store.Resource{
		UID:       "res-delete",
		CreatorID: users["userA"].ID,
		Filename:  "file.png",
		Type:      "image/png",
	})
	require.NoError(t, err)

	// Creator (userA) allowed
	err = s.checkResourceAccess(ctxFor("userA"), res, ActionDelete)
	require.NoError(t, err)

	// Host, Admin, unrelated userB, anonymous should be Denied
	for _, client := range []string{"host", "admin", "userB"} {
		err = s.checkResourceAccess(ctxFor(client), res, ActionDelete)
		require.Equal(t, codes.PermissionDenied, status.Code(err), "client: %s", client)
	}

	err = s.checkResourceAccess(ctxFor(""), res, ActionDelete)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestListResourcesAdminAccess(t *testing.T) {
	ctx := context.Background()
	db, s, users := setupResourceTestStore(t, ctx)
	defer db.Close()

	_, err := db.CreateResource(ctx, &store.Resource{
		UID:       "res-a",
		CreatorID: users["userA"].ID,
		Filename:  "a.png",
		Type:      "image/png",
	})
	require.NoError(t, err)

	_, err = db.CreateResource(ctx, &store.Resource{
		UID:       "res-b",
		CreatorID: users["userB"].ID,
		Filename:  "b.png",
		Type:      "image/png",
	})
	require.NoError(t, err)

	// Host listing should return 2 resources
	respHost, err := s.ListResources(ctxFor("host"), &v1pb.ListResourcesRequest{})
	require.NoError(t, err)
	require.Len(t, respHost.Resources, 2)

	// Admin listing should return 2 resources
	respAdmin, err := s.ListResources(ctxFor("admin"), &v1pb.ListResourcesRequest{})
	require.NoError(t, err)
	require.Len(t, respAdmin.Resources, 2)

	// userA listing should return 1 resource
	respA, err := s.ListResources(ctxFor("userA"), &v1pb.ListResourcesRequest{})
	require.NoError(t, err)
	require.Len(t, respA.Resources, 1)
}

func TestSetMemoResourcesAtomicRejection(t *testing.T) {
	ctx := context.Background()
	db, s, users := setupResourceTestStore(t, ctx)
	defer db.Close()

	// Create a private memo owned by userA
	memo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "memo-atomic",
		CreatorID:  users["userA"].ID,
		Content:    "Memo content",
		Visibility: store.Private,
	})
	require.NoError(t, err)

	_, err = db.CreateResource(ctx, &store.Resource{
		UID:       "res-a",
		CreatorID: users["userA"].ID,
		Filename:  "a.png",
		Type:      "image/png",
		MemoID:    &memo.ID,
	})
	require.NoError(t, err)

	_, err = db.CreateResource(ctx, &store.Resource{
		UID:       "res-b",
		CreatorID: users["userB"].ID,
		Filename:  "b.png",
		Type:      "image/png",
	})
	require.NoError(t, err)

	// userA tries to link resB to their memo. This must be rejected because userA doesn't own resB (unauthorized ActionWrite).
	_, err = s.SetMemoResources(ctxFor("userA"), &v1pb.SetMemoResourcesRequest{
		Name: "memos/memo-atomic",
		Resources: []*v1pb.Resource{
			{Name: "resources/res-a"},
			{Name: "resources/res-b"},
		},
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	// Verify resA is still linked and database state has not changed (atomicity)
	dbRes, err := db.GetResource(ctx, &store.FindResource{UID: proto.String("res-a")})
	require.NoError(t, err)
	require.NotNil(t, dbRes.MemoID)
	require.Equal(t, memo.ID, *dbRes.MemoID)
}

func TestSetMemoResourcesPreserveOtherUserResource(t *testing.T) {
	ctx := context.Background()
	db, s, users := setupResourceTestStore(t, ctx)
	defer db.Close()

	// 1. Create a private memo owned by userA (e.g. user ticket)
	memo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "memo-ticket",
		CreatorID:  users["userA"].ID,
		Content:    "User ticket",
		Visibility: store.Private,
	})
	require.NoError(t, err)

	// 2. Admin attaches their own resource to userA's ticket (this works because admin is superuser)
	_, err = db.CreateResource(ctx, &store.Resource{
		UID:       "res-admin",
		CreatorID: users["admin"].ID,
		Filename:  "admin-doc.pdf",
		Type:      "application/pdf",
		MemoID:    &memo.ID,
	})
	require.NoError(t, err)

	// 3. UserA creates their own resource (unattached)
	_, err = db.CreateResource(ctx, &store.Resource{
		UID:       "res-user",
		CreatorID: users["userA"].ID,
		Filename:  "user-pic.png",
		Type:      "image/png",
	})
	require.NoError(t, err)

	// 4. UserA calls SetMemoResources to update the list of resources:
	// they keep the admin's resource and add their own resource.
	// This must succeed because the admin's resource is already bound here!
	_, err = s.SetMemoResources(ctxFor("userA"), &v1pb.SetMemoResourcesRequest{
		Name: "memos/memo-ticket",
		Resources: []*v1pb.Resource{
			{Name: "resources/res-admin"},
			{Name: "resources/res-user"},
		},
	})
	require.NoError(t, err)

	// Verify both resources are bound to the memo
	dbAdminRes, err := db.GetResource(ctx, &store.FindResource{UID: proto.String("res-admin")})
	require.NoError(t, err)
	require.Equal(t, memo.ID, *dbAdminRes.MemoID)

	dbUserRes, err := db.GetResource(ctx, &store.FindResource{UID: proto.String("res-user")})
	require.NoError(t, err)
	require.Equal(t, memo.ID, *dbUserRes.MemoID)
}

func TestCreateResourcePreBlobAuthorization(t *testing.T) {
	ctx := context.Background()
	db, s, users := setupResourceTestStore(t, ctx)
	defer db.Close()

	_, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "memo-b",
		CreatorID:  users["userB"].ID,
		Content:    "Private B",
		Visibility: store.Private,
	})
	require.NoError(t, err)

	// userA tries to create resource linked to userB's memo.
	// Memo check should fail before SaveResourceBlob runs.
	memoName := "memos/memo-b"
	_, err = s.CreateResource(ctxFor("userA"), &v1pb.CreateResourceRequest{
		Resource: &v1pb.Resource{
			Filename: "hack.png",
			Type:     "image/png",
			Memo:     &memoName,
			Content:  []byte("hack content"),
		},
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	// Verify no resource was created in store
	resources, err := db.ListResources(ctx, &store.FindResource{})
	require.NoError(t, err)
	require.Len(t, resources, 0)
}

func TestCycleAndDepthLimits(t *testing.T) {
	ctx := context.Background()
	db, s, users := setupResourceTestStore(t, ctx)
	defer db.Close()

	memo1, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "memo1",
		CreatorID:  users["userA"].ID,
		Content:    "Content 1",
		Visibility: store.Private,
	})
	require.NoError(t, err)

	memo2, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "memo2",
		CreatorID:  users["userA"].ID,
		Content:    "Content 2",
		Visibility: store.Private,
	})
	require.NoError(t, err)

	_, err = db.UpsertMemoRelation(ctx, &store.MemoRelation{
		MemoID:        memo1.ID,
		RelatedMemoID: memo2.ID,
		Type:          store.MemoRelationComment,
	})
	require.NoError(t, err)

	_, err = db.UpsertMemoRelation(ctx, &store.MemoRelation{
		MemoID:        memo2.ID,
		RelatedMemoID: memo1.ID,
		Type:          store.MemoRelationComment,
	})
	require.NoError(t, err)

	// Refetch to populate ParentIDs
	memo1, err = db.GetMemo(ctx, &store.FindMemo{ID: &memo1.ID})
	require.NoError(t, err)

	res, err := db.CreateResource(ctx, &store.Resource{
		UID:       "res-cycle",
		CreatorID: users["userA"].ID,
		Filename:  "file.png",
		Type:      "image/png",
		MemoID:    &memo1.ID,
	})
	require.NoError(t, err)

	err = s.checkResourceAccess(ctxFor("userB"), res, ActionRead)
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
	require.Contains(t, err.Error(), "circular memo relation detected")

	// Test depth limit
	// 1. Create a chain of exactly 10 parent edges (11 memos: memo-depth-0 -> ... -> memo-depth-10)
	var prevMemo *store.Memo
	for i := 0; i <= 10; i++ {
		m, err := db.CreateMemo(ctx, &store.Memo{
			UID:        fmt.Sprintf("memo-depth-%d", i),
			CreatorID:  users["userA"].ID,
			Content:    fmt.Sprintf("Content %d", i),
			Visibility: store.Public,
		})
		require.NoError(t, err)
		if prevMemo != nil {
			_, err = db.UpsertMemoRelation(ctx, &store.MemoRelation{
				MemoID:        prevMemo.ID,
				RelatedMemoID: m.ID,
				Type:          store.MemoRelationComment,
			})
			require.NoError(t, err)
		}
		prevMemo = m
	}

	// Fetch memo-depth-0 so it has ParentID populated
	m0, err := db.GetMemo(ctx, &store.FindMemo{UID: proto.String("memo-depth-0")})
	require.NoError(t, err)

	resDepth10, err := db.CreateResource(ctx, &store.Resource{
		UID:       "res-depth-10",
		CreatorID: users["userA"].ID,
		Filename:  "file.png",
		Type:      "image/png",
		MemoID:    &m0.ID,
	})
	require.NoError(t, err)

	// Since it's exactly 10 edges, this should PASS
	err = s.checkResourceAccess(ctxFor("userB"), resDepth10, ActionRead)
	require.NoError(t, err)

	// 2. Now add one more level to exceed the limit (11 parent edges, 12 memos total)
	m11, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "memo-depth-11",
		CreatorID:  users["userA"].ID,
		Content:    "Content 11",
		Visibility: store.Public,
	})
	require.NoError(t, err)

	// Link memo-depth-10 to memo-depth-11
	m10, err := db.GetMemo(ctx, &store.FindMemo{UID: proto.String("memo-depth-10")})
	require.NoError(t, err)
	_, err = db.UpsertMemoRelation(ctx, &store.MemoRelation{
		MemoID:        m10.ID,
		RelatedMemoID: m11.ID,
		Type:          store.MemoRelationComment,
	})
	require.NoError(t, err)

	// Re-fetch m0 to refresh parent state
	m0, err = db.GetMemo(ctx, &store.FindMemo{UID: proto.String("memo-depth-0")})
	require.NoError(t, err)

	resDepth11, err := db.CreateResource(ctx, &store.Resource{
		UID:       "res-depth-11",
		CreatorID: users["userA"].ID,
		Filename:  "file.png",
		Type:      "image/png",
		MemoID:    &m0.ID,
	})
	require.NoError(t, err)

	// With 11 edges, it should FAIL
	err = s.checkResourceAccess(ctxFor("userB"), resDepth11, ActionRead)
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Contains(t, err.Error(), "memo traversal depth limit exceeded")
}

func TestInvalidActionFailClosed(t *testing.T) {
	ctx := context.Background()
	db, s, users := setupResourceTestStore(t, ctx)
	defer db.Close()

	res, err := db.CreateResource(ctx, &store.Resource{
		UID:       "res-invalid",
		CreatorID: users["userA"].ID,
		Filename:  "file.png",
		Type:      "image/png",
	})
	require.NoError(t, err)

	err = s.checkResourceAccess(ctxFor("userA"), res, ResourceAction(99))
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHostAndAdminSeparate(t *testing.T) {
	ctx := context.Background()
	db, s, users := setupResourceTestStore(t, ctx)
	defer db.Close()

	res, err := db.CreateResource(ctx, &store.Resource{
		UID:       "res-private",
		CreatorID: users["userA"].ID,
		Filename:  "secret.png",
		Type:      "image/png",
	})
	require.NoError(t, err)

	// Host can Read, cannot Write or Delete
	err = s.checkResourceAccess(ctxFor("host"), res, ActionRead)
	require.NoError(t, err)
	err = s.checkResourceAccess(ctxFor("host"), res, ActionWrite)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	err = s.checkResourceAccess(ctxFor("host"), res, ActionDelete)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	// Admin can Read, cannot Write or Delete
	err = s.checkResourceAccess(ctxFor("admin"), res, ActionRead)
	require.NoError(t, err)
	err = s.checkResourceAccess(ctxFor("admin"), res, ActionWrite)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	err = s.checkResourceAccess(ctxFor("admin"), res, ActionDelete)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
