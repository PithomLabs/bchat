package mysql

import (
	"context"
	"errors"

	"github.com/usememos/memos/store"
)

var errNotImplemented = errors.New("agent features not implemented for MySQL")

// Agent Tenant methods

func (d *DB) CreateAgentTenant(ctx context.Context, tenant *store.AgentTenant) (*store.AgentTenant, error) {
	return nil, errNotImplemented
}

func (d *DB) GetAgentTenant(ctx context.Context, find *store.FindAgentTenant) (*store.AgentTenant, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentTenants(ctx context.Context, find *store.FindAgentTenant) ([]*store.AgentTenant, error) {
	return nil, errNotImplemented
}

func (d *DB) UpdateAgentTenant(ctx context.Context, tenant *store.AgentTenant) (*store.AgentTenant, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentTenant(ctx context.Context, id int32) error {
	return errNotImplemented
}

// Agent Audience methods

func (d *DB) CreateAgentAudience(ctx context.Context, audience *store.AgentAudience) (*store.AgentAudience, error) {
	return nil, errNotImplemented
}

func (d *DB) GetAgentAudience(ctx context.Context, find *store.FindAgentAudience) (*store.AgentAudience, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentAudiences(ctx context.Context, find *store.FindAgentAudience) ([]*store.AgentAudience, error) {
	return nil, errNotImplemented
}

func (d *DB) UpdateAgentAudience(ctx context.Context, audience *store.AgentAudience) (*store.AgentAudience, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentAudience(ctx context.Context, tenantID int32, audienceType string) error {
	return errNotImplemented
}

// Agent Service methods

func (d *DB) CreateAgentService(ctx context.Context, service *store.AgentService) (*store.AgentService, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentServices(ctx context.Context, find *store.FindAgentService) ([]*store.AgentService, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentServices(ctx context.Context, tenantID int32, audienceType string) error {
	return errNotImplemented
}

// Agent Exclusion methods

func (d *DB) CreateAgentExclusion(ctx context.Context, exclusion *store.AgentExclusion) (*store.AgentExclusion, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentExclusions(ctx context.Context, find *store.FindAgentExclusion) ([]*store.AgentExclusion, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentExclusions(ctx context.Context, tenantID int32, audienceType string) error {
	return errNotImplemented
}

// Agent Coverage methods

func (d *DB) CreateAgentCoverage(ctx context.Context, coverage *store.AgentCoverage) (*store.AgentCoverage, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentCoverage(ctx context.Context, find *store.FindAgentCoverage) ([]*store.AgentCoverage, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentCoverage(ctx context.Context, tenantID int32) error {
	return errNotImplemented
}

// Agent FAQ methods

func (d *DB) CreateAgentFAQ(ctx context.Context, faq *store.AgentFAQ) (*store.AgentFAQ, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentFAQs(ctx context.Context, find *store.FindAgentFAQ) ([]*store.AgentFAQ, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentFAQs(ctx context.Context, tenantID int32, audienceType string) error {
	return errNotImplemented
}

// Agent Safety Protocol methods

func (d *DB) CreateAgentSafetyProtocol(ctx context.Context, protocol *store.AgentSafetyProtocol) (*store.AgentSafetyProtocol, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentSafetyProtocols(ctx context.Context, find *store.FindAgentSafetyProtocol) ([]*store.AgentSafetyProtocol, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentSafetyProtocols(ctx context.Context, tenantID int32, audienceType string) error {
	return errNotImplemented
}

// Agent KB Section methods

func (d *DB) CreateAgentKBSection(ctx context.Context, section *store.AgentKBSection) (*store.AgentKBSection, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentKBSections(ctx context.Context, find *store.FindAgentKBSection) ([]*store.AgentKBSection, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentKBSections(ctx context.Context, tenantID int32, audienceType string) error {
	return errNotImplemented
}

// Agent Intent methods

func (d *DB) CreateAgentIntent(ctx context.Context, intent *store.AgentIntent) (*store.AgentIntent, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentIntents(ctx context.Context, find *store.FindAgentIntent) ([]*store.AgentIntent, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentIntents(ctx context.Context, tenantID int32, audienceType *string) error {
	return errNotImplemented
}

// Agent Rule methods

func (d *DB) CreateAgentRule(ctx context.Context, rule *store.AgentRule) (*store.AgentRule, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentRules(ctx context.Context, find *store.FindAgentRule) ([]*store.AgentRule, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentRules(ctx context.Context, tenantID int32, audienceType string) error {
	return errNotImplemented
}

// Agent Session methods

func (d *DB) CreateAgentSession(ctx context.Context, session *store.AgentSession) (*store.AgentSession, error) {
	return nil, errNotImplemented
}

func (d *DB) GetAgentSession(ctx context.Context, find *store.FindAgentSession) (*store.AgentSession, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentSessions(ctx context.Context, find *store.FindAgentSession) ([]*store.AgentSession, error) {
	return nil, errNotImplemented
}

func (d *DB) UpdateAgentSession(ctx context.Context, update *store.UpdateAgentSession) (*store.AgentSession, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentSession(ctx context.Context, id string) error {
	return errNotImplemented
}

// Agent Source File methods

func (d *DB) UpsertAgentSourceFile(ctx context.Context, file *store.AgentSourceFile) (*store.AgentSourceFile, error) {
	return nil, errNotImplemented
}

func (d *DB) GetAgentSourceFile(ctx context.Context, find *store.FindAgentSourceFile) (*store.AgentSourceFile, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentSourceFiles(ctx context.Context, find *store.FindAgentSourceFile) ([]*store.AgentSourceFile, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentSourceFiles(ctx context.Context, tenantID int32, audienceType *string) error {
	return errNotImplemented
}

// Agent Rate Limit methods

func (d *DB) GetOrCreateAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) (*store.AgentRateLimit, error) {
	return nil, errNotImplemented
}

func (d *DB) IncrementAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) error {
	return errNotImplemented
}

func (d *DB) ResetAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) error {
	return errNotImplemented
}

// Agent Simulation Transcript methods

func (d *DB) CreateAgentSimulationTranscript(ctx context.Context, transcript *store.AgentSimulationTranscript) (*store.AgentSimulationTranscript, error) {
	return nil, errNotImplemented
}

func (d *DB) GetAgentSimulationTranscript(ctx context.Context, find *store.FindAgentSimulationTranscript) (*store.AgentSimulationTranscript, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentSimulationTranscripts(ctx context.Context, find *store.FindAgentSimulationTranscript) ([]*store.AgentSimulationTranscript, int, error) {
	return nil, 0, errNotImplemented
}

func (d *DB) DeleteAgentSimulationTranscript(ctx context.Context, id string) error {
	return errNotImplemented
}

// Agent Tenant Script methods

func (d *DB) UpsertAgentTenantScript(ctx context.Context, script *store.AgentTenantScript) (*store.AgentTenantScript, error) {
	return nil, errNotImplemented
}

func (d *DB) GetAgentTenantScript(ctx context.Context, find *store.FindAgentTenantScript) (*store.AgentTenantScript, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentTenantScript(ctx context.Context, tenantID int32) error {
	return errNotImplemented
}

// Agent Analysis Result methods

func (d *DB) CreateAgentAnalysisResult(ctx context.Context, result *store.AgentAnalysisResult) (*store.AgentAnalysisResult, error) {
	return nil, errNotImplemented
}

func (d *DB) GetAgentAnalysisResult(ctx context.Context, find *store.FindAgentAnalysisResult) (*store.AgentAnalysisResult, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentAnalysisResults(ctx context.Context, find *store.FindAgentAnalysisResult) ([]*store.AgentAnalysisResult, int, error) {
	return nil, 0, errNotImplemented
}

// Agent Learning Memory methods

func (d *DB) GetOrCreateAgentLearningMemory(ctx context.Context, tenantID int32) (*store.AgentLearningMemory, error) {
	return nil, errNotImplemented
}

func (d *DB) UpdateAgentLearningMemory(ctx context.Context, memory *store.AgentLearningMemory) (*store.AgentLearningMemory, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentLearningMemory(ctx context.Context, tenantID int32) error {
	return errNotImplemented
}

// Agent Compliance Audit methods

func (d *DB) CreateAgentComplianceAudit(ctx context.Context, audit *store.AgentComplianceAudit) error {
	return errNotImplemented
}

func (d *DB) GetAgentComplianceAudit(ctx context.Context, find *store.FindAgentComplianceAudit) (*store.AgentComplianceAudit, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentComplianceAudits(ctx context.Context, find *store.FindAgentComplianceAudit) ([]*store.AgentComplianceAudit, error) {
	return nil, errNotImplemented
}

// Agent Scoring Config methods

func (d *DB) GetOrCreateAgentScoringConfig(ctx context.Context, tenantID int32) (*store.AgentScoringConfig, error) {
	return nil, errNotImplemented
}

func (d *DB) UpdateAgentScoringConfig(ctx context.Context, config *store.AgentScoringConfig) (*store.AgentScoringConfig, error) {
	return nil, errNotImplemented
}

// Agent Q&A Pair methods

func (d *DB) CreateAgentQAPair(ctx context.Context, pair *store.AgentQAPair) (*store.AgentQAPair, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentQAPairs(ctx context.Context, find *store.FindAgentQAPair) ([]*store.AgentQAPair, error) {
	return nil, errNotImplemented
}

func (d *DB) UpdateAgentQAPair(ctx context.Context, pair *store.AgentQAPair) (*store.AgentQAPair, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentQAPair(ctx context.Context, id int32) error {
	return errNotImplemented
}

func (d *DB) DeleteAgentQAPairsByTenant(ctx context.Context, tenantID int32) error {
	return errNotImplemented
}

// Agent Transcript methods

func (d *DB) CreateAgentTranscript(ctx context.Context, transcript *store.AgentTranscript) (*store.AgentTranscript, error) {
	return nil, errNotImplemented
}

func (d *DB) GetAgentTranscript(ctx context.Context, find *store.FindAgentTranscript) (*store.AgentTranscript, error) {
	return nil, errNotImplemented
}

func (d *DB) ListAgentTranscripts(ctx context.Context, find *store.FindAgentTranscript) ([]*store.AgentTranscript, error) {
	return nil, errNotImplemented
}

func (d *DB) UpdateAgentTranscript(ctx context.Context, transcript *store.AgentTranscript) error {
	return errNotImplemented
}

func (d *DB) DeleteAgentTranscript(ctx context.Context, id string) error {
	return errNotImplemented
}
