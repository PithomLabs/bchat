package teststore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

func TestAgentLeadStoreLifecycle(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	tenant := createBridgeTenant(t, ctx, ts, "lead-lifecycle")

	lead, err := ts.UpsertAgentLead(ctx, &store.AgentLead{
		TenantID:      tenant.ID,
		SessionID:     "session-1",
		Name:          "Ada Lovelace",
		Email:         "ada@example.org",
		Topic:         "Pricing question",
		Status:        "new",
		LastMessageAt: time.Now(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, lead.ID)
	require.Equal(t, tenant.ID, lead.TenantID)
	require.Equal(t, "Ada Lovelace", lead.Name)
	require.Equal(t, "ada@example.org", lead.Email)

	fetched, err := ts.GetAgentLead(ctx, &store.FindAgentLead{ID: &lead.ID, TenantID: &tenant.ID})
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, lead.ID, fetched.ID)

	updated, err := ts.UpdateAgentLeadStatus(ctx, tenant.ID, lead.ID, "contacted", nil)
	require.NoError(t, err)
	require.Equal(t, "contacted", updated.Status)

	convertedAt := time.Now()
	converted, err := ts.UpdateAgentLeadStatus(ctx, tenant.ID, lead.ID, "converted", &convertedAt)
	require.NoError(t, err)
	require.Equal(t, "converted", converted.Status)
	require.NotNil(t, converted.ConvertedAt)
}

func TestAgentLeadUpsertKeepsOneLeadPerTenantSession(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	tenant := createBridgeTenant(t, ctx, ts, "lead-upsert")

	first, err := ts.UpsertAgentLead(ctx, &store.AgentLead{
		TenantID:  tenant.ID,
		SessionID: "session-1",
		Name:      "Grace Hopper",
		Phone:     "415-555-1212",
		Topic:     "Initial question",
	})
	require.NoError(t, err)

	second, err := ts.UpsertAgentLead(ctx, &store.AgentLead{
		TenantID:  tenant.ID,
		SessionID: "session-1",
		Name:      "Grace Hopper",
		Email:     "grace@example.org",
		Topic:     "Updated question",
	})
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, "415-555-1212", second.Phone)
	require.Equal(t, "grace@example.org", second.Email)
	require.Equal(t, "Updated question", second.Topic)

	leads, err := ts.ListAgentLeads(ctx, &store.FindAgentLead{TenantID: &tenant.ID})
	require.NoError(t, err)
	require.Len(t, leads, 1)
}

func TestAgentLeadTenantIsolation(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	tenantA := createBridgeTenant(t, ctx, ts, "lead-tenant-a")
	tenantB := createBridgeTenant(t, ctx, ts, "lead-tenant-b")

	lead, err := ts.UpsertAgentLead(ctx, &store.AgentLead{
		TenantID:  tenantA.ID,
		SessionID: "shared-session",
		Name:      "Tenant A Lead",
		Email:     "a@example.org",
	})
	require.NoError(t, err)

	otherTenantLead, err := ts.GetAgentLead(ctx, &store.FindAgentLead{ID: &lead.ID, TenantID: &tenantB.ID})
	require.NoError(t, err)
	require.Nil(t, otherTenantLead)

	leadsB, err := ts.ListAgentLeads(ctx, &store.FindAgentLead{TenantID: &tenantB.ID})
	require.NoError(t, err)
	require.Empty(t, leadsB)
}

func TestAgentLeadRequiresContactMethod(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)
	defer ts.Close()
	tenant := createBridgeTenant(t, ctx, ts, "lead-contact-required")

	_, err := ts.UpsertAgentLead(ctx, &store.AgentLead{
		TenantID:  tenant.ID,
		SessionID: "session-1",
		Name:      "No Contact",
	})
	require.Error(t, err)
}
