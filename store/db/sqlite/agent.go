package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/usememos/memos/store"
)

// ============================================================================
// TENANT OPERATIONS
// ============================================================================

func (d *DB) CreateAgentTenant(ctx context.Context, tenant *store.AgentTenant) (*store.AgentTenant, error) {
	stmt := `
		INSERT INTO agent_tenants (slug, company_name, vertical, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	now := time.Now()
	if err := d.db.QueryRowContext(ctx, stmt,
		tenant.Slug, tenant.CompanyName, tenant.Vertical, tenant.IsActive, now, now,
	).Scan(&tenant.ID); err != nil {
		return nil, err
	}
	tenant.CreatedAt = now
	tenant.UpdatedAt = now
	return tenant, nil
}

func (d *DB) GetAgentTenant(ctx context.Context, find *store.FindAgentTenant) (*store.AgentTenant, error) {
	tenants, err := d.ListAgentTenants(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(tenants) == 0 {
		return nil, nil
	}
	return tenants[0], nil
}

func (d *DB) ListAgentTenants(ctx context.Context, find *store.FindAgentTenant) ([]*store.AgentTenant, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.ID != nil {
		where = append(where, "id = ?")
		args = append(args, *find.ID)
	}
	if find.Slug != nil {
		where = append(where, "slug = ?")
		args = append(args, *find.Slug)
	}
	if find.IsActive != nil {
		where = append(where, "is_active = ?")
		args = append(args, *find.IsActive)
	}

	query := fmt.Sprintf(`
		SELECT id, slug, company_name, vertical, is_active, processing_options, created_at, updated_at
		FROM agent_tenants
		WHERE %s
		ORDER BY created_at DESC
	`, strings.Join(where, " AND "))

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []*store.AgentTenant
	for rows.Next() {
		var t store.AgentTenant
		var vertical, processingOptions sql.NullString
		if err := rows.Scan(&t.ID, &t.Slug, &t.CompanyName, &vertical, &t.IsActive, &processingOptions, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if vertical.Valid {
			t.Vertical = vertical.String
		}
		if processingOptions.Valid {
			t.ProcessingOptions = processingOptions.String
		}
		tenants = append(tenants, &t)
	}
	return tenants, rows.Err()
}

func (d *DB) UpdateAgentTenant(ctx context.Context, tenant *store.AgentTenant) (*store.AgentTenant, error) {
	stmt := `
		UPDATE agent_tenants
		SET company_name = ?, vertical = ?, is_active = ?, processing_options = ?, updated_at = ?
		WHERE id = ?
	`
	now := time.Now()
	var processingOptions interface{} = nil
	if tenant.ProcessingOptions != "" {
		processingOptions = tenant.ProcessingOptions
	}
	_, err := d.db.ExecContext(ctx, stmt, tenant.CompanyName, tenant.Vertical, tenant.IsActive, processingOptions, now, tenant.ID)
	if err != nil {
		return nil, err
	}
	tenant.UpdatedAt = now
	return tenant, nil
}

func (d *DB) DeleteAgentTenant(ctx context.Context, id int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_tenants WHERE id = ?", id)
	return err
}

// ============================================================================
// AUDIENCE OPERATIONS
// ============================================================================

func (d *DB) CreateAgentAudience(ctx context.Context, audience *store.AgentAudience) (*store.AgentAudience, error) {
	guidelinesJSON, _ := json.Marshal(audience.Guidelines)
	secondaryPhonesJSON, _ := json.Marshal(audience.SecondaryPhones)

	stmt := `
		INSERT INTO agent_audiences (
			tenant_id, audience_type, role, tone, brand_voice, guidelines,
			emergency_phone, secondary_phones, email, address,
			emergency_urgency_threshold, escalation_confidence_threshold, rate_limit_rpm, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	now := time.Now()
	if err := d.db.QueryRowContext(ctx, stmt,
		audience.TenantID, audience.AudienceType, audience.Role, audience.Tone, audience.BrandVoice,
		string(guidelinesJSON), audience.EmergencyPhone, string(secondaryPhonesJSON),
		audience.Email, audience.Address, audience.EmergencyUrgencyThreshold,
		audience.EscalationConfidenceThreshold, audience.RateLimitRPM, now,
	).Scan(&audience.ID); err != nil {
		return nil, err
	}
	audience.UpdatedAt = now
	return audience, nil
}

func (d *DB) GetAgentAudience(ctx context.Context, find *store.FindAgentAudience) (*store.AgentAudience, error) {
	audiences, err := d.ListAgentAudiences(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(audiences) == 0 {
		return nil, nil
	}
	return audiences[0], nil
}

func (d *DB) ListAgentAudiences(ctx context.Context, find *store.FindAgentAudience) ([]*store.AgentAudience, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.AudienceType != nil {
		where = append(where, "audience_type = ?")
		args = append(args, *find.AudienceType)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, audience_type, role, tone, brand_voice, guidelines,
			emergency_phone, secondary_phones, email, address,
			emergency_urgency_threshold, escalation_confidence_threshold, rate_limit_rpm, updated_at
		FROM agent_audiences
		WHERE %s
	`, strings.Join(where, " AND "))

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var audiences []*store.AgentAudience
	for rows.Next() {
		var a store.AgentAudience
		var guidelinesJSON, secondaryPhonesJSON, brandVoice, email, address sql.NullString
		if err := rows.Scan(
			&a.ID, &a.TenantID, &a.AudienceType, &a.Role, &a.Tone, &brandVoice, &guidelinesJSON,
			&a.EmergencyPhone, &secondaryPhonesJSON, &email, &address,
			&a.EmergencyUrgencyThreshold, &a.EscalationConfidenceThreshold, &a.RateLimitRPM, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if brandVoice.Valid {
			a.BrandVoice = brandVoice.String
		}
		if guidelinesJSON.Valid {
			json.Unmarshal([]byte(guidelinesJSON.String), &a.Guidelines)
		}
		if secondaryPhonesJSON.Valid {
			json.Unmarshal([]byte(secondaryPhonesJSON.String), &a.SecondaryPhones)
		}
		if email.Valid {
			a.Email = email.String
		}
		if address.Valid {
			a.Address = address.String
		}
		audiences = append(audiences, &a)
	}
	return audiences, rows.Err()
}

func (d *DB) UpdateAgentAudience(ctx context.Context, audience *store.AgentAudience) (*store.AgentAudience, error) {
	guidelinesJSON, _ := json.Marshal(audience.Guidelines)
	secondaryPhonesJSON, _ := json.Marshal(audience.SecondaryPhones)

	stmt := `
		UPDATE agent_audiences
		SET role = ?, tone = ?, brand_voice = ?, guidelines = ?,
			emergency_phone = ?, secondary_phones = ?, email = ?, address = ?,
			emergency_urgency_threshold = ?, escalation_confidence_threshold = ?,
			rate_limit_rpm = ?, updated_at = ?
		WHERE tenant_id = ? AND audience_type = ?
	`
	now := time.Now()
	_, err := d.db.ExecContext(ctx, stmt,
		audience.Role, audience.Tone, audience.BrandVoice, string(guidelinesJSON),
		audience.EmergencyPhone, string(secondaryPhonesJSON), audience.Email, audience.Address,
		audience.EmergencyUrgencyThreshold, audience.EscalationConfidenceThreshold,
		audience.RateLimitRPM, now, audience.TenantID, audience.AudienceType,
	)
	if err != nil {
		return nil, err
	}
	audience.UpdatedAt = now
	return audience, nil
}

func (d *DB) DeleteAgentAudience(ctx context.Context, tenantID int32, audienceType string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_audiences WHERE tenant_id = ? AND audience_type = ?", tenantID, audienceType)
	return err
}

// ============================================================================
// SERVICE OPERATIONS
// ============================================================================

func (d *DB) CreateAgentService(ctx context.Context, service *store.AgentService) (*store.AgentService, error) {
	stmt := `
		INSERT INTO agent_services (tenant_id, audience_type, code, name, description, is_emergency, response_time, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	if err := d.db.QueryRowContext(ctx, stmt,
		service.TenantID, service.AudienceType, service.Code, service.Name,
		service.Description, service.IsEmergency, service.ResponseTime, service.IsActive,
	).Scan(&service.ID); err != nil {
		return nil, err
	}
	return service, nil
}

func (d *DB) ListAgentServices(ctx context.Context, find *store.FindAgentService) ([]*store.AgentService, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.AudienceType != nil {
		where = append(where, "audience_type = ?")
		args = append(args, *find.AudienceType)
	}
	if find.Code != nil {
		where = append(where, "code = ?")
		args = append(args, *find.Code)
	}
	if find.IsActive != nil {
		where = append(where, "is_active = ?")
		args = append(args, *find.IsActive)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, audience_type, code, name, description, is_emergency, response_time, is_active
		FROM agent_services
		WHERE %s
		ORDER BY name
	`, strings.Join(where, " AND "))

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []*store.AgentService
	for rows.Next() {
		var s store.AgentService
		var description, responseTime sql.NullString
		if err := rows.Scan(&s.ID, &s.TenantID, &s.AudienceType, &s.Code, &s.Name,
			&description, &s.IsEmergency, &responseTime, &s.IsActive); err != nil {
			return nil, err
		}
		if description.Valid {
			s.Description = description.String
		}
		if responseTime.Valid {
			s.ResponseTime = responseTime.String
		}
		services = append(services, &s)
	}
	return services, rows.Err()
}

func (d *DB) DeleteAgentServices(ctx context.Context, tenantID int32, audienceType string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_services WHERE tenant_id = ? AND audience_type = ?", tenantID, audienceType)
	return err
}

// ============================================================================
// EXCLUSION OPERATIONS
// ============================================================================

func (d *DB) CreateAgentExclusion(ctx context.Context, exclusion *store.AgentExclusion) (*store.AgentExclusion, error) {
	stmt := `
		INSERT INTO agent_exclusions (tenant_id, audience_type, code, name, description, exception_rule, referral, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	if err := d.db.QueryRowContext(ctx, stmt,
		exclusion.TenantID, exclusion.AudienceType, exclusion.Code, exclusion.Name,
		exclusion.Description, exclusion.ExceptionRule, exclusion.Referral, exclusion.IsActive,
	).Scan(&exclusion.ID); err != nil {
		return nil, err
	}
	return exclusion, nil
}

func (d *DB) ListAgentExclusions(ctx context.Context, find *store.FindAgentExclusion) ([]*store.AgentExclusion, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.AudienceType != nil {
		where = append(where, "audience_type = ?")
		args = append(args, *find.AudienceType)
	}
	if find.IsActive != nil {
		where = append(where, "is_active = ?")
		args = append(args, *find.IsActive)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, audience_type, code, name, description, exception_rule, referral, is_active
		FROM agent_exclusions
		WHERE %s
	`, strings.Join(where, " AND "))

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exclusions []*store.AgentExclusion
	for rows.Next() {
		var e store.AgentExclusion
		var description, exceptionRule, referral sql.NullString
		if err := rows.Scan(&e.ID, &e.TenantID, &e.AudienceType, &e.Code, &e.Name,
			&description, &exceptionRule, &referral, &e.IsActive); err != nil {
			return nil, err
		}
		if description.Valid {
			e.Description = description.String
		}
		if exceptionRule.Valid {
			e.ExceptionRule = exceptionRule.String
		}
		if referral.Valid {
			e.Referral = referral.String
		}
		exclusions = append(exclusions, &e)
	}
	return exclusions, rows.Err()
}

func (d *DB) DeleteAgentExclusions(ctx context.Context, tenantID int32, audienceType string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_exclusions WHERE tenant_id = ? AND audience_type = ?", tenantID, audienceType)
	return err
}

// ============================================================================
// COVERAGE OPERATIONS
// ============================================================================

func (d *DB) CreateAgentCoverage(ctx context.Context, coverage *store.AgentCoverage) (*store.AgentCoverage, error) {
	stmt := `
		INSERT INTO agent_coverage (tenant_id, area_type, area_name, state_code, is_included)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id
	`
	if err := d.db.QueryRowContext(ctx, stmt,
		coverage.TenantID, coverage.AreaType, coverage.AreaName, coverage.StateCode, coverage.IsIncluded,
	).Scan(&coverage.ID); err != nil {
		return nil, err
	}
	return coverage, nil
}

func (d *DB) ListAgentCoverage(ctx context.Context, find *store.FindAgentCoverage) ([]*store.AgentCoverage, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.IsIncluded != nil {
		where = append(where, "is_included = ?")
		args = append(args, *find.IsIncluded)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, area_type, area_name, state_code, is_included
		FROM agent_coverage
		WHERE %s
	`, strings.Join(where, " AND "))

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var coverage []*store.AgentCoverage
	for rows.Next() {
		var c store.AgentCoverage
		var stateCode sql.NullString
		if err := rows.Scan(&c.ID, &c.TenantID, &c.AreaType, &c.AreaName, &stateCode, &c.IsIncluded); err != nil {
			return nil, err
		}
		if stateCode.Valid {
			c.StateCode = stateCode.String
		}
		coverage = append(coverage, &c)
	}
	return coverage, rows.Err()
}

func (d *DB) DeleteAgentCoverage(ctx context.Context, tenantID int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_coverage WHERE tenant_id = ?", tenantID)
	return err
}

// ============================================================================
// FAQ OPERATIONS
// ============================================================================

func (d *DB) CreateAgentFAQ(ctx context.Context, faq *store.AgentFAQ) (*store.AgentFAQ, error) {
	stmt := `
		INSERT INTO agent_faqs (tenant_id, audience_type, code, question, answer, is_active)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	if err := d.db.QueryRowContext(ctx, stmt,
		faq.TenantID, faq.AudienceType, faq.Code, faq.Question, faq.Answer, faq.IsActive,
	).Scan(&faq.ID); err != nil {
		return nil, err
	}
	return faq, nil
}

func (d *DB) ListAgentFAQs(ctx context.Context, find *store.FindAgentFAQ) ([]*store.AgentFAQ, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.AudienceType != nil {
		where = append(where, "audience_type = ?")
		args = append(args, *find.AudienceType)
	}
	if find.IsActive != nil {
		where = append(where, "is_active = ?")
		args = append(args, *find.IsActive)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, audience_type, code, question, answer, is_active
		FROM agent_faqs
		WHERE %s
	`, strings.Join(where, " AND "))

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var faqs []*store.AgentFAQ
	for rows.Next() {
		var f store.AgentFAQ
		if err := rows.Scan(&f.ID, &f.TenantID, &f.AudienceType, &f.Code, &f.Question, &f.Answer, &f.IsActive); err != nil {
			return nil, err
		}
		faqs = append(faqs, &f)
	}
	return faqs, rows.Err()
}

func (d *DB) DeleteAgentFAQs(ctx context.Context, tenantID int32, audienceType string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_faqs WHERE tenant_id = ? AND audience_type = ?", tenantID, audienceType)
	return err
}

// ============================================================================
// SAFETY PROTOCOL OPERATIONS
// ============================================================================

func (d *DB) CreateAgentSafetyProtocol(ctx context.Context, protocol *store.AgentSafetyProtocol) (*store.AgentSafetyProtocol, error) {
	triggerIntentsJSON, _ := json.Marshal(protocol.TriggerIntents)
	instructionsJSON, _ := json.Marshal(protocol.Instructions)

	stmt := `
		INSERT INTO agent_safety_protocols (tenant_id, audience_type, code, name, trigger_intents, instructions, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	if err := d.db.QueryRowContext(ctx, stmt,
		protocol.TenantID, protocol.AudienceType, protocol.Code, protocol.Name,
		string(triggerIntentsJSON), string(instructionsJSON), protocol.IsActive,
	).Scan(&protocol.ID); err != nil {
		return nil, err
	}
	return protocol, nil
}

func (d *DB) ListAgentSafetyProtocols(ctx context.Context, find *store.FindAgentSafetyProtocol) ([]*store.AgentSafetyProtocol, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.AudienceType != nil {
		where = append(where, "audience_type = ?")
		args = append(args, *find.AudienceType)
	}
	if find.IsActive != nil {
		where = append(where, "is_active = ?")
		args = append(args, *find.IsActive)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, audience_type, code, name, trigger_intents, instructions, is_active
		FROM agent_safety_protocols
		WHERE %s
	`, strings.Join(where, " AND "))

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var protocols []*store.AgentSafetyProtocol
	for rows.Next() {
		var p store.AgentSafetyProtocol
		var triggerIntentsJSON, instructionsJSON string
		if err := rows.Scan(&p.ID, &p.TenantID, &p.AudienceType, &p.Code, &p.Name,
			&triggerIntentsJSON, &instructionsJSON, &p.IsActive); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(triggerIntentsJSON), &p.TriggerIntents)
		json.Unmarshal([]byte(instructionsJSON), &p.Instructions)
		protocols = append(protocols, &p)
	}
	return protocols, rows.Err()
}

func (d *DB) DeleteAgentSafetyProtocols(ctx context.Context, tenantID int32, audienceType string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_safety_protocols WHERE tenant_id = ? AND audience_type = ?", tenantID, audienceType)
	return err
}

// ============================================================================
// KB SECTION OPERATIONS
// ============================================================================

func (d *DB) CreateAgentKBSection(ctx context.Context, section *store.AgentKBSection) (*store.AgentKBSection, error) {
	stmt := `
		INSERT INTO agent_kb_sections (tenant_id, audience_type, code, title, content, section_type, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	if err := d.db.QueryRowContext(ctx, stmt,
		section.TenantID, section.AudienceType, section.Code, section.Title,
		section.Content, section.SectionType, section.IsActive,
	).Scan(&section.ID); err != nil {
		return nil, err
	}
	return section, nil
}

func (d *DB) ListAgentKBSections(ctx context.Context, find *store.FindAgentKBSection) ([]*store.AgentKBSection, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.AudienceType != nil {
		where = append(where, "audience_type = ?")
		args = append(args, *find.AudienceType)
	}
	if find.SectionType != nil {
		where = append(where, "section_type = ?")
		args = append(args, *find.SectionType)
	}
	if find.IsActive != nil {
		where = append(where, "is_active = ?")
		args = append(args, *find.IsActive)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, audience_type, code, title, content, section_type, is_active
		FROM agent_kb_sections
		WHERE %s
	`, strings.Join(where, " AND "))

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sections []*store.AgentKBSection
	for rows.Next() {
		var s store.AgentKBSection
		var sectionType sql.NullString
		if err := rows.Scan(&s.ID, &s.TenantID, &s.AudienceType, &s.Code, &s.Title,
			&s.Content, &sectionType, &s.IsActive); err != nil {
			return nil, err
		}
		if sectionType.Valid {
			s.SectionType = sectionType.String
		}
		sections = append(sections, &s)
	}
	return sections, rows.Err()
}

func (d *DB) DeleteAgentKBSections(ctx context.Context, tenantID int32, audienceType string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_kb_sections WHERE tenant_id = ? AND audience_type = ?", tenantID, audienceType)
	return err
}

// ============================================================================
// INTENT OPERATIONS
// ============================================================================

func (d *DB) CreateAgentIntent(ctx context.Context, intent *store.AgentIntent) (*store.AgentIntent, error) {
	examplesJSON, _ := json.Marshal(intent.Examples)
	counterExamplesJSON, _ := json.Marshal(intent.CounterExamples)

	stmt := `
		INSERT INTO agent_intents (
			tenant_id, audience_type, code, name, category, description,
			examples, counter_examples, urgency, action, confidence_threshold, is_active
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	if err := d.db.QueryRowContext(ctx, stmt,
		intent.TenantID, intent.AudienceType, intent.Code, intent.Name, intent.Category,
		intent.Description, string(examplesJSON), string(counterExamplesJSON),
		intent.Urgency, intent.Action, intent.ConfidenceThreshold, intent.IsActive,
	).Scan(&intent.ID); err != nil {
		return nil, err
	}
	return intent, nil
}

func (d *DB) ListAgentIntents(ctx context.Context, find *store.FindAgentIntent) ([]*store.AgentIntent, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.TenantID != nil {
		where = append(where, "(tenant_id = ? OR tenant_id IS NULL)")
		args = append(args, *find.TenantID)
	}
	if find.AudienceType != nil {
		where = append(where, "(audience_type = ? OR audience_type IS NULL)")
		args = append(args, *find.AudienceType)
	}
	if find.Category != nil {
		where = append(where, "category = ?")
		args = append(args, *find.Category)
	}
	if find.IsActive != nil {
		where = append(where, "is_active = ?")
		args = append(args, *find.IsActive)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, audience_type, code, name, category, description,
			examples, counter_examples, urgency, action, confidence_threshold, is_active
		FROM agent_intents
		WHERE %s
		ORDER BY urgency DESC, name
	`, strings.Join(where, " AND "))

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var intents []*store.AgentIntent
	for rows.Next() {
		var i store.AgentIntent
		var tenantID, urgency sql.NullInt32
		var audienceType, examplesJSON, counterExamplesJSON sql.NullString
		var confidenceThreshold sql.NullFloat64
		if err := rows.Scan(&i.ID, &tenantID, &audienceType, &i.Code, &i.Name, &i.Category,
			&i.Description, &examplesJSON, &counterExamplesJSON, &urgency, &i.Action,
			&confidenceThreshold, &i.IsActive); err != nil {
			return nil, err
		}
		if tenantID.Valid {
			tid := tenantID.Int32
			i.TenantID = &tid
		}
		if audienceType.Valid {
			at := audienceType.String
			i.AudienceType = &at
		}
		if examplesJSON.Valid {
			json.Unmarshal([]byte(examplesJSON.String), &i.Examples)
		}
		if counterExamplesJSON.Valid {
			json.Unmarshal([]byte(counterExamplesJSON.String), &i.CounterExamples)
		}
		if urgency.Valid {
			i.Urgency = int(urgency.Int32)
		}
		if confidenceThreshold.Valid {
			i.ConfidenceThreshold = confidenceThreshold.Float64
		}
		intents = append(intents, &i)
	}
	return intents, rows.Err()
}

func (d *DB) DeleteAgentIntents(ctx context.Context, tenantID int32, audienceType *string) error {
	if audienceType != nil {
		_, err := d.db.ExecContext(ctx, "DELETE FROM agent_intents WHERE tenant_id = ? AND audience_type = ?", tenantID, *audienceType)
		return err
	}
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_intents WHERE tenant_id = ?", tenantID)
	return err
}

// ============================================================================
// RULE OPERATIONS
// ============================================================================

func (d *DB) CreateAgentRule(ctx context.Context, rule *store.AgentRule) (*store.AgentRule, error) {
	stmt := `
		INSERT INTO agent_rules (tenant_id, audience_type, code, name, description, priority, applies_to, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	if err := d.db.QueryRowContext(ctx, stmt,
		rule.TenantID, rule.AudienceType, rule.Code, rule.Name,
		rule.Description, rule.Priority, rule.AppliesTo, rule.IsActive,
	).Scan(&rule.ID); err != nil {
		return nil, err
	}
	return rule, nil
}

func (d *DB) ListAgentRules(ctx context.Context, find *store.FindAgentRule) ([]*store.AgentRule, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.AudienceType != nil {
		where = append(where, "audience_type = ?")
		args = append(args, *find.AudienceType)
	}
	if find.IsActive != nil {
		where = append(where, "is_active = ?")
		args = append(args, *find.IsActive)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, audience_type, code, name, description, priority, applies_to, is_active
		FROM agent_rules
		WHERE %s
		ORDER BY priority
	`, strings.Join(where, " AND "))

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*store.AgentRule
	for rows.Next() {
		var r store.AgentRule
		var appliesTo sql.NullString
		if err := rows.Scan(&r.ID, &r.TenantID, &r.AudienceType, &r.Code, &r.Name,
			&r.Description, &r.Priority, &appliesTo, &r.IsActive); err != nil {
			return nil, err
		}
		if appliesTo.Valid {
			r.AppliesTo = appliesTo.String
		}
		rules = append(rules, &r)
	}
	return rules, rows.Err()
}

