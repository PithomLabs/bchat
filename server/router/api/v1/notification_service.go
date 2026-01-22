package v1

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/usememos/memos/store"
)

type NotificationResponse struct {
	ID            int32  `json:"id"`
	InitiatorID   int32  `json:"initiatorId"`
	InitiatorName string `json:"initiatorName"`
	ReceiverID    int32  `json:"receiverId"`
	TicketURL     string `json:"ticketUrl"`
	CreatedTs     int64  `json:"createdTs"`
	IsRead        bool   `json:"isRead"`
}

type UpdateNotificationRequest struct {
	IsRead *bool `json:"isRead"`
}

func (s *APIV1Service) RegisterNotificationRoutes(g *echo.Group) {
	g.GET("/notifications", s.ListNotifications)
	g.PATCH("/notifications/:id", s.UpdateNotification)
}

func (s *APIV1Service) ListNotifications(c echo.Context) error {
	ctx := c.Request().Context()
	userID, ok := c.Get(getUserIDContextKey()).(int32)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Missing user in context")
	}

	find := &store.FindNotification{
		ReceiverID: &userID,
	}

	// Optional: Filter by isRead status if needed via query param
	// if isReadStr := c.QueryParam("isRead"); isReadStr != "" {
	// 	isRead := isReadStr == "true"
	// 	find.IsRead = &isRead
	// }

	list, err := s.Store.ListNotifications(ctx, find)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list notifications").SetInternal(err)
	}

	result := make([]*NotificationResponse, 0, len(list))
	for _, n := range list {
		result = append(result, s.convertNotificationFromStore(ctx, n))
	}

	return c.JSON(http.StatusOK, result)
}

func (s *APIV1Service) UpdateNotification(c echo.Context) error {
	ctx := c.Request().Context()
	userID, ok := c.Get(getUserIDContextKey()).(int32)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Missing user in context")
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid notification ID")
	}

	slog.Info("UpdateNotification request", "id", id, "userID", userID)

	request := &UpdateNotificationRequest{}
	if err := c.Bind(request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body").SetInternal(err)
	}

	// Verify ownership
	notification, err := s.Store.ListNotifications(ctx, &store.FindNotification{ID: point(int32(id))})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch notification").SetInternal(err)
	}
	if len(notification) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Notification not found")
	}
	if notification[0].ReceiverID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "Permission denied")
	}

	update := &store.UpdateNotification{
		ID:     int32(id),
		IsRead: request.IsRead,
	}

	updated, err := s.Store.UpdateNotification(ctx, update)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update notification").SetInternal(err)
	}

	return c.JSON(http.StatusOK, s.convertNotificationFromStore(ctx, updated))
}

func (s *APIV1Service) convertNotificationFromStore(ctx context.Context, n *store.Notification) *NotificationResponse {
	response := &NotificationResponse{
		ID:          n.ID,
		InitiatorID: n.InitiatorID,
		ReceiverID:  n.ReceiverID,
		TicketURL:   n.TicketURL,
		CreatedTs:   n.CreatedTs,
		IsRead:      n.IsRead,
	}

	user, _ := s.Store.GetUser(ctx, &store.FindUser{ID: &n.InitiatorID})
	if user != nil {
		if user.Nickname != "" {
			response.InitiatorName = user.Nickname
		} else {
			response.InitiatorName = user.Username
		}
		slog.Info("Resolved initiator name", "id", n.InitiatorID, "name", response.InitiatorName)
	} else {
		response.InitiatorName = "Unknown"
		slog.Warn("Failed to resolve initiator name", "id", n.InitiatorID)
	}

	return response
}

func point[T any](v T) *T {
	return &v
}
