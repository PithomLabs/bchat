package v1

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/usememos/memos/internal/profile"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/server/router/api/v1/agent"
	"github.com/usememos/memos/store"
	teststore "github.com/usememos/memos/store/test"
)

// logCaptureHandler captures slog records for test assertions.
type logCaptureHandler struct {
	records []slog.Record
	mu      sync.Mutex
}

func (h *logCaptureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *logCaptureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *logCaptureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *logCaptureHandler) WithGroup(_ string) slog.Handler       { return h }

func (h *logCaptureHandler) messages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var msgs []string
	for _, r := range h.records {
		msgs = append(msgs, r.Message)
	}
	return msgs
}

func (h *logCaptureHandler) warnMessages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var msgs []string
	for _, r := range h.records {
		if r.Level >= slog.LevelWarn {
			msgs = append(msgs, r.Message)
		}
	}
	return msgs
}

func (h *logCaptureHandler) debugMessages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var msgs []string
	for _, r := range h.records {
		if r.Level <= slog.LevelDebug {
			msgs = append(msgs, r.Message)
		}
	}
	return msgs
}

func setupMemoTest(t *testing.T) (*store.Store, *APIV1Service, *store.User, int32) {
	t.Helper()
	ctx := context.Background()
	db := teststore.NewTestingStore(ctx, t)
	s := &APIV1Service{
		Store:  db,
		Secret: "test-secret",
	}
	user, err := db.CreateUser(ctx, &store.User{
		Username: "test-user",
		Nickname: "Test User",
		Role:     store.RoleUser,
	})
	require.NoError(t, err)

	tenant, err := db.CreateAgentTenant(ctx, &store.AgentTenant{
		Slug:        "test-tenant",
		CompanyName: "Test Tenant",
		IsActive:    true,
	})
	require.NoError(t, err)

	return db, s, user, tenant.ID
}

func ctxWithUserAndTenant(username string, tenantID int32) context.Context {
	ctx := context.WithValue(context.Background(), usernameContextKey, username)
	ctx = context.WithValue(ctx, tenantIDContextKey, tenantID)
	return ctx
}

func newAgentService(t *testing.T, db *store.Store) *agent.Service {
	t.Helper()
	svc := agent.NewService(db, &profile.Profile{
		Mode:   "dev",
		Data:   t.TempDir(),
		Driver: "sqlite",
	})
	return svc
}

// --- Error path test: Fix 1 (async goroutine) ---

func TestCreateMemoComment_ListTicketsErrorLogged(t *testing.T) {
	db, s, user, tenantID := setupMemoTest(t)
	ctx := ctxWithUserAndTenant(user.Username, tenantID)

	svc := newAgentService(t, db)
	s.agentHandler = agent.NewHandler(svc, db)

	parentMemo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "err-parent-uid",
		CreatorID:  user.ID,
		Content:    "Error test parent",
		Visibility: store.Private,
		TenantID:   &tenantID,
	})
	require.NoError(t, err)

	_, err = db.CreateTicket(ctx, &store.Ticket{
		Title:       "Error Test Ticket",
		Description: "/m/" + parentMemo.UID,
		Status:      store.TicketStatusOpen,
		Priority:    store.TicketPriorityMedium,
		Type:        "SUPPORT",
		CreatorID:   user.ID,
		TenantID:    &tenantID,
	})
	require.NoError(t, err)

	capture := &logCaptureHandler{}
	old := slog.Default()
	slog.SetDefault(slog.New(capture))
	defer slog.SetDefault(old)

	// Call with open store — method succeeds, goroutine fires.
	_, err = s.CreateMemoComment(ctx, &v1pb.CreateMemoCommentRequest{
		Name: MemoNamePrefix + parentMemo.UID,
		Comment: &v1pb.Memo{
			Content:    "Error test comment",
			Visibility: v1pb.Visibility_PRIVATE,
		},
	})
	require.NoError(t, err)

	// Close store immediately — goroutine's ListTickets should fail.
	require.NoError(t, db.Close())

	time.Sleep(300 * time.Millisecond)

	warnMsgs := capture.warnMessages()
	found := false
	for _, msg := range warnMsgs {
		if msg == "failed to find parent ticket for comment reindex" {
			found = true
			break
		}
	}
	require.True(t, found,
		"expected warn log about ListTickets failure, got: %v", warnMsgs)
}

// --- Happy path tests ---

func TestCreateMemoComment_NoErrorLogs(t *testing.T) {
	db, s, user, tenantID := setupMemoTest(t)
	ctx := ctxWithUserAndTenant(user.Username, tenantID)

	svc := newAgentService(t, db)
	s.agentHandler = agent.NewHandler(svc, db)

	parentMemo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "happy-parent-uid",
		CreatorID:  user.ID,
		Content:    "Happy parent",
		Visibility: store.Private,
		TenantID:   &tenantID,
	})
	require.NoError(t, err)

	_, err = db.CreateTicket(ctx, &store.Ticket{
		Title:       "Happy Ticket",
		Description: "/m/" + parentMemo.UID,
		Status:      store.TicketStatusOpen,
		Priority:    store.TicketPriorityMedium,
		Type:        "SUPPORT",
		CreatorID:   user.ID,
		TenantID:    &tenantID,
	})
	require.NoError(t, err)

	capture := &logCaptureHandler{}
	old := slog.Default()
	slog.SetDefault(slog.New(capture))
	defer slog.SetDefault(old)

	_, err = s.CreateMemoComment(ctx, &v1pb.CreateMemoCommentRequest{
		Name: MemoNamePrefix + parentMemo.UID,
		Comment: &v1pb.Memo{
			Content:    "Happy comment",
			Visibility: v1pb.Visibility_PRIVATE,
		},
	})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	warnMsgs := capture.warnMessages()
	for _, msg := range warnMsgs {
		require.NotContains(t, msg, "failed to find parent ticket")
		require.NotContains(t, msg, "failed to fetch comments for ticket re-index")
		require.NotContains(t, msg, "failed to re-index ticket after comment creation")
	}
}