func (d *DB) DeleteAgentRules(ctx context.Context, tenantID int32, audienceType string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_rules WHERE tenant_id = ? AND audience_type = ?", tenantID, audienceType)
	return err
}

// ============================================================================
// SESSION OPERATIONS
// ============================================================================

func (d *DB) CreateAgentSession(ctx context.Context, session *store.AgentSession) (*store.AgentSession, error) {
	messagesJSON, _ := json.Marshal(session.Messages)

	stmt := `
		INSERT INTO agent_sessions (
			id, tenant_id, user_id, audience_type, phase, current_intent,
			urgency_level, coverage_status, customer_name, customer_phone,
			customer_location, detected_service, message_count, messages,
			created_at, updated_at, is_completed
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	_, err := d.db.ExecContext(ctx, stmt,
		session.ID, session.TenantID, session.UserID, session.AudienceType,
		session.Phase, session.CurrentIntent, session.UrgencyLevel, session.CoverageStatus,
		session.CustomerName, session.CustomerPhone, session.CustomerLocation,
		session.DetectedService, session.MessageCount, string(messagesJSON),
		now, now, session.IsCompleted,
	)
	if err != nil {
		return nil, err
	}
	session.CreatedAt = now
	session.UpdatedAt = now
	return session, nil
}

func (d *DB) GetAgentSession(ctx context.Context, find *store.FindAgentSession) (*store.AgentSession, error) {
	sessions, err := d.ListAgentSessions(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, nil
	}
	return sessions[0], nil
}

func (d *DB) ListAgentSessions(ctx context.Context, find *store.FindAgentSession) ([]*store.AgentSession, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.ID != nil {
		where = append(where, "id = ?")
		args = append(args, *find.ID)
	}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.UserID != nil {
		where = append(where, "user_id = ?")
		args = append(args, *find.UserID)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, user_id, audience_type, phase, current_intent,
			urgency_level, coverage_status, customer_name, customer_phone,
			customer_location, detected_service, message_count, messages,
			created_at, updated_at, completed_at, is_completed, completion_reason
		FROM agent_sessions
		WHERE %s
		ORDER BY updated_at DESC
	`, strings.Join(where, " AND "))

	if find.Limit != nil && *find.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", *find.Limit)
	}

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*store.AgentSession
	for rows.Next() {
		var s store.AgentSession
		var userID sql.NullInt32
		var currentIntent, coverageStatus, customerName, customerPhone, customerLocation, detectedService, completionReason sql.NullString
		var completedAt sql.NullTime
		var messagesJSON string
		if err := rows.Scan(
			&s.ID, &s.TenantID, &userID, &s.AudienceType, &s.Phase, &currentIntent,
			&s.UrgencyLevel, &coverageStatus, &customerName, &customerPhone,
			&customerLocation, &detectedService, &s.MessageCount, &messagesJSON,
			&s.CreatedAt, &s.UpdatedAt, &completedAt, &s.IsCompleted, &completionReason,
		); err != nil {
			return nil, err
		}
		if userID.Valid {
			uid := userID.Int32
			s.UserID = &uid
		}
		if currentIntent.Valid {
			s.CurrentIntent = currentIntent.String
		}
		if coverageStatus.Valid {
			s.CoverageStatus = coverageStatus.String
		}
		if customerName.Valid {
			s.CustomerName = customerName.String
		}
		if customerPhone.Valid {
			s.CustomerPhone = customerPhone.String
		}
		if customerLocation.Valid {
			s.CustomerLocation = customerLocation.String
		}
		if detectedService.Valid {
			s.DetectedService = detectedService.String
		}
		if completedAt.Valid {
			s.CompletedAt = &completedAt.Time
		}
		if completionReason.Valid {
			s.CompletionReason = completionReason.String
		}
		json.Unmarshal([]byte(messagesJSON), &s.Messages)
		sessions = append(sessions, &s)
	}
	return sessions, rows.Err()
}

