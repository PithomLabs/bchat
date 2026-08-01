package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
	teststore "github.com/usememos/memos/store/test"
)

func setupTicketAPITest(t *testing.T) (*echo.Echo, *store.Store, map[string]*store.User, map[string]*store.Ticket) {
	t.Helper()

	ctx := context.Background()
	db := teststore.NewTestingStore(ctx, t)
	service := &APIV1Service{Store: db, Secret: "test-secret"}

	users := make(map[string]*store.User)
	for name, role := range map[string]store.Role{
		"userA": store.RoleUser,
		"userB": store.RoleUser,
		"admin": store.RoleAdmin,
		"host":  store.RoleHost,
	} {
		user, err := db.CreateUser(ctx, &store.User{
			Username:     name,
			Nickname:     name,
			Role:         role,
			PasswordHash: "hash",
		})
		require.NoError(t, err)
		users[name] = user
	}

	tickets := make(map[string]*store.Ticket)
	for name, owner := range map[string]*store.User{
		"ticketA": users["userA"],
		"ticketB": users["userB"],
	} {
		ticket, err := db.CreateTicket(ctx, &store.Ticket{
			Title:       name,
			Description: "/m/" + name,
			Status:      store.TicketStatusOpen,
			Priority:    store.TicketPriorityMedium,
			Type:        "SUPPORT",
			Tags:        []string{},
			CreatorID:   owner.ID,
			CreatedTs:   time.Now().Unix(),
			UpdatedTs:   time.Now().Unix(),
		})
		require.NoError(t, err)
		tickets[name] = ticket
	}

	e := echo.New()
	api := e.Group("/api/v1")
	api.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if username := c.Request().Header.Get("X-Test-User"); username != "" {
				user, exists := users[username]
				if !exists {
					return echo.NewHTTPError(http.StatusUnauthorized, "Unknown test user")
				}
				c.Set(getUserIDContextKey(), user.ID)
			}
			return next(c)
		}
	})
	service.RegisterTicketRoutes(api)

	return e, db, users, tickets
}

func performTicketAPIRequest(e *echo.Echo, method, path, username string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if username != "" {
		req.Header.Set("X-Test-User", username)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestTicketAPIRegularUserListsOnlyOwnTickets(t *testing.T) {
	e, db, users, tickets := setupTicketAPITest(t)
	defer db.Close()

	rec := performTicketAPIRequest(e, http.MethodGet, "/api/v1/tickets", "userA")
	require.Equal(t, http.StatusOK, rec.Code)

	var result []*Ticket
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Len(t, result, 1)
	require.Equal(t, tickets["ticketA"].ID, result[0].ID)
	require.Equal(t, users["userA"].ID, result[0].CreatorID)
}

func TestTicketAPIRegularUserCannotOverrideCreatorFilter(t *testing.T) {
	e, db, users, tickets := setupTicketAPITest(t)
	defer db.Close()

	path := fmt.Sprintf("/api/v1/tickets?creatorId=%d", users["userB"].ID)
	rec := performTicketAPIRequest(e, http.MethodGet, path, "userA")
	require.Equal(t, http.StatusOK, rec.Code)

	var result []*Ticket
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Len(t, result, 1)
	require.Equal(t, tickets["ticketA"].ID, result[0].ID)
}

func TestTicketAPIAssigneesRequireInternalStaff(t *testing.T) {
	e, db, _, _ := setupTicketAPITest(t)
	defer db.Close()

	tests := []struct {
		name     string
		username string
		status   int
	}{
		{name: "unauthenticated", status: http.StatusUnauthorized},
		{name: "regular user", username: "userA", status: http.StatusForbidden},
		{name: "admin", username: "admin", status: http.StatusOK},
		{name: "host", username: "host", status: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := performTicketAPIRequest(e, http.MethodGet, "/api/v1/tickets/assignees", test.username)
			require.Equal(t, test.status, rec.Code)
			if test.status == http.StatusOK {
				var result []*AssigneeUser
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
				require.Len(t, result, 4)
			}
		})
	}
}

func TestTicketAPIRegularUserCannotGetAnotherUsersTicket(t *testing.T) {
	e, db, _, tickets := setupTicketAPITest(t)
	defer db.Close()

	path := fmt.Sprintf("/api/v1/tickets/%d", tickets["ticketB"].ID)
	rec := performTicketAPIRequest(e, http.MethodGet, path, "userA")
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func setupTicketServiceTest(t *testing.T) (*store.Store, *APIV1Service, int32) {
	t.Helper()
	ctx := context.Background()
	db := teststore.NewTestingStore(ctx, t)
	tenant, err := db.CreateAgentTenant(ctx, &store.AgentTenant{
		Slug:        "test-tenant",
		CompanyName: "Test Tenant",
		IsActive:    true,
	})
	require.NoError(t, err)
	return db, &APIV1Service{Store: db, Secret: "test-secret"}, tenant.ID
}

func TestCreateSystemResolutionComment_LegacyMemo(t *testing.T) {
	db, s, tenantID := setupTicketServiceTest(t)
	defer db.Close()
	ctx := context.Background()

	user, err := db.CreateUser(ctx, &store.User{
		Username:     "test-user",
		Nickname:     "Test User",
		Role:         store.RoleUser,
		PasswordHash: "hash",
	})
	require.NoError(t, err)

	legacyMemo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "legacy-parent",
		CreatorID:  user.ID,
		Content:    "legacy content",
		Visibility: store.Public,
		RowStatus:  store.Normal,
		TenantID:   nil,
	})
	require.NoError(t, err)

	ticket, err := db.CreateTicket(ctx, &store.Ticket{
		Title:       "Legacy Memo Ticket",
		Description: "/m/" + legacyMemo.UID,
		Status:      store.TicketStatusOpen,
		Priority:    store.TicketPriorityMedium,
		Type:        "SUPPORT",
		CreatorID:   user.ID,
		TenantID:    &tenantID,
		CreatedTs:   time.Now().Unix(),
		UpdatedTs:   time.Now().Unix(),
	})
	require.NoError(t, err)

	err = s.createSystemResolutionComment(ctx, tenantID, ticket, "test suggestion")
	require.NoError(t, err)
}

