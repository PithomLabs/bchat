package teststore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
)

func TestTenantRoleTemplateCRUD(t *testing.T) {
	ctx := context.Background()
	ts := NewTestingStore(ctx, t)

	tenant := &store.AgentTenant{
		Slug:        "template-crud-tenant",
		CompanyName: "Template CRUD Company",
		IsActive:    true,
	}
	tenant, err := ts.CreateAgentTenant(ctx, tenant)
	require.NoError(t, err)
	require.NotZero(t, tenant.ID)

	user := &store.User{
		Username: "template-test-user",
		Role:     store.RoleUser,
	}
	user, err = ts.CreateUser(ctx, user)
	require.NoError(t, err)
	require.NotZero(t, user.ID)

	t.Run("system templates are seeded", func(t *testing.T) {
		templates, err := ts.ListTenantRoleTemplates(ctx, &store.FindTenantRoleTemplate{})
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(templates), 5)

		codes := map[string]bool{}
		for _, tmpl := range templates {
			codes[tmpl.Code] = true
		}
		require.True(t, codes["viewer"])
		require.True(t, codes["tenant_admin"])
	})

	t.Run("create custom template", func(t *testing.T) {
		template := &store.TenantRoleTemplate{
			TenantID:    &tenant.ID,
			Name:        "Custom Viewer",
			Code:        "custom_viewer",
			Permissions: []string{"tenant:read", "chat:logs"},
		}
		created, err := ts.CreateTenantRoleTemplate(ctx, template)
		require.NoError(t, err)
		require.NotZero(t, created.ID)
		require.Equal(t, tenant.ID, *created.TenantID)
	})

	t.Run("get custom template", func(t *testing.T) {
		template, err := ts.GetTenantRoleTemplate(ctx, &store.FindTenantRoleTemplate{
			TenantID: &tenant.ID,
			Code:     strPtr("custom_viewer"),
		})
		require.NoError(t, err)
		require.NotNil(t, template)
		require.Equal(t, "Custom Viewer", template.Name)
	})

	t.Run("update custom template", func(t *testing.T) {
		template, err := ts.GetTenantRoleTemplate(ctx, &store.FindTenantRoleTemplate{
			TenantID: &tenant.ID,
			Code:     strPtr("custom_viewer"),
		})
		require.NoError(t, err)

		newName := "Updated Custom Viewer"
		updated, err := ts.UpdateTenantRoleTemplate(ctx, &store.TenantRoleTemplate{
			ID:          template.ID,
			Name:        newName,
			Code:        template.Code,
			Permissions: template.Permissions,
		})
		require.NoError(t, err)
		require.Equal(t, newName, updated.Name)
	})

	t.Run("delete custom template blocks when assignments exist", func(t *testing.T) {
		template, err := ts.GetTenantRoleTemplate(ctx, &store.FindTenantRoleTemplate{
			TenantID: &tenant.ID,
			Code:     strPtr("custom_viewer"),
		})
		require.NoError(t, err)

		_, err = ts.CreateUserTenantPermission(ctx, &store.UserTenantPermission{
			UserID:          user.ID,
			TenantID:        tenant.ID,
			Permissions:     template.Permissions,
			SourceTemplateID: &template.ID,
		})
		require.NoError(t, err)

		err = ts.DeleteTenantRoleTemplate(ctx, template.ID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "active assignments")
	})

	t.Run("delete custom template succeeds when no assignments", func(t *testing.T) {
		template, err := ts.GetTenantRoleTemplate(ctx, &store.FindTenantRoleTemplate{
			TenantID: &tenant.ID,
			Code:     strPtr("custom_viewer"),
		})
		require.NoError(t, err)

		ts.DeleteAllUserTenantPermissions(ctx, user.ID, tenant.ID)

		err = ts.DeleteTenantRoleTemplate(ctx, template.ID)
		require.NoError(t, err)
	})
}

func intPtr(v int32) *int32 {
	return &v
}
