package v1

import (
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/starfederation/datastar-go/datastar"
)

// Notification represents a real-time notification to be pushed via SSE
type Notification struct {
	ID         int32
	Type       string // "MEMO_COMMENT" or "TICKET_COMMENT"
	SenderName string
	SenderID   int32
	MemoName   string
	TicketID   int32
	TicketName string
	Snippet    string
	Timestamp  time.Time
}

// NotificationHub manages SSE connections for real-time notifications
type NotificationHub struct {
	mu          sync.RWMutex
	connections map[int32][]*sseConnection // userID -> active SSE connections
}

type sseConnection struct {
	sse    *datastar.ServerSentEventGenerator
	done   chan struct{}
	userID int32
}

// Global notification hub instance
var notificationHub = &NotificationHub{
	connections: make(map[int32][]*sseConnection),
}

// GetNotificationHub returns the global notification hub
func GetNotificationHub() *NotificationHub {
	return notificationHub
}

// Register adds a new SSE connection for a user
func (h *NotificationHub) Register(userID int32, sse *datastar.ServerSentEventGenerator) *sseConnection {
	h.mu.Lock()
	defer h.mu.Unlock()

	slog.Info("SSE: Registering connection", "userID", userID)
	conn := &sseConnection{
		sse:    sse,
		done:   make(chan struct{}),
		userID: userID,
	}
	h.connections[userID] = append(h.connections[userID], conn)
	return conn
}

// Unregister removes an SSE connection
func (h *NotificationHub) Unregister(conn *sseConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	slog.Info("SSE: Unregistering connection", "userID", conn.userID)
	connections := h.connections[conn.userID]
	for i, c := range connections {
		if c == conn {
			// Remove this connection by replacing with last and truncating
			connections[i] = connections[len(connections)-1]
			h.connections[conn.userID] = connections[:len(connections)-1]
			close(conn.done)
			break
		}
	}
}

// NotifyUser sends a notification to all connections for a user
func (h *NotificationHub) NotifyUser(userID int32, notification Notification) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	connections := h.connections[userID]
	slog.Info("SSE: Notifying user", "userID", userID, "activeConnections", len(connections))
	if len(connections) == 0 {
		return
	}

	// Render notification HTML
	notificationHTML := renderNotificationHTML(notification)
	toastHTML := renderToastHTML(notification)

	for _, conn := range connections {
		// Append new notification to the notification list (if element exists)
		// Using PatchElements with selector option
		conn.sse.PatchElements(notificationHTML, datastar.WithSelectorID("sse-notification-list"))

		// Show toast popup (top-right) - append to toast container
		conn.sse.PatchElements(toastHTML, datastar.WithSelectorID("sse-toast-container"))

		// Update signals to indicate new notification
		conn.sse.MarshalAndPatchSignals(map[string]any{
			"hasNewNotification": true,
		})
	}
}

// GetConnectionCount returns the number of active connections for a user (for testing)
func (h *NotificationHub) GetConnectionCount(userID int32) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections[userID])
}

// renderNotificationHTML generates HTML for a notification list item
func renderNotificationHTML(n Notification) string {
	var icon, message string

	switch n.Type {
	case "MEMO_COMMENT":
		icon = "💬"
		message = fmt.Sprintf("<b>%s</b> mentioned you in a memo", html.EscapeString(n.SenderName))
	case "TICKET_COMMENT":
		icon = "🎫"
		message = fmt.Sprintf("<b>%s</b> mentioned you in ticket #%d", html.EscapeString(n.SenderName), n.TicketID)
	default:
		icon = "🔔"
		message = fmt.Sprintf("<b>%s</b> sent you a notification", html.EscapeString(n.SenderName))
	}

	return fmt.Sprintf(`
		<div id="notification-%d" class="sse-notification-item unread" onclick="window.location='/notifications'">
			<div class="sse-notification-icon">%s</div>
			<div class="sse-notification-content">
				<p class="sse-notification-message">%s</p>
				<span class="sse-notification-time">%s</span>
			</div>
		</div>
	`, n.ID, icon, message, n.Timestamp.Format("3:04 PM"))
}

// renderToastHTML generates HTML for a toast popup notification
func renderToastHTML(n Notification) string {
	var message string

	switch n.Type {
	case "MEMO_COMMENT":
		message = fmt.Sprintf("%s mentioned you in a memo", html.EscapeString(n.SenderName))
	case "TICKET_COMMENT":
		message = fmt.Sprintf("%s mentioned you in ticket #%d", html.EscapeString(n.SenderName), n.TicketID)
	default:
		message = fmt.Sprintf("%s sent you a notification", html.EscapeString(n.SenderName))
	}

	// Simple toast popup (top-right)
	return fmt.Sprintf(`
		<div id="notification-toast-%d" class="sse-notification-toast">
			<div class="sse-toast-icon">🔔</div>
			<div class="sse-toast-content">%s</div>
			<button class="sse-toast-close" onclick="this.parentElement.remove()">×</button>
		</div>
	`, n.ID, message)
}

// NotificationStreamHandler handles SSE connections for real-time notifications
func (s *APIV1Service) NotificationStreamHandler(w http.ResponseWriter, r *http.Request, userID int32) {

	// Create SSE writer (returns *ServerSentEventGenerator)
	sse := datastar.NewSSE(w, r)

	// Register connection
	conn := notificationHub.Register(userID, sse)
	defer notificationHub.Unregister(conn)

	// Send initial connection confirmation signal
	sse.MarshalAndPatchSignals(map[string]any{
		"sseConnected":       true,
		"hasNewNotification": false,
	})

	// Keep connection alive until client disconnects
	<-r.Context().Done()
}