func TestCreateSystemResolutionComment_CrossTenantReject(t *testing.T) {
	db, s, tenantID := setupTicketServiceTest(t)
	defer db.Close()
	ctx := context.Background()

	user, err := db.CreateUser(ctx, &store.User{
		Username:     "test-user",
		Nickname:     "Test User",
		Role:         store.RoleUser,
		PasswordHash: "hash",
	})
	require.NoError(t, err)

	otherTenant := int32(tenantID + 1)
	otherMemo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "other-tenant-parent",
		CreatorID:  user.ID,
		Content:    "other tenant content",
		Visibility: store.Public,
		RowStatus:  store.Normal,
		TenantID:   &otherTenant,
	})
	require.NoError(t, err)

	ticket, err := db.CreateTicket(ctx, &store.Ticket{
		Title:       "Cross Tenant Ticket",
		Description: "/m/" + otherMemo.UID,
		Status:      store.TicketStatusOpen,
		Priority:    store.TicketPriorityMedium,
		Type:        "SUPPORT",
		CreatorID:   user.ID,
		TenantID:    &tenantID,
		CreatedTs:   time.Now().Unix(),
		UpdatedTs:   time.Now().Unix(),
	})
	require.NoError(t, err)

	err = s.createSystemResolutionComment(ctx, tenantID, ticket, "test suggestion")
	require.Error(t, err)
}

func TestCreateSystemResolutionComment_EmptyUID(t *testing.T) {
	db, s, tenantID := setupTicketServiceTest(t)
	defer db.Close()
	ctx := context.Background()

	user, err := db.CreateUser(ctx, &store.User{
		Username:     "test-user",
		Nickname:     "Test User",
		Role:         store.RoleUser,
		PasswordHash: "hash",
	})
	require.NoError(t, err)

	ticket, err := db.CreateTicket(ctx, &store.Ticket{
		Title:       "Empty UID Ticket",
		Description: "/m/",
		Status:      store.TicketStatusOpen,
		Priority:    store.TicketPriorityMedium,
		Type:        "SUPPORT",
		CreatorID:   user.ID,
		TenantID:    &tenantID,
		CreatedTs:   time.Now().Unix(),
		UpdatedTs:   time.Now().Unix(),
	})
	require.NoError(t, err)

	err = s.createSystemResolutionComment(ctx, tenantID, ticket, "test suggestion")
	require.NoError(t, err)
}

