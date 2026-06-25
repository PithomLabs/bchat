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