func (d *DB) UpdateAgentSession(ctx context.Context, update *store.UpdateAgentSession) (*store.AgentSession, error) {
	set, args := []string{}, []interface{}{}
	if update.Phase != nil {
		set = append(set, "phase = ?")
		args = append(args, *update.Phase)
	}
	if update.CurrentIntent != nil {
		set = append(set, "current_intent = ?")
		args = append(args, *update.CurrentIntent)
	}
	if update.UrgencyLevel != nil {
		set = append(set, "urgency_level = ?")
		args = append(args, *update.UrgencyLevel)
	}
	if update.CoverageStatus != nil {
		set = append(set, "coverage_status = ?")
		args = append(args, *update.CoverageStatus)
	}
	if update.CustomerName != nil {
		set = append(set, "customer_name = ?")
		args = append(args, *update.CustomerName)
	}
	if update.CustomerPhone != nil {
		set = append(set, "customer_phone = ?")
		args = append(args, *update.CustomerPhone)
	}
	if update.CustomerLocation != nil {
		set = append(set, "customer_location = ?")
		args = append(args, *update.CustomerLocation)
	}
	if update.DetectedService != nil {
		set = append(set, "detected_service = ?")
		args = append(args, *update.DetectedService)
	}
	if update.MessageCount != nil {
		set = append(set, "message_count = ?")
		args = append(args, *update.MessageCount)
	}
	if update.Messages != nil {
		messagesJSON, _ := json.Marshal(update.Messages)
		set = append(set, "messages = ?")
		args = append(args, string(messagesJSON))
	}
	if update.CompletedAt != nil {
		set = append(set, "completed_at = ?")
		args = append(args, *update.CompletedAt)
	}
	if update.IsCompleted != nil {
		set = append(set, "is_completed = ?")
		args = append(args, *update.IsCompleted)
	}
	if update.CompletionReason != nil {
		set = append(set, "completion_reason = ?")
		args = append(args, *update.CompletionReason)
	}

	now := time.Now()
	set = append(set, "updated_at = ?")
	args = append(args, now)
	args = append(args, update.ID)

	stmt := fmt.Sprintf("UPDATE agent_sessions SET %s WHERE id = ?", strings.Join(set, ", "))
	_, err := d.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}

	return d.GetAgentSession(ctx, &store.FindAgentSession{ID: &update.ID})
}

