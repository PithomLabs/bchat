package v1

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/usememos/memos/store"
)

type Ticket struct {
	ID          int32    `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	CreatorID   int32    `json:"creatorId"`
	AssigneeID  *int32   `json:"assigneeId"`
	CreatedTs   int64    `json:"createdTs"`
	UpdatedTs   int64    `json:"updatedTs"`
	Type        string   `json:"type"`
	Tags        []string `json:"tags"`
}

type CreateTicketRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Type        string   `json:"type"`
	Tags        []string `json:"tags"`
	AssigneeID  *int32   `json:"assigneeId"`
}

type UpdateTicketRequest struct {
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	Status      *string  `json:"status"`
	Priority    *string  `json:"priority"`
	Type        *string  `json:"type"`
	Tags        []string `json:"tags"`
	AssigneeID  *int32   `json:"assigneeId"`
}

func (s *APIV1Service) RegisterTicketRoutes(g *echo.Group) {
	g.POST("/tickets", s.CreateTicket)
	g.GET("/tickets", s.ListTickets)
	g.GET("/tickets/assignees", s.ListTicketAssignees)
	g.GET("/tickets/:id", s.GetTicket)
	g.PATCH("/tickets/:id", s.UpdateTicket)
	g.DELETE("/tickets/:id", s.DeleteTicket)
}

func (s *APIV1Service) CreateTicket(c echo.Context) error {
	ctx := c.Request().Context()
	slog.Info("CreateTicket handler", "context_keys", c.ParamNames())
	userID, ok := c.Get(getUserIDContextKey()).(int32)
	slog.Info("CreateTicket userID", "userID", userID, "ok", ok)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Missing user in context")
	}

	request := &CreateTicketRequest{}
	if err := c.Bind(request); err != nil {
		slog.Error("CreateTicket bind error", "error", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body").SetInternal(err)
	}
	slog.Info("CreateTicket request", "title", request.Title, "status", request.Status, "priority", request.Priority)

	ticket := &store.Ticket{
		Title:       request.Title,
		Description: request.Description,
		Status:      store.TicketStatus(request.Status),
		Priority:    store.TicketPriority(request.Priority),
		Type:        request.Type,
		Tags:        request.Tags,
		CreatorID:   userID,
		AssigneeID:  request.AssigneeID,
		CreatedTs:   time.Now().Unix(),
		UpdatedTs:   time.Now().Unix(),
	}

	if ticket.Type == "" {
		ticket.Type = "TASK"
	}
	if ticket.Tags == nil {
		ticket.Tags = []string{}
	}

	if err := ticket.Validate(); err != nil {
		slog.Error("CreateTicket validate error", "error", err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	slog.Info("CreateTicket validated")

	ticket, err := s.Store.CreateTicket(ctx, ticket)
	if err != nil {
		slog.Error("CreateTicket store error", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create ticket").SetInternal(err)
	}

	slog.Info("CreateTicket success", "id", ticket.ID)

	return c.JSON(http.StatusOK, convertTicketFromStore(ticket))
}

func (s *APIV1Service) ListTickets(c echo.Context) error {
	ctx := c.Request().Context()

	find := &store.FindTicket{}
	if typeStr := c.QueryParam("type"); typeStr != "" {
		find.Type = &typeStr
	}
	if creatorIDStr := c.QueryParam("creatorId"); creatorIDStr != "" {
		creatorID, err := strconv.Atoi(creatorIDStr)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid creatorId")
		}
		id := int32(creatorID)
		find.CreatorID = &id
	}

	list, err := s.Store.ListTickets(ctx, find)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list tickets").SetInternal(err)
	}

	result := make([]*Ticket, 0, len(list))
	for _, t := range list {
		result = append(result, convertTicketFromStore(t))
	}

	return c.JSON(http.StatusOK, result)
}

// AssigneeUser is a simplified user structure for the assignee dropdown
type AssigneeUser struct {
	ID       int32  `json:"id"`
	Username string `json:"username"`
}

func (s *APIV1Service) ListTicketAssignees(c echo.Context) error {
	ctx := c.Request().Context()

	// Verify user is logged in
	_, ok := c.Get(getUserIDContextKey()).(int32)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Missing user in context")
	}

	// List all users for assignee dropdown
	users, err := s.Store.ListUsers(ctx, &store.FindUser{})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list users").SetInternal(err)
	}

	result := make([]*AssigneeUser, 0, len(users))
	for _, user := range users {
		result = append(result, &AssigneeUser{
			ID:       user.ID,
			Username: user.Username,
		})
	}

	return c.JSON(http.StatusOK, result)
}

func (s *APIV1Service) UpdateTicket(c echo.Context) error {
	ctx := c.Request().Context()
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ticket ID")
	}

	request := &UpdateTicketRequest{}
	if err := c.Bind(request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body").SetInternal(err)
	}

	update := &store.UpdateTicket{
		ID:          int32(id),
		Title:       request.Title,
		Description: request.Description,
		AssigneeID:  request.AssigneeID,
	}
	if request.Status != nil {
		status := store.TicketStatus(*request.Status)
		update.Status = &status
	}
	if request.Priority != nil {
		priority := store.TicketPriority(*request.Priority)
		update.Priority = &priority
	}
	if request.Type != nil {
		update.Type = request.Type
	}
	if request.Tags != nil {
		update.Tags = request.Tags
	}
	now := time.Now().Unix()
	update.UpdatedTs = &now

	ticket, err := s.Store.UpdateTicket(ctx, update)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update ticket").SetInternal(err)
	}

	return c.JSON(http.StatusOK, convertTicketFromStore(ticket))
}

func (s *APIV1Service) DeleteTicket(c echo.Context) error {
	ctx := c.Request().Context()
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ticket ID")
	}

	if err := s.Store.DeleteTicket(ctx, &store.DeleteTicket{ID: int32(id)}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete ticket").SetInternal(err)
	}

	return c.JSON(http.StatusOK, true)
}

func convertTicketFromStore(ticket *store.Ticket) *Ticket {
	return &Ticket{
		ID:          ticket.ID,
		Title:       ticket.Title,
		Description: ticket.Description,
		Status:      string(ticket.Status),
		Priority:    string(ticket.Priority),
		CreatorID:   ticket.CreatorID,
		AssigneeID:  ticket.AssigneeID,
		CreatedTs:   ticket.CreatedTs,
		UpdatedTs:   ticket.UpdatedTs,
		Type:        ticket.Type,
		Tags:        ticket.Tags,
	}
}

func (s *APIV1Service) GetTicket(c echo.Context) error {
	ctx := c.Request().Context()
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ticket ID")
	}

	// Use FindTicket to get by ID
	ticketID := int32(id)
	slog.Info("GetTicket request", "id", ticketID)
	list, err := s.Store.ListTickets(ctx, &store.FindTicket{
		ID: &ticketID,
	})
	if err != nil {
		slog.Error("GetTicket store error", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get ticket").SetInternal(err)
	}

	// SMART FALLBACK: If ticket not found by ID, it might be a Legacy Memo ID.
	if len(list) == 0 {
		slog.Warn("GetTicket not found by ID, attempting fallback to Memo ID", "id", ticketID)

		// Try to find if a memo with this ID exists
		memoID := int32(id)
		memo, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: &memoID})
		if err == nil && memo != nil {
			// Found a memo. Now find the ticket that points to this memo.
			descriptionLink := "/m/" + memo.UID
			slog.Info("Found memo for ticket fallback", "memoID", memoID, "uid", memo.UID)

			tickets, err := s.Store.ListTickets(ctx, &store.FindTicket{
				Description: &descriptionLink,
			})
			if err == nil && len(tickets) > 0 {
				slog.Info("Successfully resolved ticket from memo link", "ticketID", tickets[0].ID)
				list = tickets
			}
		}
	}

	if len(list) == 0 {
		slog.Warn("GetTicket not found after all fallbacks", "id", ticketID)
		return echo.NewHTTPError(http.StatusNotFound, "Ticket not found")
	}

	slog.Info("GetTicket success", "id", list[0].ID)
	return c.JSON(http.StatusOK, convertTicketFromStore(list[0]))
}

// Helper to match the key used in common/auth.go checks
func getUserIDContextKey() string {
	return "user-id"
}
