package v1

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/usememos/memos/server/router/api/v1/agent"
	"github.com/usememos/memos/store"
)

type Ticket struct {
	ID            int32    `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Status        string   `json:"status"`
	Priority      string   `json:"priority"`
	CreatorID     int32    `json:"creatorId"`
	AssigneeID    *int32   `json:"assigneeId"`
	CreatedTs     int64    `json:"createdTs"`
	UpdatedTs     int64    `json:"updatedTs"`
	Type          string   `json:"type"`
	Tags          []string `json:"tags"`
	InternalNotes string   `json:"internalNotes"`
	TenantID      *int32   `json:"tenantId"`
	TenantName    string   `json:"tenantName"`
}

type CreateTicketRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Type        string   `json:"type"`
	Tags        []string `json:"tags"`
	AssigneeID  *int32   `json:"assigneeId"`
	TenantID    *int32   `json:"tenantId"`
}

type UpdateTicketRequest struct {
	Title         *string  `json:"title"`
	Description   *string  `json:"description"`
	Status        *string  `json:"status"`
	Priority      *string  `json:"priority"`
	Type          *string  `json:"type"`
	Tags          []string `json:"tags"`
	AssigneeID    *int32   `json:"assigneeId"`
	InternalNotes *string  `json:"internalNotes"`
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

	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil || user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
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
		TenantID:    getTenantFromContext(c),
	}

	if request.TenantID != nil {
		if !isSuperUser(user) {
			return echo.NewHTTPError(http.StatusBadRequest, "tenantId is only available to admins")
		}
		tenant, err := s.Store.GetAgentTenant(ctx, &store.FindAgentTenant{ID: request.TenantID})
		if err != nil || tenant == nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid tenantId")
		}
		ticket.TenantID = request.TenantID
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

	// Check for existing ticket with same memo description (auto-creation de-duplication)
	if strings.HasPrefix(ticket.Description, "/m/") {
		existingList, err := s.Store.ListTickets(ctx, &store.FindTicket{
			Description: &ticket.Description,
			CreatorID:   &userID,
		})
		if err == nil && len(existingList) > 0 {
			existing := existingList[0]

			// Smart merge: preserve auto-derived values if user didn't override
			if ticket.Priority == store.TicketPriorityMedium {
				// User used default, keep auto-derived priority
				ticket.Priority = existing.Priority
			}
			if ticket.Type == "" || ticket.Type == "TASK" {
				// User used default, keep auto-derived type
				ticket.Type = existing.Type
			}

			// Customers cannot change ticket assignees
			assigneeID := ticket.AssigneeID
			if !isSuperUser(user) {
				assigneeID = nil
			}

			// Update the existing ticket
			update := &store.UpdateTicket{
				ID:          existing.ID,
				Title:       &ticket.Title,
				Description: &ticket.Description,
				Status:      &ticket.Status,
				Priority:    &ticket.Priority,
				Type:        &ticket.Type,
				Tags:        ticket.Tags,
				AssigneeID:  assigneeID,
			}
			now := time.Now().Unix()
			update.UpdatedTs = &now

			ticket, err = s.Store.UpdateTicket(ctx, update)
			if err != nil {
				slog.Error("CreateTicket store update error", "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update existing ticket").SetInternal(err)
			}
			slog.Info("CreateTicket deduplication success", "id", ticket.ID)

		// Index deduplicated ticket for RAG
		if s.agentHandler != nil && ticket.TenantID != nil {
			go func() {
				ctx := context.WithoutCancel(ctx)
				_, _, err := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, nil, false)
				if err != nil {
					slog.Error("failed to index deduplicated ticket for RAG", "ticket_id", ticket.ID, "error", err)
					}
				}()
			}

			return c.JSON(http.StatusOK, convertTicketFromStore(ticket))
		}
	}

	ticket, err = s.Store.CreateTicket(ctx, ticket)
	if err != nil {
		slog.Error("CreateTicket store error", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create ticket").SetInternal(err)
	}

	// Index ticket for RAG, then trigger inference (chained, not parallel)
	if s.agentHandler != nil && ticket.TenantID != nil {
		go func() {
			ctx := context.WithoutCancel(ctx)
			_, _, err := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, nil, true)
			if err != nil {
				slog.Error("failed to index new ticket for RAG", "ticket_id", ticket.ID, "error", err)
				return
			}

			// Fetch updated ticket to read populated internal_notes
			updated, fetchErr := s.Store.GetTicket(ctx, &store.FindTicket{ID: &ticket.ID})
			if fetchErr != nil || updated == nil || updated.InternalNotes == "" {
				return
			}

			// Create system-authored comment with the inferred resolution
			suggestion := updated.InternalNotes
			if commentErr := s.createSystemResolutionComment(ctx, *ticket.TenantID, ticket, suggestion); commentErr != nil {
				slog.Error("failed to create system resolution comment", "ticket_id", ticket.ID, "error", commentErr)
			}
		}()
	}

	slog.Info("CreateTicket success", "id", ticket.ID)

	return c.JSON(http.StatusOK, convertTicketFromStore(ticket))
}

func (s *APIV1Service) ListTickets(c echo.Context) error {
	ctx := c.Request().Context()
	userID, ok := c.Get(getUserIDContextKey()).(int32)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Missing user in context")
	}
	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil || user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}

	find := &store.FindTicket{}
	if desc := c.QueryParam("description"); desc != "" {
		find.Description = &desc
	}
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

	if !isSuperUser(user) {
		// Customers can only list their own tickets
		find.CreatorID = &userID
	}

	// Apply tenant filter (defense-in-depth)
	ApplyTicketTenantFilter(c, s.Store, find)

	list, err := s.Store.ListTickets(ctx, find)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list tickets").SetInternal(err)
	}

	result := make([]*Ticket, 0, len(list))
	// Resolve internal notes permission once before loop
	tenantID := getTenantFromContext(c)
	var hasPerm bool
	if tenantID != nil {
		resolvedPerms, err := agent.ResolveEffectivePermissions(ctx, s.Store, *tenantID, userID)
		if err == nil {
			hasPerm = agent.HasPermission(resolvedPerms, agent.PermTicketInternalNotes)
		}
	}
	for _, t := range list {
		resp := convertTicketFromStore(t)
		filterInternalNotes(resp, t, user, hasPerm)
		result = append(result, resp)
	}

	// Batch-resolve tenant names (N+1 is acceptable for admin-scale ticket lists)
	tenantMap := make(map[int32]string)
	for _, t := range result {
		if t.TenantID != nil {
			if _, ok := tenantMap[*t.TenantID]; !ok {
				tenant, err := s.Store.GetAgentTenant(ctx, &store.FindAgentTenant{ID: t.TenantID})
				if err == nil && tenant != nil {
					tenantMap[*t.TenantID] = tenant.CompanyName
				}
			}
		}
	}
	for _, t := range result {
		if t.TenantID != nil {
			t.TenantName = tenantMap[*t.TenantID]
		}
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

	userID, ok := c.Get(getUserIDContextKey()).(int32)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Missing user in context")
	}

	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil || user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}
	if !isSuperUser(user) {
		return echo.NewHTTPError(http.StatusForbidden, "Only internal staff can list ticket assignees")
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
	userID, ok := c.Get(getUserIDContextKey()).(int32)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Missing user in context")
	}
	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil || user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ticket ID")
	}

	// Verify ownership/permission before update
	ticketID := int32(id)
	existingList, err := s.Store.ListTickets(ctx, &store.FindTicket{ID: &ticketID})
	if err != nil || len(existingList) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Ticket not found")
	}
	existingTicket := existingList[0]

	// Check tenant ownership (superusers bypass this check)
	tenantID := getTenantFromContext(c)
	if tenantID != nil && existingTicket.TenantID != nil && *existingTicket.TenantID != *tenantID && !isSuperUser(user) {
		return echo.NewHTTPError(http.StatusForbidden, "You do not have permission to update this ticket")
	}

	if !isSuperUser(user) && existingTicket.CreatorID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "You do not have permission to update this ticket")
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

	// Customers cannot change ticket assignees
	if !isSuperUser(user) {
		update.AssigneeID = nil
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
	// Internal notes update requires ticket:internal_notes permission or superuser
	if request.InternalNotes != nil {
		tenantID := getTenantFromContext(c)
		hasPerm := false
		if tenantID != nil {
			resolvedPerms, err := agent.ResolveEffectivePermissions(ctx, s.Store, *tenantID, userID)
			if err == nil {
				hasPerm = agent.HasPermission(resolvedPerms, agent.PermTicketInternalNotes)
			}
		}
		if isSuperUser(user) || hasPerm {
			update.InternalNotes = request.InternalNotes
		}
	}
	now := time.Now().Unix()
	update.UpdatedTs = &now

	ticket, err := s.Store.UpdateTicket(ctx, update)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update ticket").SetInternal(err)
	}

	// Re-index ticket content for RAG
	if s.agentHandler != nil && ticket.TenantID != nil {
		go func() {
			ctx := context.WithoutCancel(ctx)
			comments, err := s.getTicketComments(ctx, ticket)
			if err != nil {
				slog.Warn("failed to fetch comments for ticket re-index, indexing title+description only",
					"ticket_id", ticket.ID, "error", err)
			}
			_, _, idxErr := s.agentHandler.GetService().IndexTicketContent(ctx, *ticket.TenantID, ticket, comments, false)
			if idxErr != nil {
				slog.Error("failed to re-index updated ticket for RAG", "ticket_id", ticket.ID, "error", idxErr)
			}
		}()
	}

	return c.JSON(http.StatusOK, convertTicketFromStore(ticket))
}

func (s *APIV1Service) DeleteTicket(c echo.Context) error {
	ctx := c.Request().Context()
	userID, ok := c.Get(getUserIDContextKey()).(int32)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Missing user in context")
	}
	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil || user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}

	// Customers cannot delete support tickets (required for history/compliance)
	if !isSuperUser(user) {
		return echo.NewHTTPError(http.StatusForbidden, "Only internal staff can delete tickets")
	}

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

func (s *APIV1Service) GetTicket(c echo.Context) error {
	ctx := c.Request().Context()
	userID, ok := c.Get(getUserIDContextKey()).(int32)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Missing user in context")
	}
	user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil || user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}

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

	ticket := list[0]

	// Check tenant ownership (superusers bypass this check)
	tenantID := getTenantFromContext(c)
	if tenantID != nil && ticket.TenantID != nil && *ticket.TenantID != *tenantID && !isSuperUser(user) {
		return echo.NewHTTPError(http.StatusForbidden, "You do not have permission to access this ticket")
	}

	// Security check: Only superusers or creator can see the ticket details
	if !isSuperUser(user) && ticket.CreatorID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "You do not have permission to access this ticket")
	}

	// RBAC: filter internal notes based on permissions
	tnID := getTenantFromContext(c)
	var hasPerm bool
	if tnID != nil {
		resolvedPerms, err := agent.ResolveEffectivePermissions(ctx, s.Store, *tnID, userID)
		if err == nil {
			hasPerm = agent.HasPermission(resolvedPerms, agent.PermTicketInternalNotes)
		}
	}
	resp := convertTicketFromStore(ticket)
	filterInternalNotes(resp, ticket, user, hasPerm)

	slog.Info("GetTicket success", "id", ticket.ID)
	return c.JSON(http.StatusOK, resp)
}

func convertTicketFromStore(ticket *store.Ticket) *Ticket {
	return &Ticket{
		ID:            ticket.ID,
		Title:         ticket.Title,
		Description:   ticket.Description,
		Status:        string(ticket.Status),
		Priority:      string(ticket.Priority),
		CreatorID:     ticket.CreatorID,
		AssigneeID:    ticket.AssigneeID,
		CreatedTs:     ticket.CreatedTs,
		UpdatedTs:     ticket.UpdatedTs,
		Type:          ticket.Type,
		Tags:          ticket.Tags,
		InternalNotes: ticket.InternalNotes,
		TenantID:      ticket.TenantID,
	}
}

// filterInternalNotes hides internal notes for users without permission.
// Visibility: superuser, ticket creator, assigned user, or ticket:internal_notes permission.
func filterInternalNotes(resp *Ticket, ticket *store.Ticket, user *store.User, hasPerm bool) {
	if isSuperUser(user) || ticket.CreatorID == user.ID ||
		(ticket.AssigneeID != nil && *ticket.AssigneeID == user.ID) || hasPerm {
		return
	}
	resp.InternalNotes = ""
}

// Helper to match the key used in common/auth.go checks
func getUserIDContextKey() string {
	return "user-id"
}

func getTenantIDContextKey() string {
	return "tenant-id"
}

// getTicketComments fetches all comment memos for a ticket.
// Returns nil, nil if the ticket has no description link or no comments.
func (s *APIV1Service) getTicketComments(ctx context.Context, ticket *store.Ticket) ([]*store.Memo, error) {
	if !strings.HasPrefix(ticket.Description, "/m/") {
		return nil, nil
	}
	memoUID := strings.TrimPrefix(ticket.Description, "/m/")
	parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
		UID:       &memoUID,
		TenantID:  ticket.TenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get parent memo: %w", err)
	}
	if parentMemo == nil {
		return nil, nil
	}
	commentType := store.MemoRelationComment
	relations, err := s.Store.ListMemoRelations(ctx, &store.FindMemoRelation{
		RelatedMemoID: &parentMemo.ID,
		Type:          &commentType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list memo relations: %w", err)
	}
	var comments []*store.Memo
	for _, rel := range relations {
		memo, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: &rel.MemoID})
		if err != nil {
			slog.Warn("failed to load comment memo", "memo_id", rel.MemoID, "error", err)
			continue
		}
		if memo != nil {
			comments = append(comments, memo)
		}
	}
	return comments, nil
}

func (s *APIV1Service) createSystemResolutionComment(ctx context.Context, tenantID int32, ticket *store.Ticket, suggestion string) error {
	if !strings.HasPrefix(ticket.Description, "/m/") {
		return nil
	}
	memoUID := strings.TrimPrefix(ticket.Description, "/m/")

	parentMemo, err := s.Store.GetMemo(ctx, &store.FindMemo{
		UID:      &memoUID,
		TenantID: &tenantID,
	})
	if err != nil || parentMemo == nil {
		return fmt.Errorf("parent memo not found: %w", err)
	}

	comment, err := s.Store.CreateMemo(ctx, &store.Memo{
		RowStatus:  store.Normal,
		CreatorID:  store.SystemBotID,
		Content:    "## AI Suggestion\n\n" + suggestion,
		Visibility: store.Public,
		TenantID:   &tenantID,
	})
	if err != nil {
		return fmt.Errorf("failed to create system comment: %w", err)
	}

	_, err = s.Store.UpsertMemoRelation(ctx, &store.MemoRelation{
		MemoID:        comment.ID,
		RelatedMemoID: parentMemo.ID,
		Type:          store.MemoRelationComment,
		TenantID:      &tenantID,
	})
	if err != nil {
		return fmt.Errorf("failed to link system comment: %w", err)
	}

	return nil
}