func (d *DB) DeleteAgentSession(ctx context.Context, id string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_sessions WHERE id = ?", id)
	return err
}

// ============================================================================
// SOURCE FILE OPERATIONS
// ============================================================================

func (d *DB) UpsertAgentSourceFile(ctx context.Context, file *store.AgentSourceFile) (*store.AgentSourceFile, error) {
	// Calculate next version number for this tenant+audience+filetype combination
	var nextVersion int32 = 1
	err := d.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM agent_source_files
		WHERE tenant_id = ? AND audience_type = ? AND file_type = ?
	`, file.TenantID, file.AudienceType, file.FileType).Scan(&nextVersion)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get next version: %w", err)
	}

	// Insert new version (versioning - keep all previous versions)
	stmt := `
		INSERT INTO agent_source_files (tenant_id, audience_type, file_type, content, content_hash, version, imported_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`
	now := time.Now()
	if err := d.db.QueryRowContext(ctx, stmt,
		file.TenantID, file.AudienceType, file.FileType, file.Content, file.ContentHash, nextVersion, now,
	).Scan(&file.ID); err != nil {
		return nil, err
	}
	file.Version = nextVersion
	file.ImportedAt = now
	return file, nil
}

func (d *DB) GetAgentSourceFile(ctx context.Context, find *store.FindAgentSourceFile) (*store.AgentSourceFile, error) {
	files, err := d.ListAgentSourceFiles(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}
	return files[0], nil
}

func (d *DB) ListAgentSourceFiles(ctx context.Context, find *store.FindAgentSourceFile) ([]*store.AgentSourceFile, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.ID != nil {
		where = append(where, "id = ?")
		args = append(args, *find.ID)
	}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.AudienceType != nil {
		where = append(where, "audience_type = ?")
		args = append(args, *find.AudienceType)
	}
	if find.FileType != nil {
		where = append(where, "file_type = ?")
		args = append(args, *find.FileType)
	}
	if find.Version != nil {
		where = append(where, "version = ?")
		args = append(args, *find.Version)
	}

	var query string
	if find.LatestOnly {
		// Get only the latest version per tenant+audience+filetype
		query = fmt.Sprintf(`
			SELECT id, tenant_id, audience_type, file_type, content, content_hash, COALESCE(version, 1), imported_at
			FROM agent_source_files
			WHERE %s
			AND (tenant_id, audience_type, file_type, version) IN (
				SELECT tenant_id, audience_type, file_type, MAX(version)
				FROM agent_source_files
				GROUP BY tenant_id, audience_type, file_type
			)
			ORDER BY imported_at DESC
		`, strings.Join(where, " AND "))
	} else {
		query = fmt.Sprintf(`
			SELECT id, tenant_id, audience_type, file_type, content, content_hash, COALESCE(version, 1), imported_at
			FROM agent_source_files
			WHERE %s
			ORDER BY version DESC, imported_at DESC
		`, strings.Join(where, " AND "))
	}

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*store.AgentSourceFile
	for rows.Next() {
		var f store.AgentSourceFile
		if err := rows.Scan(&f.ID, &f.TenantID, &f.AudienceType, &f.FileType,
			&f.Content, &f.ContentHash, &f.Version, &f.ImportedAt); err != nil {
			return nil, err
		}
		files = append(files, &f)
	}
	return files, rows.Err()
}

func (d *DB) DeleteAgentSourceFiles(ctx context.Context, tenantID int32, audienceType *string) error {
	if audienceType != nil {
		_, err := d.db.ExecContext(ctx, "DELETE FROM agent_source_files WHERE tenant_id = ? AND audience_type = ?", tenantID, *audienceType)
		return err
	}
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_source_files WHERE tenant_id = ?", tenantID)
	return err
}

// ============================================================================
// RATE LIMIT OPERATIONS
// ============================================================================

func (d *DB) GetOrCreateAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) (*store.AgentRateLimit, error) {
	// Try to get existing
	var rl store.AgentRateLimit
	err := d.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, audience_type, client_ip, request_count, window_start
		FROM agent_rate_limits
		WHERE tenant_id = ? AND audience_type = ? AND client_ip = ?
	`, tenantID, audienceType, clientIP).Scan(
		&rl.ID, &rl.TenantID, &rl.AudienceType, &rl.ClientIP, &rl.RequestCount, &rl.WindowStart,
	)
	if err == nil {
		return &rl, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create new
	now := time.Now()
	err = d.db.QueryRowContext(ctx, `
		INSERT INTO agent_rate_limits (tenant_id, audience_type, client_ip, request_count, window_start)
		VALUES (?, ?, ?, 0, ?)
		RETURNING id
	`, tenantID, audienceType, clientIP, now).Scan(&rl.ID)
	if err != nil {
		return nil, err
	}
	rl.TenantID = tenantID
	rl.AudienceType = audienceType
	rl.ClientIP = clientIP
	rl.RequestCount = 0
	rl.WindowStart = now
	return &rl, nil
}

func (d *DB) IncrementAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) error {
	_, err := d.db.ExecContext(ctx, `
		UPDATE agent_rate_limits
		SET request_count = request_count + 1
		WHERE tenant_id = ? AND audience_type = ? AND client_ip = ?
	`, tenantID, audienceType, clientIP)
	return err
}

