package store

import (
	"context"
	"time"
)

// UserTenantPermission represents a user's permissions for a specific tenant.
type UserTenantPermission struct {
	ID          int32
	UserID      int32
	TenantID    int32
	Permissions []string // Parsed from comma-separated string
	GrantedBy   *int32
	GrantedAt   time.Time
}

// FindUserTenantPermission contains filters for finding permissions.
type FindUserTenantPermission struct {
	ID       *int32
	UserID   *int32
	TenantID *int32
}

// TenantConfig holds tenant-specific LLM configuration.
type TenantConfig struct {
	ID                        int32
	TenantID                  int32
	LLMModel                  string
	SimulationHumanModel      string // Model for human role in agent simulation
	OpenRouterAPIKeyEncrypted []byte
	OpenRouterAPIKeyNonce     []byte
	Features                  map[string]interface{}
	RetrievalMode             string // "long_context" or "rag" - determines KB retrieval strategy
	ContentTokens             int32  // Estimated token count of KB + Policy content
	RecordTranscripts         bool   // Whether to record chat conversation transcripts (default: true)
	UpdatedAt                 time.Time
	UpdatedBy                 *int32
}

// FindTenantConfig contains filters for finding tenant configs.
type FindTenantConfig struct {
	ID       *int32
	TenantID *int32
}

// SystemSecret holds the system encryption salt.
type SystemSecret struct {
	ID             int32
	EncryptionSalt []byte
	KeyVersion     int
	CreatedAt      time.Time
	RotatedAt      *time.Time
}

// RBACStore interface defines all RBAC-related database operations.
type RBACStore interface {
	// User-tenant permission operations
	CreateUserTenantPermission(ctx context.Context, perm *UserTenantPermission) (*UserTenantPermission, error)
	GetUserTenantPermission(ctx context.Context, find *FindUserTenantPermission) (*UserTenantPermission, error)
	ListUserTenantPermissions(ctx context.Context, find *FindUserTenantPermission) ([]*UserTenantPermission, error)
	UpdateUserTenantPermission(ctx context.Context, perm *UserTenantPermission) (*UserTenantPermission, error)
	DeleteUserTenantPermission(ctx context.Context, userID, tenantID int32) error

	// Tenant config operations
	GetTenantConfig(ctx context.Context, find *FindTenantConfig) (*TenantConfig, error)
	UpsertTenantConfig(ctx context.Context, config *TenantConfig) (*TenantConfig, error)
	DeleteTenantConfig(ctx context.Context, tenantID int32) error

	// System secret operations
	GetSystemSecret(ctx context.Context) (*SystemSecret, error)
	UpsertSystemSecret(ctx context.Context, secret *SystemSecret) (*SystemSecret, error)
}

// Store methods that delegate to the driver

func (s *Store) CreateUserTenantPermission(ctx context.Context, perm *UserTenantPermission) (*UserTenantPermission, error) {
	return s.driver.CreateUserTenantPermission(ctx, perm)
}

func (s *Store) GetUserTenantPermission(ctx context.Context, find *FindUserTenantPermission) (*UserTenantPermission, error) {
	return s.driver.GetUserTenantPermission(ctx, find)
}

func (s *Store) ListUserTenantPermissions(ctx context.Context, find *FindUserTenantPermission) ([]*UserTenantPermission, error) {
	return s.driver.ListUserTenantPermissions(ctx, find)
}

func (s *Store) UpdateUserTenantPermission(ctx context.Context, perm *UserTenantPermission) (*UserTenantPermission, error) {
	return s.driver.UpdateUserTenantPermission(ctx, perm)
}

func (s *Store) DeleteUserTenantPermission(ctx context.Context, userID, tenantID int32) error {
	return s.driver.DeleteUserTenantPermission(ctx, userID, tenantID)
}

func (s *Store) GetTenantConfig(ctx context.Context, find *FindTenantConfig) (*TenantConfig, error) {
	return s.driver.GetTenantConfig(ctx, find)
}

func (s *Store) UpsertTenantConfig(ctx context.Context, config *TenantConfig) (*TenantConfig, error) {
	return s.driver.UpsertTenantConfig(ctx, config)
}

func (s *Store) DeleteTenantConfig(ctx context.Context, tenantID int32) error {
	return s.driver.DeleteTenantConfig(ctx, tenantID)
}

func (s *Store) GetSystemSecret(ctx context.Context) (*SystemSecret, error) {
	return s.driver.GetSystemSecret(ctx)
}

func (s *Store) UpsertSystemSecret(ctx context.Context, secret *SystemSecret) (*SystemSecret, error) {
	return s.driver.UpsertSystemSecret(ctx, secret)
}
