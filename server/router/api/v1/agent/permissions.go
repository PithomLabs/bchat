package agent

import "strings"

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

// PermissionPresets defines common permission combinations
var PermissionPresets = map[string][]string{
	"viewer":       {PermTenantRead},
	"tester":       {PermTenantRead, PermChatTest},
	"analyst":      {PermTenantRead, PermChatLogs},
	"editor":       {PermTenantRead, PermTenantWrite, PermFilesUpload},
	"tenant_admin": {PermTenantAdmin},
}

// ContainsPermission checks if a permission list contains the required permission
func ContainsPermission(permissions []string, required string) bool {
	for _, p := range permissions {
		if p == PermWildcard || p == required {
			return true
		}
		// Handle wildcard patterns like "tenant:*"
		if strings.HasSuffix(p, ":*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
		// tenant:admin implies all tenant permissions
		if p == PermTenantAdmin && strings.HasPrefix(required, "tenant:") {
			return true
		}
	}
	return false
}

// ValidatePermissions checks if all permissions are valid
func ValidatePermissions(permissions []string) bool {
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