func (d *DB) ResetAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) error {
	now := time.Now()
	_, err := d.db.ExecContext(ctx, `
		UPDATE agent_rate_limits
		SET request_count = 0, window_start = ?
		WHERE tenant_id = ? AND audience_type = ? AND client_ip = ?
	`, now, tenantID, audienceType, clientIP)
	return err
}

// ============================================================================
// SIMULATION TRANSCRIPT OPERATIONS
// ============================================================================

func (d *DB) CreateAgentSimulationTranscript(ctx context.Context, transcript *store.AgentSimulationTranscript) (*store.AgentSimulationTranscript, error) {
	messagesJSON, _ := json.Marshal(transcript.Messages)

	stmt := `
		INSERT INTO agent_simulation_transcripts (
			id, tenant_id, user_id, initial_prompt, persona_hint,
			total_turns, end_reason, messages, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	_, err := d.db.ExecContext(ctx, stmt,
		transcript.ID, transcript.TenantID, transcript.UserID,
		transcript.InitialPrompt, transcript.PersonaHint,
		transcript.TotalTurns, transcript.EndReason, string(messagesJSON), now,
	)
	if err != nil {
		return nil, err
	}
	transcript.CreatedAt = now
	return transcript, nil
}

func (d *DB) GetAgentSimulationTranscript(ctx context.Context, find *store.FindAgentSimulationTranscript) (*store.AgentSimulationTranscript, error) {
	transcripts, _, err := d.ListAgentSimulationTranscripts(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(transcripts) == 0 {
		return nil, nil
	}
	return transcripts[0], nil
}

func (d *DB) ListAgentSimulationTranscripts(ctx context.Context, find *store.FindAgentSimulationTranscript) ([]*store.AgentSimulationTranscript, int, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.ID != nil {
		where = append(where, "id = ?")
		args = append(args, *find.ID)
	}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.UserID != nil {
		where = append(where, "user_id = ?")
		args = append(args, *find.UserID)
	}

	// Count query
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM agent_simulation_transcripts WHERE %s
	`, strings.Join(where, " AND "))

	var total int
	if err := d.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// List query with pagination
	query := fmt.Sprintf(`
		SELECT id, tenant_id, user_id, initial_prompt, persona_hint,
			total_turns, end_reason, messages, created_at
		FROM agent_simulation_transcripts
		WHERE %s
		ORDER BY created_at DESC
	`, strings.Join(where, " AND "))

	if find.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", find.Limit)
		if find.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", find.Offset)
		}
	}

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var transcripts []*store.AgentSimulationTranscript
	for rows.Next() {
		var t store.AgentSimulationTranscript
		var personaHint sql.NullString
		var messagesJSON string
		if err := rows.Scan(
			&t.ID, &t.TenantID, &t.UserID, &t.InitialPrompt, &personaHint,
			&t.TotalTurns, &t.EndReason, &messagesJSON, &t.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		if personaHint.Valid {
			t.PersonaHint = personaHint.String
		}
		json.Unmarshal([]byte(messagesJSON), &t.Messages)
		transcripts = append(transcripts, &t)
	}
	return transcripts, total, rows.Err()
}