func TestUpdateMemo_NoErrorLogs(t *testing.T) {
	db, s, user, tenantID := setupMemoTest(t)
	ctx := ctxWithUserAndTenant(user.Username, tenantID)

	svc := newAgentService(t, db)
	s.agentHandler = agent.NewHandler(svc, db)

	parentMemo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "happy-parent-update",
		CreatorID:  user.ID,
		Content:    "Happy parent",
		Visibility: store.Private,
		TenantID:   &tenantID,
	})
	require.NoError(t, err)

	commentMemo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "happy-comment-update",
		CreatorID:  user.ID,
		Content:    "Happy comment original",
		Visibility: store.Private,
		TenantID:   &tenantID,
	})
	require.NoError(t, err)

	_, err = db.UpsertMemoRelation(ctx, &store.MemoRelation{
		MemoID:        commentMemo.ID,
		RelatedMemoID: parentMemo.ID,
		Type:          store.MemoRelationComment,
		TenantID:      &tenantID,
	})
	require.NoError(t, err)

	_, err = db.CreateTicket(ctx, &store.Ticket{
		Title:       "Happy Update Ticket",
		Description: "/m/" + parentMemo.UID,
		Status:      store.TicketStatusOpen,
		Priority:    store.TicketPriorityMedium,
		Type:        "SUPPORT",
		CreatorID:   user.ID,
		TenantID:    &tenantID,
	})
	require.NoError(t, err)

	capture := &logCaptureHandler{}
	old := slog.Default()
	slog.SetDefault(slog.New(capture))
	defer slog.SetDefault(old)

	_, err = s.UpdateMemo(ctx, &v1pb.UpdateMemoRequest{
		Memo: &v1pb.Memo{
			Name:    MemoNamePrefix + commentMemo.UID,
			Content: "Happy comment updated",
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"content"}},
	})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	warnMsgs := capture.warnMessages()
	for _, msg := range warnMsgs {
		require.NotContains(t, msg, "failed to find ticket for comment-edit reindex")
		require.NotContains(t, msg, "failed to fetch comments for ticket re-index after comment edit")
		require.NotContains(t, msg, "failed to re-index ticket after comment edit")
	}
}

// --- Nil handler test: hooks skipped ---

func TestCreateMemoComment_HooksSkippedWithoutAgentHandler(t *testing.T) {
	db, s, user, tenantID := setupMemoTest(t)
	ctx := ctxWithUserAndTenant(user.Username, tenantID)

	parentMemo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "nil-agent-parent",
		CreatorID:  user.ID,
		Content:    "Nil agent parent",
		Visibility: store.Private,
		TenantID:   &tenantID,
	})
	require.NoError(t, err)

	capture := &logCaptureHandler{}
	old := slog.Default()
	slog.SetDefault(slog.New(capture))
	defer slog.SetDefault(old)

	_, err = s.CreateMemoComment(ctx, &v1pb.CreateMemoCommentRequest{
		Name: MemoNamePrefix + parentMemo.UID,
		Comment: &v1pb.Memo{
			Content:    "Comment with nil agent",
			Visibility: v1pb.Visibility_PRIVATE,
		},
	})
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	allMsgs := capture.messages()
	for _, msg := range allMsgs {
		require.NotContains(t, msg, "failed to find parent ticket")
	}
}

func TestUpdateMemo_HooksSkippedWithoutAgentHandler(t *testing.T) {
	db, s, user, tenantID := setupMemoTest(t)
	ctx := ctxWithUserAndTenant(user.Username, tenantID)

	parentMemo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "nil-agent-parent-update",
		CreatorID:  user.ID,
		Content:    "Nil agent parent",
		Visibility: store.Private,
		TenantID:   &tenantID,
	})
	require.NoError(t, err)

	commentMemo, err := db.CreateMemo(ctx, &store.Memo{
		UID:        "nil-agent-comment-update",
		CreatorID:  user.ID,
		Content:    "Original comment",
		Visibility: store.Private,
		TenantID:   &tenantID,
	})
	require.NoError(t, err)

	_, err = db.UpsertMemoRelation(ctx, &store.MemoRelation{
		MemoID:        commentMemo.ID,
		RelatedMemoID: parentMemo.ID,
		Type:          store.MemoRelationComment,
		TenantID:      &tenantID,
	})
	require.NoError(t, err)

	capture := &logCaptureHandler{}
	old := slog.Default()
	slog.SetDefault(slog.New(capture))
	defer slog.SetDefault(old)

	_, err = s.UpdateMemo(ctx, &v1pb.UpdateMemoRequest{
		Memo: &v1pb.Memo{
			Name:    MemoNamePrefix + commentMemo.UID,
			Content: "Updated comment nil agent",
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"content"}},
	})
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	allMsgs := capture.messages()
	for _, msg := range allMsgs {
		require.NotContains(t, msg, "failed to find ticket for comment-edit reindex")
		require.NotContains(t, msg, "failed to load memo relations for comment-edit reindex")
	}
}
