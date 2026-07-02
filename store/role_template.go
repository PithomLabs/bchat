package store

import "time"

type TenantRoleTemplate struct {
	ID          int32
	TenantID    *int32
	Name        string
	Code        string
	Permissions []string
	CreatedBy   *int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type FindTenantRoleTemplate struct {
	ID       *int32
	TenantID *int32
	Code     *string
	Name     *string
}

type CreateTenantRoleTemplateRequest struct {
	TenantID    *int32
	Name        string
	Code        string
	Permissions []string
	CreatedBy   *int32
}

type UpdateTenantRoleTemplateRequest struct {
	ID          int32
	Name        *string
	Code        *string
	Permissions *[]string
}

type TenantRoleTemplateResponse struct {
	ID          int32
	TenantID    *int32
	Name        string
	Code        string
	Permissions []string
	CreatedBy   *int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