func (d *DB) DeleteAgentSimulationTranscript(ctx context.Context, id string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_simulation_transcripts WHERE id = ?", id)
	return err
}

// ============================================================================
// TENANT SCRIPT OPERATIONS (SCRIPT.MD)
// ============================================================================

func (d *DB) UpsertAgentTenantScript(ctx context.Context, script *store.AgentTenantScript) (*store.AgentTenantScript, error) {
	// Check if script exists for this tenant
	var existingID int32
	err := d.db.QueryRowContext(ctx, "SELECT id FROM agent_tenant_scripts WHERE tenant_id = ?", script.TenantID).Scan(&existingID)

	now := time.Now()
	if err == sql.ErrNoRows {
		// Insert new script
		stmt := `
			INSERT INTO agent_tenant_scripts (tenant_id, content, content_hash, summary, imported_at, version)
			VALUES (?, ?, ?, ?, ?, 1)
			RETURNING id
		`
		if err := d.db.QueryRowContext(ctx, stmt,
			script.TenantID, script.Content, script.ContentHash, script.Summary, now,
		).Scan(&script.ID); err != nil {
			return nil, err
		}
		script.Version = 1
	} else if err != nil {
		return nil, err
	} else {
		// Update existing script (increment version)
		stmt := `
			UPDATE agent_tenant_scripts
			SET content = ?, content_hash = ?, summary = ?, imported_at = ?, version = version + 1
			WHERE tenant_id = ?
			RETURNING id, version
		`
		if err := d.db.QueryRowContext(ctx, stmt,
			script.Content, script.ContentHash, script.Summary, now, script.TenantID,
		).Scan(&script.ID, &script.Version); err != nil {
			return nil, err
		}
	}
	script.ImportedAt = now
	return script, nil
}

