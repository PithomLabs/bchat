package postgres

import (
	"context"

	"github.com/usememos/memos/store"
)

// Stub implementations for RBAC to satisfy Driver interface
// Postgres support can be added later if needed

func (d *DB) CreateUserTenantPermission(ctx context.Context, perm *store.UserTenantPermission) (*store.UserTenantPermission, error) {
	return nil, nil
}

func (d *DB) GetUserTenantPermission(ctx context.Context, find *store.FindUserTenantPermission) (*store.UserTenantPermission, error) {
	return nil, nil
}

func (d *DB) ListUserTenantPermissions(ctx context.Context, find *store.FindUserTenantPermission) ([]*store.UserTenantPermission, error) {
	return nil, nil
}

func (d *DB) UpdateUserTenantPermission(ctx context.Context, perm *store.UserTenantPermission) (*store.UserTenantPermission, error) {
	return nil, nil
}

func (d *DB) DeleteUserTenantPermission(ctx context.Context, userID, tenantID int32) error {
	return nil
}

func (d *DB) GetTenantConfig(ctx context.Context, find *store.FindTenantConfig) (*store.TenantConfig, error) {
	return nil, nil
}

func (d *DB) UpsertTenantConfig(ctx context.Context, config *store.TenantConfig) (*store.TenantConfig, error) {
	return nil, nil
}

func (d *DB) DeleteTenantConfig(ctx context.Context, tenantID int32) error {
	return nil
}

func (d *DB) GetSystemSecret(ctx context.Context) (*store.SystemSecret, error) {
	return nil, nil
}

func (d *DB) UpsertSystemSecret(ctx context.Context, secret *store.SystemSecret) (*store.SystemSecret, error) {
	return nil, nil
}
