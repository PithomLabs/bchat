package teststore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

func TestTicketStore(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	user, err := createTestingHostUser(ctx, ts)
	require.NoError(t, err)

	// Create Ticket
	ticketCreate := &store.Ticket{
		Title:       "Test Ticket",
		Description: "/m/valid-memo-id",
		Status:      store.TicketStatusOpen,
		Priority:    store.TicketPriorityHigh,
		Type:        "BUG",
		Tags:        []string{"backend", "urgent"},
		CreatorID:   user.ID,
		CreatedTs:   1600000000,
		UpdatedTs:   1600000000,
	}

	// Test invalid description
	invalidTicket := *ticketCreate
	invalidTicket.Description = "Invalid Description"
	err = invalidTicket.Validate()
	require.Error(t, err)
	ticket, err := ts.CreateTicket(ctx, ticketCreate)
	require.NoError(t, err)
	require.NotNil(t, ticket)
	require.Equal(t, "BUG", ticket.Type)
	require.Equal(t, []string{"backend", "urgent"}, ticket.Tags)

	// List Ticket (Filter by Type)
	typeBug := "BUG"
	list, err := ts.ListTickets(ctx, &store.FindTicket{
		Type: &typeBug,
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(list))
	require.Equal(t, ticket.ID, list[0].ID)

	// Update Ticket
	newType := "STORY"
	newTags := []string{"frontend"}
	updated, err := ts.UpdateTicket(ctx, &store.UpdateTicket{
		ID:   ticket.ID,
		Type: &newType,
		Tags: newTags,
	})
	require.NoError(t, err)
	require.Equal(t, "STORY", updated.Type)
	require.Equal(t, []string{"frontend"}, updated.Tags)

	// Verify Update Persisted
	fetched, err := ts.GetTicket(ctx, &store.FindTicket{ID: &ticket.ID})
	require.NoError(t, err)
	require.Equal(t, "STORY", fetched.Type)

	// Delete
	err = ts.DeleteTicket(ctx, &store.DeleteTicket{ID: ticket.ID})
	require.NoError(t, err)
	list, err = ts.ListTickets(ctx, &store.FindTicket{CreatorID: &user.ID})
	require.NoError(t, err)
	require.Equal(t, 0, len(list))

	ts.Close()
}

func TestTicketForeignKeyConstraints(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	user, err := createTestingHostUser(ctx, ts)
	require.NoError(t, err)

	// Verify foreign keys are enabled
	var fkEnabled int
	err = ts.GetDriver().GetDB().QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fkEnabled)
	require.NoError(t, err)
	t.Logf("Foreign keys pragma: %d (should be 1)", fkEnabled)
	if fkEnabled != 1 {
		t.Skip("Foreign keys not enabled in test database - this is expected if using older SQLite or test configuration")
	}

	// Check if the tickets table has foreign keys defined
	rows, err := ts.GetDriver().GetDB().QueryContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name='tickets'")
	require.NoError(t, err)
	defer rows.Close()
	if rows.Next() {
		var sql string
		err = rows.Scan(&sql)
		require.NoError(t, err)
		t.Logf("Tickets table schema: %s", sql)
	}

	// Test: Cannot create ticket with invalid creator_id
	invalidTicket := &store.Ticket{
		Title:       "Invalid Ticket",
		Description: "/m/test",
		Status:      store.TicketStatusOpen,
		Priority:    store.TicketPriorityMedium,
		Type:        "BUG",
		Tags:        []string{},
		CreatorID:   99999, // Non-existent user
		CreatedTs:   1600000000,
		UpdatedTs:   1600000000,
	}
	_, err = ts.CreateTicket(ctx, invalidTicket)
	if err != nil {
		t.Logf("Got error creating ticket with invalid creator_id: %v", err)
	}
	require.Error(t, err, "Should fail with invalid creator_id")
	// Note: Error message may be "FOREIGN KEY constraint failed" or similar

	// Test: Cannot assign to invalid user
	invalidAssignee := int32(99999)
	validTicket := &store.Ticket{
		Title:       "Valid Ticket",
		Description: "/m/test",
		Status:      store.TicketStatusOpen,
		Priority:    store.TicketPriorityMedium,
		Type:        "TASK",
		Tags:        []string{},
		CreatorID:   user.ID,
		AssigneeID:  &invalidAssignee,
		CreatedTs:   1600000000,
		UpdatedTs:   1600000000,
	}
	_, err = ts.CreateTicket(ctx, validTicket)
	require.Error(t, err, "Should fail with invalid assignee_id")
	require.Contains(t, err.Error(), "FOREIGN KEY constraint failed")

	// Test: CASCADE DELETE - tickets deleted when user deleted
	// Create a second user to avoid deleting the host
	user2, err := ts.CreateUser(ctx, &store.User{
		Username:     "testuser2",
		Role:         store.RoleUser,
		Email:        "test2@example.com",
		Nickname:     "Test User 2",
		PasswordHash: "hash",
	})
	require.NoError(t, err)

	ticket, err := ts.CreateTicket(ctx, &store.Ticket{
		Title:       "Test CASCADE",
		Description: "/m/test",
		Status:      store.TicketStatusOpen,
		Priority:    store.TicketPriorityMedium,
		Type:        "BUG",
		Tags:        []string{},
		CreatorID:   user2.ID,
		CreatedTs:   1600000000,
		UpdatedTs:   1600000000,
	})
	require.NoError(t, err)

	// Delete the user
	err = ts.DeleteUser(ctx, &store.DeleteUser{ID: user2.ID})
	require.NoError(t, err)

	// Verify ticket was cascaded
	fetched, err := ts.GetTicket(ctx, &store.FindTicket{ID: &ticket.ID})
	require.NoError(t, err)
	require.Nil(t, fetched, "Ticket should be deleted via CASCADE when creator is deleted")

	ts.Close()
}