func (d *DB) GetAgentTenantScript(ctx context.Context, find *store.FindAgentTenantScript) (*store.AgentTenantScript, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.ID != nil {
		where = append(where, "id = ?")
		args = append(args, *find.ID)
	}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, content, content_hash, summary, imported_at, version
		FROM agent_tenant_scripts
		WHERE %s
	`, strings.Join(where, " AND "))

	var s store.AgentTenantScript
	var summary sql.NullString
	err := d.db.QueryRowContext(ctx, query, args...).Scan(
		&s.ID, &s.TenantID, &s.Content, &s.ContentHash, &summary, &s.ImportedAt, &s.Version,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if summary.Valid {
		s.Summary = summary.String
	}
	return &s, nil
}

func (d *DB) DeleteAgentTenantScript(ctx context.Context, tenantID int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_tenant_scripts WHERE tenant_id = ?", tenantID)
	return err
}

// ============================================================================
// ANALYSIS RESULT OPERATIONS
// ============================================================================

func (d *DB) CreateAgentAnalysisResult(ctx context.Context, result *store.AgentAnalysisResult) (*store.AgentAnalysisResult, error) {
	breakdownJSON, _ := json.Marshal(result.Breakdown)
	issuesJSON, _ := json.Marshal(result.Issues)
	suggestionsJSON, _ := json.Marshal(result.Suggestions)

	stmt := `
		INSERT INTO agent_analysis_results (
			id, tenant_id, conversation_id, conversation_type, user_id,
			score, grade, breakdown, issues, suggestions, benchmark_version, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	_, err := d.db.ExecContext(ctx, stmt,
		result.ID, result.TenantID, result.ConversationID, result.ConversationType, result.UserID,
		result.Score, result.Grade, string(breakdownJSON), string(issuesJSON),
		string(suggestionsJSON), result.BenchmarkVersion, now,
	)
	if err != nil {
		return nil, err
	}
	result.CreatedAt = now
	return result, nil
}

func (d *DB) GetAgentAnalysisResult(ctx context.Context, find *store.FindAgentAnalysisResult) (*store.AgentAnalysisResult, error) {
	results, _, err := d.ListAgentAnalysisResults(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

func (d *DB) ListAgentAnalysisResults(ctx context.Context, find *store.FindAgentAnalysisResult) ([]*store.AgentAnalysisResult, int, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.ID != nil {
		where = append(where, "id = ?")
		args = append(args, *find.ID)
	}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.ConversationID != nil {
		where = append(where, "conversation_id = ?")
		args = append(args, *find.ConversationID)
	}
	if find.UserID != nil {
		where = append(where, "user_id = ?")
		args = append(args, *find.UserID)
	}

	// Count query
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM agent_analysis_results WHERE %s
	`, strings.Join(where, " AND "))

	var total int
	if err := d.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// List query with pagination
	query := fmt.Sprintf(`
		SELECT id, tenant_id, conversation_id, conversation_type, user_id,
			score, grade, breakdown, issues, suggestions, benchmark_version, created_at
		FROM agent_analysis_results
		WHERE %s
		ORDER BY created_at DESC
	`, strings.Join(where, " AND "))

	if find.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", find.Limit)
		if find.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", find.Offset)
		}
	}

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []*store.AgentAnalysisResult
	for rows.Next() {
		var r store.AgentAnalysisResult
		var breakdownJSON, issuesJSON, suggestionsJSON string
		var benchmarkVersion sql.NullString
		if err := rows.Scan(
			&r.ID, &r.TenantID, &r.ConversationID, &r.ConversationType, &r.UserID,
			&r.Score, &r.Grade, &breakdownJSON, &issuesJSON, &suggestionsJSON,
			&benchmarkVersion, &r.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		json.Unmarshal([]byte(breakdownJSON), &r.Breakdown)
		json.Unmarshal([]byte(issuesJSON), &r.Issues)
		json.Unmarshal([]byte(suggestionsJSON), &r.Suggestions)
		if benchmarkVersion.Valid {
			r.BenchmarkVersion = benchmarkVersion.String
		}
		results = append(results, &r)
	}
	return results, total, rows.Err()
}

// ============================================================================
// LEARNING MEMORY OPERATIONS
// ============================================================================

func (d *DB) GetOrCreateAgentLearningMemory(ctx context.Context, tenantID int32) (*store.AgentLearningMemory, error) {
	// Try to get existing
	query := `
		SELECT id, tenant_id, common_issues, learned_behaviors, improvement_areas,
			pending_suggestions, analysis_count, last_updated, version
		FROM agent_learning_memory
		WHERE tenant_id = ?
	`
	var m store.AgentLearningMemory
	var commonIssuesJSON, learnedBehaviorsJSON, improvementAreasJSON, pendingSuggestionsJSON string
	err := d.db.QueryRowContext(ctx, query, tenantID).Scan(
		&m.ID, &m.TenantID, &commonIssuesJSON, &learnedBehaviorsJSON,
		&improvementAreasJSON, &pendingSuggestionsJSON, &m.AnalysisCount,
		&m.LastUpdated, &m.Version,
	)
	if err == nil {
		// Parse JSON fields
		json.Unmarshal([]byte(commonIssuesJSON), &m.CommonIssues)
		json.Unmarshal([]byte(learnedBehaviorsJSON), &m.LearnedBehaviors)
		json.Unmarshal([]byte(improvementAreasJSON), &m.ImprovementAreas)
		json.Unmarshal([]byte(pendingSuggestionsJSON), &m.PendingSuggestions)
		// Ensure slices are never nil
		if m.CommonIssues == nil {
			m.CommonIssues = []store.CommonIssue{}
		}
		if m.LearnedBehaviors == nil {
			m.LearnedBehaviors = []store.LearnedBehavior{}
		}
		if m.ImprovementAreas == nil {
			m.ImprovementAreas = []store.ImprovementArea{}
		}
		if m.PendingSuggestions == nil {
			m.PendingSuggestions = []store.PendingSuggestion{}
		}
		return &m, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create new
	stmt := `
		INSERT INTO agent_learning_memory (
			tenant_id, common_issues, learned_behaviors, improvement_areas,
			pending_suggestions, analysis_count, last_updated, version
		) VALUES (?, '[]', '[]', '[]', '[]', 0, ?, 1)
	`
	now := time.Now()
	result, err := d.db.ExecContext(ctx, stmt, tenantID, now)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &store.AgentLearningMemory{
		ID:                 int32(id),
		TenantID:           tenantID,
		CommonIssues:       []store.CommonIssue{},
		LearnedBehaviors:   []store.LearnedBehavior{},
		ImprovementAreas:   []store.ImprovementArea{},
		PendingSuggestions: []store.PendingSuggestion{},
		AnalysisCount:      0,
		LastUpdated:        now,
		Version:            1,
	}, nil
}

func (d *DB) UpdateAgentLearningMemory(ctx context.Context, memory *store.AgentLearningMemory) (*store.AgentLearningMemory, error) {
	commonIssuesJSON, _ := json.Marshal(memory.CommonIssues)
	learnedBehaviorsJSON, _ := json.Marshal(memory.LearnedBehaviors)
	improvementAreasJSON, _ := json.Marshal(memory.ImprovementAreas)
	pendingSuggestionsJSON, _ := json.Marshal(memory.PendingSuggestions)

	stmt := `
		UPDATE agent_learning_memory SET
			common_issues = ?,
			learned_behaviors = ?,
			improvement_areas = ?,
			pending_suggestions = ?,
			analysis_count = ?,
			last_updated = ?,
			version = version + 1
		WHERE tenant_id = ?
	`
	now := time.Now()
	_, err := d.db.ExecContext(ctx, stmt,
		string(commonIssuesJSON), string(learnedBehaviorsJSON),
		string(improvementAreasJSON), string(pendingSuggestionsJSON),
		memory.AnalysisCount, now, memory.TenantID,
	)
	if err != nil {
		return nil, err
	}
	memory.LastUpdated = now
	memory.Version++
	return memory, nil
}

func (d *DB) DeleteAgentLearningMemory(ctx context.Context, tenantID int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_learning_memory WHERE tenant_id = ?", tenantID)
	return err
}

// ============================================================================
// COMPLIANCE AUDIT OPERATIONS
// ============================================================================

func (d *DB) CreateAgentComplianceAudit(ctx context.Context, audit *store.AgentComplianceAudit) error {
	stmt := `
		INSERT INTO agent_compliance_audits (
			id, tenant_id, conversation_id, conversation_type,
			score, checks, overall_passed, audited_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := d.db.ExecContext(ctx, stmt,
		audit.ID, audit.TenantID, audit.ConversationID, audit.ConversationType,
		audit.Score, audit.Checks, audit.OverallPassed, audit.AuditedAt,
	)
	return err
}

func (d *DB) GetAgentComplianceAudit(ctx context.Context, find *store.FindAgentComplianceAudit) (*store.AgentComplianceAudit, error) {
	where, args := []string{"1 = 1"}, []any{}

	if find.ID != nil {
		where = append(where, "id = ?")
		args = append(args, *find.ID)
	}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.ConversationID != nil {
		where = append(where, "conversation_id = ?")
		args = append(args, *find.ConversationID)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, conversation_id, conversation_type,
			   score, checks, overall_passed, audited_at
		FROM agent_compliance_audits
		WHERE %s
		ORDER BY audited_at DESC
		LIMIT 1
	`, strings.Join(where, " AND "))

	audit := &store.AgentComplianceAudit{}
	err := d.db.QueryRowContext(ctx, query, args...).Scan(
		&audit.ID, &audit.TenantID, &audit.ConversationID, &audit.ConversationType,
		&audit.Score, &audit.Checks, &audit.OverallPassed, &audit.AuditedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return audit, nil
}

func (d *DB) ListAgentComplianceAudits(ctx context.Context, find *store.FindAgentComplianceAudit) ([]*store.AgentComplianceAudit, error) {
	where, args := []string{"1 = 1"}, []any{}

	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.ConversationType != nil {
		where = append(where, "conversation_type = ?")
		args = append(args, *find.ConversationType)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, conversation_id, conversation_type,
			   score, checks, overall_passed, audited_at
		FROM agent_compliance_audits
		WHERE %s
		ORDER BY audited_at DESC
	`, strings.Join(where, " AND "))

	if find.Limit != nil && *find.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", *find.Limit)
	}
	if find.Offset != nil && *find.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", *find.Offset)
	}

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var audits []*store.AgentComplianceAudit
	for rows.Next() {
		audit := &store.AgentComplianceAudit{}
		if err := rows.Scan(
			&audit.ID, &audit.TenantID, &audit.ConversationID, &audit.ConversationType,
			&audit.Score, &audit.Checks, &audit.OverallPassed, &audit.AuditedAt,
		); err != nil {
			return nil, err
		}
		audits = append(audits, audit)
	}
	return audits, nil
}

// ============================================================================
// SCORING CONFIG OPERATIONS
// ============================================================================

func (d *DB) GetOrCreateAgentScoringConfig(ctx context.Context, tenantID int32) (*store.AgentScoringConfig, error) {
	// Try to get existing config
	query := `
		SELECT id, tenant_id, version, config, created_at, updated_at
		FROM agent_scoring_config
		WHERE tenant_id = ?
	`
	config := &store.AgentScoringConfig{}
	err := d.db.QueryRowContext(ctx, query, tenantID).Scan(
		&config.ID, &config.TenantID, &config.Version, &config.Config,
		&config.CreatedAt, &config.UpdatedAt,
	)
	if err == nil {
		return config, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create default config
	defaultConfig := `{
		"version": "1.0",
		"thresholds": {
			"high_urgency": 75,
			"medium_urgency": 40,
			"low_urgency": 0
		},
		"categories": [
			{"name": "urgency", "weight": 25},
			{"name": "safety_risk", "weight": 20},
			{"name": "service_match", "weight": 20},
			{"name": "escalation_signal", "weight": 15},
			{"name": "lead_quality", "weight": 10},
			{"name": "sentiment", "weight": 10}
		]
	}`

	now := time.Now()
	stmt := `
		INSERT INTO agent_scoring_config (tenant_id, version, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := d.db.ExecContext(ctx, stmt, tenantID, "1.0", defaultConfig, now, now)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &store.AgentScoringConfig{
		ID:        int32(id),
		TenantID:  tenantID,
		Version:   "1.0",
		Config:    defaultConfig,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (d *DB) UpdateAgentScoringConfig(ctx context.Context, config *store.AgentScoringConfig) (*store.AgentScoringConfig, error) {
	now := time.Now()
	stmt := `
		UPDATE agent_scoring_config SET
			version = ?,
			config = ?,
			updated_at = ?
		WHERE tenant_id = ?
	`
	_, err := d.db.ExecContext(ctx, stmt, config.Version, config.Config, now, config.TenantID)
	if err != nil {
		return nil, err
	}
	config.UpdatedAt = now
	return config, nil
}

// ============================================================================
// Q&A PAIR OPERATIONS (for embedding/retrieval testing)
// ============================================================================

func (d *DB) CreateAgentQAPair(ctx context.Context, pair *store.AgentQAPair) (*store.AgentQAPair, error) {
	now := time.Now()
	stmt := `
		INSERT INTO agent_qa_pairs (
			tenant_id, question, expected_answer, source_section, source_chunk_id,
			difficulty, category, is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := d.db.ExecContext(ctx, stmt,
		pair.TenantID, pair.Question, pair.ExpectedAnswer, pair.SourceSection, pair.SourceChunkID,
		pair.Difficulty, pair.Category, pair.IsActive, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	pair.ID = int32(id)
	pair.CreatedAt = now
	pair.UpdatedAt = now
	return pair, nil
}

func (d *DB) ListAgentQAPairs(ctx context.Context, find *store.FindAgentQAPair) ([]*store.AgentQAPair, error) {
	where, args := []string{"1 = 1"}, []any{}

	if find.ID != nil {
		where = append(where, "id = ?")
		args = append(args, *find.ID)
	}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}
	if find.Category != nil {
		where = append(where, "category = ?")
		args = append(args, *find.Category)
	}
	if find.IsActive != nil {
		where = append(where, "is_active = ?")
		args = append(args, *find.IsActive)
	}

	query := `
		SELECT id, tenant_id, question, expected_answer, source_section, source_chunk_id,
			difficulty, category, is_active, created_at, updated_at
		FROM agent_qa_pairs
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY id ASC
	`

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []*store.AgentQAPair
	for rows.Next() {
		pair := &store.AgentQAPair{}
		var sourceSection, sourceChunkID, difficulty, category sql.NullString
		if err := rows.Scan(
			&pair.ID, &pair.TenantID, &pair.Question, &pair.ExpectedAnswer,
			&sourceSection, &sourceChunkID, &difficulty, &category,
			&pair.IsActive, &pair.CreatedAt, &pair.UpdatedAt,
		); err != nil {
			return nil, err
		}
		pair.SourceSection = sourceSection.String
		pair.SourceChunkID = sourceChunkID.String
		pair.Difficulty = difficulty.String
		pair.Category = category.String
		pairs = append(pairs, pair)
	}
	return pairs, nil
}

func (d *DB) UpdateAgentQAPair(ctx context.Context, pair *store.AgentQAPair) (*store.AgentQAPair, error) {
	now := time.Now()
	stmt := `
		UPDATE agent_qa_pairs SET
			question = ?,
			expected_answer = ?,
			source_section = ?,
			source_chunk_id = ?,
			difficulty = ?,
			category = ?,
			is_active = ?,
			updated_at = ?
		WHERE id = ?
	`
	_, err := d.db.ExecContext(ctx, stmt,
		pair.Question, pair.ExpectedAnswer, pair.SourceSection, pair.SourceChunkID,
		pair.Difficulty, pair.Category, pair.IsActive, now, pair.ID,
	)
	if err != nil {
		return nil, err
	}
	pair.UpdatedAt = now
	return pair, nil
}

func (d *DB) DeleteAgentQAPair(ctx context.Context, id int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_qa_pairs WHERE id = ?", id)
	return err
}

func (d *DB) DeleteAgentQAPairsByTenant(ctx context.Context, tenantID int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_qa_pairs WHERE tenant_id = ?", tenantID)
	return err
}
