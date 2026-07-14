package agent

import (
	"context"
	"strings"

	"github.com/usememos/memos/store"
)

// Permission constants
const (
	PermTenantAdmin  = "tenant:admin"
	PermTenantRead   = "tenant:read"
	PermTenantWrite  = "tenant:write"
	PermChatTest     = "chat:test"
	PermChatLogs     = "chat:logs"
	PermFilesUpload  = "files:upload"
	PermFilesRestore = "files:restore"
	PermAPIConfig    = "api:config"
	PermWildcard     = "*"
)

// AllPermissions lists all valid permissions
var AllPermissions = []string{
	PermTenantAdmin, PermTenantRead, PermTenantWrite,
	PermChatTest, PermChatLogs,
	PermFilesUpload, PermFilesRestore, PermAPIConfig,
}

// SourceGlobalRole is the permission source for global role grants.
const SourceGlobalRole = "global_role"

// SourceTenantTemplate is the permission source for tenant template assignments.
const SourceTenantTemplate = "tenant_template"

// SourceExplicit is the permission source for explicit per-user grants.
const SourceExplicit = "explicit"

// PermissionPresets defines common permission combinations
var PermissionPresets = map[string][]string{
	"viewer":       {PermTenantRead},
	"tester":       {PermTenantRead, PermChatTest},
	"analyst":      {PermTenantRead, PermChatLogs},
	"editor":       {PermTenantRead, PermTenantWrite, PermFilesUpload},
	"tenant_admin": {PermTenantAdmin},
}

// SystemRoleTemplates maps system template codes to their permission sets.
var SystemRoleTemplates = PermissionPresets

// ResolvedPermission represents a single permission with its source metadata.
type ResolvedPermission struct {
	Permission   string  `json:"permission"`
	Source       string  `json:"source"`
	TemplateID   *int32  `json:"template_id,omitempty"`
	TemplateName *string `json:"template_name,omitempty"`
}

// ExplicitGrantSourceTemplate is a typed sentinel used to filter explicitly granted
// permissions (source_template_id IS NULL). Auto-increment template IDs start at 1.
const ExplicitGrantSourceTemplate = int32(0)

// ContainsPermission checks if a permission list contains the required permission.
// Deprecated: prefer containsResolvedPermission for new code.
func ContainsPermission(permissions []string, required string) bool {
	for _, p := range permissions {
		if p == PermWildcard || p == required {
			return true
		}
		if strings.HasSuffix(p, ":*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
		// tenant:admin grants all permissions for this tenant (superset of tenant:*, chat:*, files:*, api:*).
		if p == PermTenantAdmin {
			return true
		}
	}
	return false
}

func containsResolvedPermission(permissions []ResolvedPermission, required string) bool {
	for _, p := range permissions {
		if p.Permission == PermWildcard || p.Permission == required {
			return true
		}
		if strings.HasSuffix(p.Permission, ":*") {
			prefix := strings.TrimSuffix(p.Permission, "*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
		// tenant:admin grants all permissions for this tenant (superset of tenant:*, chat:*, files:*, api:*).
		if p.Permission == PermTenantAdmin {
			return true
		}
	}
	return false
}

// ValidatePermissions checks if all permissions are valid.
// Rejects templates whose sole permission is the wildcard "*".
func ValidatePermissions(permissions []string) bool {
	if len(permissions) == 1 && permissions[0] == PermWildcard {
		return false
	}
	for _, p := range permissions {
		if p == PermWildcard {
			continue
		}
		if strings.HasSuffix(p, ":*") {
			continue
		}
		valid := false
		for _, ap := range AllPermissions {
			if p == ap {
				valid = true
				break
			}
		}
		if !valid {
			return false
		}
	}
	return true
}

func getSystemRoleTemplate(code string) ([]string, bool) {
	perms, ok := SystemRoleTemplates[code]
	return perms, ok
}

// ResolveEffectivePermissions returns the effective permissions for a user on a tenant.
// For HOST it returns wildcard; for ADMIN it returns tenant:read and api:config;
// for other roles it unions all UserTenantPermission rows for the user/tenant.
func ResolveEffectivePermissions(ctx context.Context, s *store.Store, tenantID, userID int32) ([]ResolvedPermission, error) {
	user, err := s.GetUser(ctx, &store.FindUser{ID: &userID})
	if err != nil || user == nil {
		return nil, nil
	}

	if user.Role == store.RoleHost {
		return []ResolvedPermission{{Permission: PermWildcard, Source: SourceGlobalRole}}, nil
	}
	if user.Role == store.RoleAdmin {
		return []ResolvedPermission{
			{Permission: PermTenantRead, Source: SourceGlobalRole},
			{Permission: PermAPIConfig, Source: SourceGlobalRole},
		}, nil
	}

	perms, err := s.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{
		UserID:   &userID,
		TenantID: &tenantID,
	})
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var resolved []ResolvedPermission
	for _, perm := range perms {
		for _, p := range perm.Permissions {
			if !seen[p] {
				source := SourceExplicit
				var templateID *int32
				var templateName *string
				if perm.SourceTemplateID != nil {
					source = SourceTenantTemplate
					templateID = perm.SourceTemplateID
					tmpl, err := s.GetTenantRoleTemplate(ctx, &store.FindTenantRoleTemplate{ID: templateID})
					if err == nil && tmpl != nil {
						name := tmpl.Name
						templateName = &name
					}
				}
				resolved = append(resolved, ResolvedPermission{Permission: p, Source: source, TemplateID: templateID, TemplateName: templateName})
				seen[p] = true
			}
		}
	}
	return resolved, nil
}