func TestGetTicketComments_LegacyParentMemo(t *testing.T) {
	db, s, tenantID := setupTicketServiceTest(t)
	defer db.Close()
	ctx := context.Background()

	user, err := db.CreateUser(ctx, &store.User{
		Username:     "test-user",
		Nickname:     "Test User",
		Role:         store.RoleUser,
		PasswordHash: "hash",
	})
	require.NoError(t, err)

	legacyMemo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "legacy-parent-comments",
		CreatorID:  user.ID,
		Content:    "legacy parent",
		Visibility: store.Public,
		RowStatus:  store.Normal,
		TenantID:   nil,
	})
	require.NoError(t, err)

	commentMemo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "legacy-comment",
		CreatorID:  user.ID,
		Content:    "legacy comment",
		Visibility: store.Public,
		RowStatus:  store.Normal,
		TenantID:   &tenantID,
	})
	require.NoError(t, err)

	_, err = db.UpsertMemoRelation(ctx, &store.MemoRelation{
		MemoID:        commentMemo.ID,
		RelatedMemoID: legacyMemo.ID,
		Type:          store.MemoRelationComment,
		TenantID:      &tenantID,
	})
	require.NoError(t, err)

	ticket, err := db.CreateTicket(ctx, &store.Ticket{
		Title:       "Legacy Parent Ticket",
		Description: "/m/" + legacyMemo.UID,
		Status:      store.TicketStatusOpen,
		Priority:    store.TicketPriorityMedium,
		Type:        "SUPPORT",
		CreatorID:   user.ID,
		TenantID:    &tenantID,
		CreatedTs:   time.Now().Unix(),
		UpdatedTs:   time.Now().Unix(),
	})
	require.NoError(t, err)

	comments, err := s.getTicketComments(ctx, ticket)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	require.Equal(t, commentMemo.ID, comments[0].ID)
}

func TestGetTicketComments_RelationIsolation(t *testing.T) {
	db, s, tenantID := setupTicketServiceTest(t)
	defer db.Close()
	ctx := context.Background()

	user, err := db.CreateUser(ctx, &store.User{
		Username:     "test-user",
		Nickname:     "Test User",
		Role:         store.RoleUser,
		PasswordHash: "hash",
	})
	require.NoError(t, err)

	legacyMemo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "legacy-parent-isolation",
		CreatorID:  user.ID,
		Content:    "legacy parent isolation",
		Visibility: store.Public,
		RowStatus:  store.Normal,
		TenantID:   nil,
	})
	require.NoError(t, err)

	commentTenant19, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "comment-19",
		CreatorID:  user.ID,
		Content:    "comment 19",
		Visibility: store.Public,
		RowStatus:  store.Normal,
		TenantID:   &tenantID,
	})
	require.NoError(t, err)

	otherTenantID := tenantID + 1
	commentTenant20, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "comment-20",
		CreatorID:  user.ID,
		Content:    "comment 20",
		Visibility: store.Public,
		RowStatus:  store.Normal,
		TenantID:   &otherTenantID,
	})
	require.NoError(t, err)

	nilTenantComment, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "comment-nil",
		CreatorID:  user.ID,
		Content:    "comment nil",
		Visibility: store.Public,
		RowStatus:  store.Normal,
		TenantID:   nil,
	})
	require.NoError(t, err)

	_, err = db.UpsertMemoRelation(ctx, &store.MemoRelation{
		MemoID:        commentTenant19.ID,
		RelatedMemoID: legacyMemo.ID,
		Type:          store.MemoRelationComment,
		TenantID:      &tenantID,
	})
	require.NoError(t, err)

	_, err = db.UpsertMemoRelation(ctx, &store.MemoRelation{
		MemoID:        commentTenant20.ID,
		RelatedMemoID: legacyMemo.ID,
		Type:          store.MemoRelationComment,
		TenantID:      &otherTenantID,
	})
	require.NoError(t, err)

	_, err = db.UpsertMemoRelation(ctx, &store.MemoRelation{
		MemoID:        nilTenantComment.ID,
		RelatedMemoID: legacyMemo.ID,
		Type:          store.MemoRelationComment,
		TenantID:      nil,
	})
	require.NoError(t, err)

	ticket, err := db.CreateTicket(ctx, &store.Ticket{
		Title:       "Isolation Ticket",
		Description: "/m/" + legacyMemo.UID,
		Status:      store.TicketStatusOpen,
		Priority:    store.TicketPriorityMedium,
		Type:        "SUPPORT",
		CreatorID:   user.ID,
		TenantID:    &tenantID,
		CreatedTs:   time.Now().Unix(),
		UpdatedTs:   time.Now().Unix(),
	})
	require.NoError(t, err)

	comments, err := s.getTicketComments(ctx, ticket)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	require.Equal(t, commentTenant19.ID, comments[0].ID)
}
