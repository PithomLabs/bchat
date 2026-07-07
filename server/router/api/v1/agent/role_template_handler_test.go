package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
	teststore "github.com/usememos/memos/store/test"
)

func setupRoleTemplateTestStore(t *testing.T) (context.Context, *store.Store, *profile.Profile, *store.AgentTenant, *store.User) {
	ctx := context.Background()
	ts := teststore.NewTestingStore(ctx, t)

	tenant, err := ts.CreateAgentTenant(ctx, &store.AgentTenant{
		Slug:        "role-template-test-tenant",
		CompanyName: "Role Template Test",
		IsActive:    true,
	})
	require.NoError(t, err)

	user, err := ts.CreateUser(ctx, &store.User{
		Username: "role-template-user",
		Role:     store.RoleUser,
	})
	require.NoError(t, err)

	_ = ts
	return ctx, ts, nil, tenant, user
}

func TestRoleTemplateEndpoints(t *testing.T) {
	ctx, ts, _, tenant, user := setupRoleTemplateTestStore(t)
	defer ts.Close()

	// Create a separate admin user for managing templates (not the target of assignments)
	adminUser, err := ts.CreateUser(ctx, &store.User{
		Username: "role-template-admin",
		Role:     store.RoleUser,
	})
	require.NoError(t, err)

	_, err = ts.CreateUserTenantPermission(ctx, &store.UserTenantPermission{
		UserID:      adminUser.ID,
		TenantID:    tenant.ID,
		Permissions: []string{"tenant:admin"},
	})
	require.NoError(t, err)

	prof := &profile.Profile{
		Mode: "dev",
	}
	svc := NewService(ts, prof)
	handler := NewHandler(svc, ts)

	e := echo.New()
	e.GET("/api/v1/agent/:slug/role-templates", handler.HandleListRoleTemplates)
	e.POST("/api/v1/agent/:slug/role-templates", handler.HandleCreateRoleTemplate)
	e.PATCH("/api/v1/agent/role-templates/:id", handler.HandleUpdateRoleTemplate)
	e.DELETE("/api/v1/agent/role-templates/:id", handler.HandleDeleteRoleTemplate)
	e.POST("/api/v1/agent/:slug/role-templates/:id/assign", handler.HandleAssignRoleTemplate)
	e.GET("/api/v1/agent/:slug/users/:userId/roles", handler.HandleListUserRoles)

	require := require.New(t)

	t.Run("list templates includes seeded templates", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/"+tenant.Slug+"/role-templates", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("slug")
		c.SetParamValues(tenant.Slug)
		c.Set("user-id", adminUser.ID)

		err := handler.HandleListRoleTemplates(c)
		require.NoError(err)
		require.Equal(http.StatusOK, rec.Code)

		var body map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&body)
		templates, ok := body["templates"].([]interface{})
		require.True(ok, "response should contain templates array")
		require.GreaterOrEqual(len(templates), 1)
	})

	t.Run("list templates hides permissions from non-admin", func(t *testing.T) {
		regularUser, err := ts.CreateUser(ctx, &store.User{
			Username: "regular-viewer",
			Role:     store.RoleUser,
		})
		require.NoError(err)

		_, err = ts.CreateUserTenantPermission(ctx, &store.UserTenantPermission{
			UserID:      regularUser.ID,
			TenantID:    tenant.ID,
			Permissions: []string{"tenant:read"},
		})
		require.NoError(err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/"+tenant.Slug+"/role-templates", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("slug")
		c.SetParamValues(tenant.Slug)
		c.Set("user-id", regularUser.ID)

		err = handler.HandleListRoleTemplates(c)
		require.NoError(err)
		require.Equal(http.StatusOK, rec.Code)

		var body map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&body)
		templates, ok := body["templates"].([]interface{})
		require.True(ok)
		require.GreaterOrEqual(len(templates), 1)
		for _, t := range templates {
			tmpl := t.(map[string]interface{})
			_, hasPerms := tmpl["permissions"]
			require.False(hasPerms, "non-admin should not see permission lists")
		}
	})

	t.Run("create template", func(t *testing.T) {
		reqBody := []byte(`{"name":"Custom Support","code":"custom_support","permissions":["tenant:read","chat:logs"]}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/"+tenant.Slug+"/role-templates", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("slug")
		c.SetParamValues(tenant.Slug)
		c.Set("user-id", adminUser.ID)

		err := handler.HandleCreateRoleTemplate(c)
		require.NoError(err)
		require.Equal(http.StatusCreated, rec.Code)

		var created store.TenantRoleTemplate
		json.NewDecoder(rec.Body).Decode(&created)
		require.Equal("Custom Support", created.Name)
		require.Equal("custom_support", created.Code)
		require.Equal([]string{"tenant:read", "chat:logs"}, created.Permissions)
	})

	t.Run("assign template", func(t *testing.T) {
		reqBody := []byte(`{"user_id":` + strconv.Itoa(int(user.ID)) + `}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/"+tenant.Slug+"/role-templates/1/assign", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("slug", "id")
		c.SetParamValues(tenant.Slug, "1")
		c.Set("user-id", adminUser.ID)

		err := handler.HandleAssignRoleTemplate(c)
		require.NoError(err)
		require.Equal(http.StatusCreated, rec.Code)

		var body map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&body)
		created, ok := body["created"].(bool)
		require.True(ok)
		require.True(created)
	})

	t.Run("assign template idempotent", func(t *testing.T) {
		reqBody := []byte(`{"user_id":` + strconv.Itoa(int(user.ID)) + `}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/"+tenant.Slug+"/role-templates/1/assign", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("slug", "id")
		c.SetParamValues(tenant.Slug, "1")
		c.Set("user-id", adminUser.ID)

		err := handler.HandleAssignRoleTemplate(c)
		require.NoError(err)
		require.Equal(http.StatusOK, rec.Code)

		var body map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&body)
		created, ok := body["created"].(bool)
		require.True(ok)
		require.False(created)
	})

	t.Run("list user roles", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/"+tenant.Slug+"/users/"+strconv.Itoa(int(user.ID))+"/roles", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("slug", "userId")
		c.SetParamValues(tenant.Slug, strconv.Itoa(int(user.ID)))
		c.Set("user-id", adminUser.ID)

		err := handler.HandleListUserRoles(c)
		require.NoError(err)
		require.Equal(http.StatusOK, rec.Code)

		var body map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&body)
		require.Equal(float64(user.ID), body["user_id"])
		perms, ok := body["permissions"].([]interface{})
		require.True(ok)
		require.GreaterOrEqual(len(perms), 1)
	})

	t.Run("tenant_admin_sees_system_template_contents", func(t *testing.T) {
		tenantAdmin, err := ts.CreateUser(ctx, &store.User{
			Username: "tenant-admin-user",
			Role:     store.RoleUser,
		})
		require.NoError(err)

		_, err = ts.CreateUserTenantPermission(ctx, &store.UserTenantPermission{
			UserID:      tenantAdmin.ID,
			TenantID:    tenant.ID,
			Permissions: []string{"tenant:admin"},
		})
		require.NoError(err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/"+tenant.Slug+"/role-templates", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("slug")
		c.SetParamValues(tenant.Slug)
		c.Set("user-id", tenantAdmin.ID)

		err = handler.HandleListRoleTemplates(c)
		require.NoError(err)
		require.Equal(http.StatusOK, rec.Code)

		var body map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&body)
		templates, ok := body["templates"].([]interface{})
		require.True(ok)
		require.GreaterOrEqual(len(templates), 1)

		hasPerms := false
		for _, t := range templates {
			tmpl := t.(map[string]interface{})
			if _, ok := tmpl["permissions"]; ok {
				hasPerms = true
				break
			}
		}
		require.True(hasPerms, "tenant:admin user should see system template permissions")
	})

	t.Run("revoke_preserves_template_assignments", func(t *testing.T) {
		viewerUser, err := ts.CreateUser(ctx, &store.User{
			Username: "template-viewer",
			Role:     store.RoleUser,
		})
		require.NoError(err)

		templates, err := ts.ListTenantRoleTemplates(ctx, &store.FindTenantRoleTemplate{})
		require.NoError(err)
		require.GreaterOrEqual(len(templates), 1)

		viewerTemplate := templates[0]

		// Create a template-based permission (the only row for this user+tenant)
		_, err = ts.CreateUserTenantPermission(ctx, &store.UserTenantPermission{
			UserID:          viewerUser.ID,
			TenantID:        tenant.ID,
			Permissions:     viewerTemplate.Permissions,
			SourceTemplateID: intPtr(viewerTemplate.ID),
		})
		require.NoError(err)

		// Revoke should delete the explicit row (source_template_id IS NULL)
		// Since our row has source_template_id set, it should be preserved
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/agent/"+tenant.Slug+"/permissions/"+strconv.Itoa(int(viewerUser.ID)), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("slug", "userId")
		c.SetParamValues(tenant.Slug, strconv.Itoa(int(viewerUser.ID)))
		c.Set("user-id", adminUser.ID)

		err = handler.HandleRevokePermission(c)
		require.NoError(err)
		require.Equal(http.StatusOK, rec.Code)

		// Template-based permission should still exist
		perms, err := ts.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{
			UserID:   &viewerUser.ID,
			TenantID: &tenant.ID,
		})
		require.NoError(err)
		require.Len(perms, 1)
		require.NotNil(perms[0].SourceTemplateID)
		require.Equal(viewerTemplate.ID, *perms[0].SourceTemplateID)

		resolved, err := ResolveEffectivePermissions(ctx, ts, tenant.ID, viewerUser.ID)
		require.NoError(err)
		require.Len(resolved, len(viewerTemplate.Permissions))
	})

	t.Run("list_user_roles_includes_template_identity", func(t *testing.T) {
		templateUser, err := ts.CreateUser(ctx, &store.User{
			Username: "template-identity-user",
			Role:     store.RoleUser,
		})
		require.NoError(err)

		templates, err := ts.ListTenantRoleTemplates(ctx, &store.FindTenantRoleTemplate{})
		require.NoError(err)
		require.GreaterOrEqual(len(templates), 1)

		viewerTemplate := templates[0]

		_, err = ts.CreateUserTenantPermission(ctx, &store.UserTenantPermission{
			UserID:          templateUser.ID,
			TenantID:        tenant.ID,
			Permissions:     viewerTemplate.Permissions,
			SourceTemplateID: intPtr(viewerTemplate.ID),
		})
		require.NoError(err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/"+tenant.Slug+"/users/"+strconv.Itoa(int(templateUser.ID))+"/roles", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("slug", "userId")
		c.SetParamValues(tenant.Slug, strconv.Itoa(int(templateUser.ID)))
		c.Set("user-id", adminUser.ID)

		err = handler.HandleListUserRoles(c)
		require.NoError(err)
		require.Equal(http.StatusOK, rec.Code)

		var body map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&body)
		require.Equal(float64(templateUser.ID), body["user_id"])

		perms, ok := body["permissions"].([]interface{})
		require.True(ok)
		require.GreaterOrEqual(len(perms), 1)

		templatePerms := []interface{}{}
		for _, p := range perms {
			pm := p.(map[string]interface{})
			if pm["source"] == "tenant_template" {
				templatePerms = append(templatePerms, pm)
			}
		}
		require.GreaterOrEqual(len(templatePerms), 1)
		for _, p := range templatePerms {
			pm := p.(map[string]interface{})
			require.NotNil(pm["template_id"], "template_id should be present for template-sourced permissions")
			require.Equal(float64(viewerTemplate.ID), pm["template_id"])
			require.Equal(viewerTemplate.Name, pm["template_name"])
		}
	})

	t.Run("grant_deduplicates_orphaned_explicit_rows", func(t *testing.T) {
		orphanUser, err := ts.CreateUser(ctx, &store.User{
			Username: "orphan-user",
			Role:     store.RoleUser,
		})
		require.NoError(err)

		// Create a single explicit permission row
		_, err = ts.CreateUserTenantPermission(ctx, &store.UserTenantPermission{
			UserID:      orphanUser.ID,
			TenantID:    tenant.ID,
			Permissions: []string{"chat:test"},
		})
		require.NoError(err)

		perms, err := ts.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{
			UserID:   &orphanUser.ID,
			TenantID: &tenant.ID,
		})
		require.NoError(err)
		require.Len(perms, 1, "should have 1 explicit row before grant")

		// Grant new permissions — should update existing row, not create a duplicate
		reqBody := []byte(`{"user_id":` + strconv.Itoa(int(orphanUser.ID)) + `,"permissions":["chat:logs"]}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/"+tenant.Slug+"/permissions", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("slug")
		c.SetParamValues(tenant.Slug)
		c.Set("user-id", adminUser.ID)

		err = handler.HandleGrantPermission(c)
		require.NoError(err)
		require.Equal(http.StatusOK, rec.Code)

		perms, err = ts.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{
			UserID:   &orphanUser.ID,
			TenantID: &tenant.ID,
		})
		require.NoError(err)
		require.Len(perms, 1, "should have 1 row after grant (no duplicate)")
		require.Equal([]string{"chat:logs"}, perms[0].Permissions)
	})

}
