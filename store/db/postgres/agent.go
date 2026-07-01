package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/usememos/memos/store"
)

var errNotImplemented = errors.New("agent features not implemented for PostgreSQL")

// Agent Tenant methods

func (d *DB) CreateAgentTenant(ctx context.Context, tenant *store.AgentTenant) (*store.AgentTenant, error) {
	now := time.Now()
	if tenant.GUID == "" {
		tenant.GUID = uuid.NewString()
	}
	err := d.db.QueryRowContext(ctx, `
		INSERT INTO agent_tenants (slug, company_name, guid, vertical, is_active, processing_options, allowed_domains, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),$8,$8)
		RETURNING id
	`, tenant.Slug, tenant.CompanyName, tenant.GUID, tenant.Vertical, tenant.IsActive, tenant.ProcessingOptions, tenant.AllowedDomains, now).Scan(&tenant.ID)
	if err != nil {
		return nil, err
	}
	tenant.CreatedAt, tenant.UpdatedAt = now, now
	return tenant, nil
}

func (d *DB) GetAgentTenant(ctx context.Context, find *store.FindAgentTenant) (*store.AgentTenant, error) {
	list, err := d.ListAgentTenants(ctx, find)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return list[0], nil
}

func (d *DB) ListAgentTenants(ctx context.Context, find *store.FindAgentTenant) ([]*store.AgentTenant, error) {
	where := []string{"TRUE"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if find.ID != nil {
		add("id = $%d", *find.ID)
	}
	if find.Slug != nil {
		add("slug = $%d", *find.Slug)
	}
	if find.IsActive != nil {
		add("is_active = $%d", *find.IsActive)
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, slug, company_name, guid, vertical, is_active, processing_options, allowed_domains, created_at, updated_at
		FROM agent_tenants WHERE `+strings.Join(where, " AND ")+` ORDER BY created_at DESC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*store.AgentTenant
	for rows.Next() {
		var tenant store.AgentTenant
		var guid, vertical, processing, domains sql.NullString
		if err := rows.Scan(&tenant.ID, &tenant.Slug, &tenant.CompanyName, &guid, &vertical, &tenant.IsActive, &processing, &domains, &tenant.CreatedAt, &tenant.UpdatedAt); err != nil {
			return nil, err
		}
		tenant.GUID, tenant.Vertical = guid.String, vertical.String
		tenant.ProcessingOptions, tenant.AllowedDomains = processing.String, domains.String
		result = append(result, &tenant)
	}
	return result, rows.Err()
}

func (d *DB) UpdateAgentTenant(ctx context.Context, tenant *store.AgentTenant) (*store.AgentTenant, error) {
	now := time.Now()
	_, err := d.db.ExecContext(ctx, `
		UPDATE agent_tenants SET company_name=$1, vertical=$2, is_active=$3,
			processing_options=NULLIF($4,''), allowed_domains=NULLIF($5,''), updated_at=$6
		WHERE id=$7
	`, tenant.CompanyName, tenant.Vertical, tenant.IsActive, tenant.ProcessingOptions, tenant.AllowedDomains, now, tenant.ID)
	if err != nil {
		return nil, err
	}
	tenant.UpdatedAt = now
	return tenant, nil
}

func (d *DB) DeleteAgentTenant(ctx context.Context, id int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_tenants WHERE id=$1", id)
	return err
}

// Agent Audience methods

func (d *DB) CreateAgentAudience(ctx context.Context, audience *store.AgentAudience) (*store.AgentAudience, error) {
	guidelines, err := json.Marshal(audience.Guidelines)
	if err != nil {
		return nil, err
	}
	phones, err := json.Marshal(audience.SecondaryPhones)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	err = d.db.QueryRowContext(ctx, `
		INSERT INTO agent_audiences (
			tenant_id,audience_type,role,tone,brand_voice,guidelines,emergency_phone,
			secondary_phones,email,address,emergency_urgency_threshold,
			escalation_confidence_threshold,rate_limit_rpm,require_contact_on_fallback,updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id
	`, audience.TenantID, audience.AudienceType, audience.Role, audience.Tone, audience.BrandVoice,
		string(guidelines), audience.EmergencyPhone, string(phones), audience.Email, audience.Address,
		audience.EmergencyUrgencyThreshold, audience.EscalationConfidenceThreshold, audience.RateLimitRPM,
		audience.RequireContactOnFallback, now,
	).Scan(&audience.ID)
	if err != nil {
		return nil, err
	}
	audience.UpdatedAt = now
	return audience, nil
}

func (d *DB) GetAgentAudience(ctx context.Context, find *store.FindAgentAudience) (*store.AgentAudience, error) {
	list, err := d.ListAgentAudiences(ctx, find)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return list[0], nil
}

func (d *DB) ListAgentAudiences(ctx context.Context, find *store.FindAgentAudience) ([]*store.AgentAudience, error) {
	where := []string{"TRUE"}
	args := []any{}
	if find.TenantID != nil {
		args = append(args, *find.TenantID)
		where = append(where, fmt.Sprintf("tenant_id=$%d", len(args)))
	}
	if find.AudienceType != nil {
		args = append(args, *find.AudienceType)
		where = append(where, fmt.Sprintf("audience_type=$%d", len(args)))
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id,tenant_id,audience_type,role,tone,brand_voice,guidelines,emergency_phone,
			secondary_phones,email,address,emergency_urgency_threshold,
			escalation_confidence_threshold,rate_limit_rpm,require_contact_on_fallback,updated_at
		FROM agent_audiences WHERE `+strings.Join(where, " AND "), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*store.AgentAudience
	for rows.Next() {
		var audience store.AgentAudience
		var brand, guidelines, phones, email, address sql.NullString
		if err := rows.Scan(&audience.ID, &audience.TenantID, &audience.AudienceType, &audience.Role,
			&audience.Tone, &brand, &guidelines, &audience.EmergencyPhone, &phones, &email, &address,
			&audience.EmergencyUrgencyThreshold, &audience.EscalationConfidenceThreshold,
			&audience.RateLimitRPM, &audience.RequireContactOnFallback, &audience.UpdatedAt); err != nil {
			return nil, err
		}
		audience.BrandVoice, audience.Email, audience.Address = brand.String, email.String, address.String
		if guidelines.Valid {
			_ = json.Unmarshal([]byte(guidelines.String), &audience.Guidelines)
		}
		if phones.Valid {
			_ = json.Unmarshal([]byte(phones.String), &audience.SecondaryPhones)
		}
		result = append(result, &audience)
	}
	return result, rows.Err()
}

func (d *DB) UpdateAgentAudience(ctx context.Context, audience *store.AgentAudience) (*store.AgentAudience, error) {
	guidelines, err := json.Marshal(audience.Guidelines)
	if err != nil {
		return nil, err
	}
	phones, err := json.Marshal(audience.SecondaryPhones)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	_, err = d.db.ExecContext(ctx, `
		UPDATE agent_audiences SET role=$1,tone=$2,brand_voice=$3,guidelines=$4,
			emergency_phone=$5,secondary_phones=$6,email=$7,address=$8,
			emergency_urgency_threshold=$9,escalation_confidence_threshold=$10,
			rate_limit_rpm=$11,require_contact_on_fallback=$12,updated_at=$13
		WHERE tenant_id=$14 AND audience_type=$15
	`, audience.Role, audience.Tone, audience.BrandVoice, string(guidelines), audience.EmergencyPhone,
		string(phones), audience.Email, audience.Address, audience.EmergencyUrgencyThreshold,
		audience.EscalationConfidenceThreshold, audience.RateLimitRPM, audience.RequireContactOnFallback,
		now, audience.TenantID, audience.AudienceType)
	if err != nil {
		return nil, err
	}
	audience.UpdatedAt = now
	return audience, nil
}

func (d *DB) DeleteAgentAudience(ctx context.Context, tenantID int32, audienceType string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_audiences WHERE tenant_id=$1 AND audience_type=$2", tenantID, audienceType)
	return err
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

// Agent Message operations

func (d *DB) CreateAgentMessages(ctx context.Context, messages []*store.AgentMessageRecord) error {
	if len(messages) == 0 {
		return nil
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt := `
		INSERT INTO agent_messages (session_id, tenant_id, source, source_id, role, content, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	for _, m := range messages {
		if _, err := tx.ExecContext(ctx, stmt, m.SessionID, m.TenantID, m.Source, m.SourceID, m.Role, m.Content, m.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) GetAssistantMessageBySourceID(ctx context.Context, sessionID, sourceID string) (*store.AgentMessageRecord, error) {
	query := `
		SELECT id, session_id, tenant_id, source, source_id, role, content, created_at
		FROM agent_messages
		WHERE session_id = $1 AND source = 'external_response' AND source_id = $2
		LIMIT 1
	`
	var m store.AgentMessageRecord
	if err := d.db.QueryRowContext(ctx, query, sessionID, sourceID).Scan(
		&m.ID, &m.SessionID, &m.TenantID, &m.Source, &m.SourceID, &m.Role, &m.Content, &m.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (d *DB) GetUserMessageBySourceID(ctx context.Context, sessionID, sourceID string) (*store.AgentMessageRecord, error) {
	query := `
		SELECT id, session_id, tenant_id, source, source_id, role, content, created_at
		FROM agent_messages
		WHERE session_id = $1 AND source = 'external_client_message' AND source_id = $2
		LIMIT 1
	`
	var m store.AgentMessageRecord
	if err := d.db.QueryRowContext(ctx, query, sessionID, sourceID).Scan(
		&m.ID, &m.SessionID, &m.TenantID, &m.Source, &m.SourceID, &m.Role, &m.Content, &m.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
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

func (d *DB) UpdateAgentQAPair(ctx context.Context, pair *store.AgentQAPair, tenantID int32) (*store.AgentQAPair, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteAgentQAPair(ctx context.Context, id int32, tenantID int32) error {
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

func (d *DB) UpsertAgentLead(ctx context.Context, lead *store.AgentLead) (*store.AgentLead, error) {
	if lead.ID == "" {
		lead.ID = uuid.NewString()
	}
	if lead.Status == "" {
		lead.Status = "new"
	}
	now := time.Now()
	if lead.CreatedAt.IsZero() {
		lead.CreatedAt = now
	}
	lead.UpdatedAt = now
	if lead.LastMessageAt.IsZero() {
		lead.LastMessageAt = now
	}
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO agent_leads (
			id, tenant_id, session_id, transcript_id, name, email, phone, topic,
			location, detected_intent, status, created_at, updated_at, last_message_at, converted_at
		) VALUES ($1, $2, $3, NULLIF($4, ''), $5, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''),
			NULLIF($9, ''), NULLIF($10, ''), $11, $12, $13, $14, $15)
		ON CONFLICT(tenant_id, session_id) DO UPDATE SET
			transcript_id = COALESCE(excluded.transcript_id, agent_leads.transcript_id),
			name = excluded.name,
			email = COALESCE(excluded.email, agent_leads.email),
			phone = COALESCE(excluded.phone, agent_leads.phone),
			topic = COALESCE(excluded.topic, agent_leads.topic),
			location = COALESCE(excluded.location, agent_leads.location),
			detected_intent = COALESCE(excluded.detected_intent, agent_leads.detected_intent),
			updated_at = excluded.updated_at,
			last_message_at = excluded.last_message_at
	`, lead.ID, lead.TenantID, lead.SessionID, lead.TranscriptID, lead.Name, lead.Email, lead.Phone,
		lead.Topic, lead.Location, lead.DetectedIntent, lead.Status, lead.CreatedAt, lead.UpdatedAt,
		lead.LastMessageAt, lead.ConvertedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert agent lead: %w", err)
	}
	return d.GetAgentLead(ctx, &store.FindAgentLead{TenantID: &lead.TenantID, SessionID: &lead.SessionID})
}

func (d *DB) GetAgentLead(ctx context.Context, find *store.FindAgentLead) (*store.AgentLead, error) {
	leads, err := d.ListAgentLeads(ctx, &store.FindAgentLead{
		ID:        find.ID,
		TenantID:  find.TenantID,
		SessionID: find.SessionID,
		Status:    find.Status,
		Limit:     1,
	})
	if err != nil || len(leads) == 0 {
		return nil, err
	}
	return leads[0], nil
}

func (d *DB) ListAgentLeads(ctx context.Context, find *store.FindAgentLead) ([]*store.AgentLead, error) {
	where := []string{"TRUE"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if find.ID != nil {
		add("id = $%d", *find.ID)
	}
	if find.TenantID != nil {
		add("tenant_id = $%d", *find.TenantID)
	}
	if find.SessionID != nil {
		add("session_id = $%d", *find.SessionID)
	}
	if find.Status != nil {
		add("status = $%d", *find.Status)
	}
	limit := 100
	if find.Limit > 0 {
		limit = find.Limit
	}
	args = append(args, limit, find.Offset)
	limitPos := len(args) - 1
	offsetPos := len(args)
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, tenant_id, session_id, transcript_id, name, email, phone, topic,
			location, detected_intent, status, created_at, updated_at, last_message_at, converted_at
		FROM agent_leads
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY updated_at DESC
		LIMIT $`+fmt.Sprint(limitPos)+` OFFSET $`+fmt.Sprint(offsetPos), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent leads: %w", err)
	}
	defer rows.Close()
	var leads []*store.AgentLead
	for rows.Next() {
		lead, err := scanAgentLead(rows)
		if err != nil {
			return nil, err
		}
		leads = append(leads, lead)
	}
	return leads, rows.Err()
}

func (d *DB) UpdateAgentLeadStatus(ctx context.Context, tenantID int32, id string, status string, convertedAt *time.Time) (*store.AgentLead, error) {
	updatedAt := time.Now()
	_, err := d.db.ExecContext(ctx, `
		UPDATE agent_leads
		SET status = $1, converted_at = $2, updated_at = $3
		WHERE tenant_id = $4 AND id = $5
	`, status, convertedAt, updatedAt, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("failed to update agent lead status: %w", err)
	}
	return d.GetAgentLead(ctx, &store.FindAgentLead{ID: &id, TenantID: &tenantID})
}

type agentLeadScanner interface {
	Scan(dest ...interface{}) error
}

func scanAgentLead(scanner agentLeadScanner) (*store.AgentLead, error) {
	var lead store.AgentLead
	var transcriptID, email, phone, topic, location, detectedIntent sql.NullString
	var convertedAt sql.NullTime
	if err := scanner.Scan(
		&lead.ID, &lead.TenantID, &lead.SessionID, &transcriptID, &lead.Name, &email, &phone,
		&topic, &location, &detectedIntent, &lead.Status, &lead.CreatedAt, &lead.UpdatedAt,
		&lead.LastMessageAt, &convertedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan agent lead: %w", err)
	}
	lead.TranscriptID = transcriptID.String
	lead.Email = email.String
	lead.Phone = phone.String
	lead.Topic = topic.String
	lead.Location = location.String
	lead.DetectedIntent = detectedIntent.String
	if convertedAt.Valid {
		lead.ConvertedAt = &convertedAt.Time
	}
	return &lead, nil
}

// Reindex Checkpoint methods

func (d *DB) UpsertReindexCheckpoint(ctx context.Context, checkpoint *store.ReindexCheckpoint) (*store.ReindexCheckpoint, error) {
	return nil, errNotImplemented
}

func (d *DB) GetReindexCheckpoint(ctx context.Context, find *store.FindReindexCheckpoint) (*store.ReindexCheckpoint, error) {
	return nil, errNotImplemented
}

func (d *DB) DeleteReindexCheckpoint(ctx context.Context, tenantID int32, audience string) error {
	return errNotImplemented
}

func (d *DB) SupportsBridgeDelivery() bool {
	return false
}
