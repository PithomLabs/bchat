package store

import (
	"context"
	"database/sql"

	exprv1 "google.golang.org/genproto/googleapis/api/expr/v1alpha1"

	"github.com/usememos/memos/plugin/filter"
)

// Driver is an interface for store driver.
// It contains all methods that store database driver should implement.
type Driver interface {
	GetDB() *sql.DB
	Close() error

	// MigrationHistory model related methods.
	FindMigrationHistoryList(ctx context.Context, find *FindMigrationHistory) ([]*MigrationHistory, error)
	UpsertMigrationHistory(ctx context.Context, upsert *UpsertMigrationHistory) (*MigrationHistory, error)

	// Activity model related methods.
	CreateActivity(ctx context.Context, create *Activity) (*Activity, error)
	ListActivities(ctx context.Context, find *FindActivity) ([]*Activity, error)

	// Resource model related methods.
	CreateResource(ctx context.Context, create *Resource) (*Resource, error)
	ListResources(ctx context.Context, find *FindResource) ([]*Resource, error)
	UpdateResource(ctx context.Context, update *UpdateResource) error
	DeleteResource(ctx context.Context, delete *DeleteResource) error

	// Memo model related methods.
	CreateMemo(ctx context.Context, create *Memo) (*Memo, error)
	ListMemos(ctx context.Context, find *FindMemo) ([]*Memo, error)
	UpdateMemo(ctx context.Context, update *UpdateMemo) error
	DeleteMemo(ctx context.Context, delete *DeleteMemo) error

	// MemoRelation model related methods.
	UpsertMemoRelation(ctx context.Context, create *MemoRelation) (*MemoRelation, error)
	ListMemoRelations(ctx context.Context, find *FindMemoRelation) ([]*MemoRelation, error)
	DeleteMemoRelation(ctx context.Context, delete *DeleteMemoRelation) error

	// WorkspaceSetting model related methods.
	UpsertWorkspaceSetting(ctx context.Context, upsert *WorkspaceSetting) (*WorkspaceSetting, error)
	ListWorkspaceSettings(ctx context.Context, find *FindWorkspaceSetting) ([]*WorkspaceSetting, error)
	DeleteWorkspaceSetting(ctx context.Context, delete *DeleteWorkspaceSetting) error

	// User model related methods.
	CreateUser(ctx context.Context, create *User) (*User, error)
	UpdateUser(ctx context.Context, update *UpdateUser) (*User, error)
	ListUsers(ctx context.Context, find *FindUser) ([]*User, error)
	DeleteUser(ctx context.Context, delete *DeleteUser) error

	// UserSetting model related methods.
	UpsertUserSetting(ctx context.Context, upsert *UserSetting) (*UserSetting, error)
	ListUserSettings(ctx context.Context, find *FindUserSetting) ([]*UserSetting, error)

	// IdentityProvider model related methods.
	CreateIdentityProvider(ctx context.Context, create *IdentityProvider) (*IdentityProvider, error)
	ListIdentityProviders(ctx context.Context, find *FindIdentityProvider) ([]*IdentityProvider, error)
	UpdateIdentityProvider(ctx context.Context, update *UpdateIdentityProvider) (*IdentityProvider, error)
	DeleteIdentityProvider(ctx context.Context, delete *DeleteIdentityProvider) error

	// Inbox model related methods.
	CreateInbox(ctx context.Context, create *Inbox) (*Inbox, error)
	ListInboxes(ctx context.Context, find *FindInbox) ([]*Inbox, error)
	UpdateInbox(ctx context.Context, update *UpdateInbox) (*Inbox, error)
	DeleteInbox(ctx context.Context, delete *DeleteInbox) error

	// Webhook model related methods.
	CreateWebhook(ctx context.Context, create *Webhook) (*Webhook, error)
	ListWebhooks(ctx context.Context, find *FindWebhook) ([]*Webhook, error)
	UpdateWebhook(ctx context.Context, update *UpdateWebhook) (*Webhook, error)
	DeleteWebhook(ctx context.Context, delete *DeleteWebhook) error

	// Reaction model related methods.
	UpsertReaction(ctx context.Context, create *Reaction) (*Reaction, error)
	ListReactions(ctx context.Context, find *FindReaction) ([]*Reaction, error)
	DeleteReaction(ctx context.Context, delete *DeleteReaction) error

	// Ticket model related methods.
	CreateTicket(ctx context.Context, create *Ticket) (*Ticket, error)
	ListTickets(ctx context.Context, find *FindTicket) ([]*Ticket, error)
	GetTicket(ctx context.Context, find *FindTicket) (*Ticket, error)
	UpdateTicket(ctx context.Context, update *UpdateTicket) (*Ticket, error)
	DeleteTicket(ctx context.Context, delete *DeleteTicket) error

	// Notification model related methods.
	CreateNotification(ctx context.Context, create *Notification) (*Notification, error)
	ListNotifications(ctx context.Context, find *FindNotification) ([]*Notification, error)
	UpdateNotification(ctx context.Context, update *UpdateNotification) (*Notification, error)

	// Shortcut related methods.
	ConvertExprToSQL(ctx *filter.ConvertContext, expr *exprv1.Expr) error

	// Agent model related methods.
	CreateAgentTenant(ctx context.Context, tenant *AgentTenant) (*AgentTenant, error)
	GetAgentTenant(ctx context.Context, find *FindAgentTenant) (*AgentTenant, error)
	ListAgentTenants(ctx context.Context, find *FindAgentTenant) ([]*AgentTenant, error)
	UpdateAgentTenant(ctx context.Context, tenant *AgentTenant) (*AgentTenant, error)
	DeleteAgentTenant(ctx context.Context, id int32) error

	CreateAgentAudience(ctx context.Context, audience *AgentAudience) (*AgentAudience, error)
	GetAgentAudience(ctx context.Context, find *FindAgentAudience) (*AgentAudience, error)
	ListAgentAudiences(ctx context.Context, find *FindAgentAudience) ([]*AgentAudience, error)
	UpdateAgentAudience(ctx context.Context, audience *AgentAudience) (*AgentAudience, error)
	DeleteAgentAudience(ctx context.Context, tenantID int32, audienceType string) error

	CreateAgentService(ctx context.Context, service *AgentService) (*AgentService, error)
	ListAgentServices(ctx context.Context, find *FindAgentService) ([]*AgentService, error)
	DeleteAgentServices(ctx context.Context, tenantID int32, audienceType string) error

	CreateAgentExclusion(ctx context.Context, exclusion *AgentExclusion) (*AgentExclusion, error)
	ListAgentExclusions(ctx context.Context, find *FindAgentExclusion) ([]*AgentExclusion, error)
	DeleteAgentExclusions(ctx context.Context, tenantID int32, audienceType string) error

	CreateAgentCoverage(ctx context.Context, coverage *AgentCoverage) (*AgentCoverage, error)
	ListAgentCoverage(ctx context.Context, find *FindAgentCoverage) ([]*AgentCoverage, error)
	DeleteAgentCoverage(ctx context.Context, tenantID int32) error

	CreateAgentFAQ(ctx context.Context, faq *AgentFAQ) (*AgentFAQ, error)
	ListAgentFAQs(ctx context.Context, find *FindAgentFAQ) ([]*AgentFAQ, error)
	DeleteAgentFAQs(ctx context.Context, tenantID int32, audienceType string) error

	CreateAgentSafetyProtocol(ctx context.Context, protocol *AgentSafetyProtocol) (*AgentSafetyProtocol, error)
	ListAgentSafetyProtocols(ctx context.Context, find *FindAgentSafetyProtocol) ([]*AgentSafetyProtocol, error)
	DeleteAgentSafetyProtocols(ctx context.Context, tenantID int32, audienceType string) error

	CreateAgentKBSection(ctx context.Context, section *AgentKBSection) (*AgentKBSection, error)
	ListAgentKBSections(ctx context.Context, find *FindAgentKBSection) ([]*AgentKBSection, error)
	DeleteAgentKBSections(ctx context.Context, tenantID int32, audienceType string) error

	CreateAgentIntent(ctx context.Context, intent *AgentIntent) (*AgentIntent, error)
	ListAgentIntents(ctx context.Context, find *FindAgentIntent) ([]*AgentIntent, error)
	DeleteAgentIntents(ctx context.Context, tenantID int32, audienceType *string) error

	CreateAgentRule(ctx context.Context, rule *AgentRule) (*AgentRule, error)
	ListAgentRules(ctx context.Context, find *FindAgentRule) ([]*AgentRule, error)
	DeleteAgentRules(ctx context.Context, tenantID int32, audienceType string) error

	CreateAgentSession(ctx context.Context, session *AgentSession) (*AgentSession, error)
	GetAgentSession(ctx context.Context, find *FindAgentSession) (*AgentSession, error)
	ListAgentSessions(ctx context.Context, find *FindAgentSession) ([]*AgentSession, error)
	UpdateAgentSession(ctx context.Context, update *UpdateAgentSession) (*AgentSession, error)
	DeleteAgentSession(ctx context.Context, id string) error

	UpsertAgentSourceFile(ctx context.Context, file *AgentSourceFile) (*AgentSourceFile, error)
	GetAgentSourceFile(ctx context.Context, find *FindAgentSourceFile) (*AgentSourceFile, error)
	ListAgentSourceFiles(ctx context.Context, find *FindAgentSourceFile) ([]*AgentSourceFile, error)
	DeleteAgentSourceFiles(ctx context.Context, tenantID int32, audienceType *string) error

	GetOrCreateAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) (*AgentRateLimit, error)
	IncrementAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) error
	ResetAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) error

	CreateAgentSimulationTranscript(ctx context.Context, transcript *AgentSimulationTranscript) (*AgentSimulationTranscript, error)
	GetAgentSimulationTranscript(ctx context.Context, find *FindAgentSimulationTranscript) (*AgentSimulationTranscript, error)
	ListAgentSimulationTranscripts(ctx context.Context, find *FindAgentSimulationTranscript) ([]*AgentSimulationTranscript, int, error)
	DeleteAgentSimulationTranscript(ctx context.Context, id string) error

	// RBAC model related methods.
	CreateUserTenantPermission(ctx context.Context, perm *UserTenantPermission) (*UserTenantPermission, error)
	GetUserTenantPermission(ctx context.Context, find *FindUserTenantPermission) (*UserTenantPermission, error)
	ListUserTenantPermissions(ctx context.Context, find *FindUserTenantPermission) ([]*UserTenantPermission, error)
	UpdateUserTenantPermission(ctx context.Context, perm *UserTenantPermission) (*UserTenantPermission, error)
	DeleteUserTenantPermission(ctx context.Context, userID, tenantID int32) error

	GetTenantConfig(ctx context.Context, find *FindTenantConfig) (*TenantConfig, error)
	UpsertTenantConfig(ctx context.Context, config *TenantConfig) (*TenantConfig, error)
	DeleteTenantConfig(ctx context.Context, tenantID int32) error

	GetSystemSecret(ctx context.Context) (*SystemSecret, error)
	UpsertSystemSecret(ctx context.Context, secret *SystemSecret) (*SystemSecret, error)

	// SCRIPT.MD model related methods.
	UpsertAgentTenantScript(ctx context.Context, script *AgentTenantScript) (*AgentTenantScript, error)
	GetAgentTenantScript(ctx context.Context, find *FindAgentTenantScript) (*AgentTenantScript, error)
	DeleteAgentTenantScript(ctx context.Context, tenantID int32) error

	// Analysis model related methods.
	CreateAgentAnalysisResult(ctx context.Context, result *AgentAnalysisResult) (*AgentAnalysisResult, error)
	GetAgentAnalysisResult(ctx context.Context, find *FindAgentAnalysisResult) (*AgentAnalysisResult, error)
	ListAgentAnalysisResults(ctx context.Context, find *FindAgentAnalysisResult) ([]*AgentAnalysisResult, int, error)

	// Learning memory model related methods.
	GetOrCreateAgentLearningMemory(ctx context.Context, tenantID int32) (*AgentLearningMemory, error)
	UpdateAgentLearningMemory(ctx context.Context, memory *AgentLearningMemory) (*AgentLearningMemory, error)
	DeleteAgentLearningMemory(ctx context.Context, tenantID int32) error

	// Compliance audit model related methods.
	CreateAgentComplianceAudit(ctx context.Context, audit *AgentComplianceAudit) error
	GetAgentComplianceAudit(ctx context.Context, find *FindAgentComplianceAudit) (*AgentComplianceAudit, error)
	ListAgentComplianceAudits(ctx context.Context, find *FindAgentComplianceAudit) ([]*AgentComplianceAudit, error)

	// Scoring config model related methods.
	GetOrCreateAgentScoringConfig(ctx context.Context, tenantID int32) (*AgentScoringConfig, error)
	UpdateAgentScoringConfig(ctx context.Context, config *AgentScoringConfig) (*AgentScoringConfig, error)
}
