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
	if tenant.WidgetKey == "" {
		tenant.WidgetKey = uuid.NewString()
	}
	err := d.db.QueryRowContext(ctx, `
		INSERT INTO agent_tenants (slug, company_name, guid, widget_key, vertical, is_active, processing_options, allowed_domains, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),NULLIF($8,''),$9,$9)
		RETURNING id
	`, tenant.Slug, tenant.CompanyName, tenant.GUID, tenant.WidgetKey, tenant.Vertical, tenant.IsActive, tenant.ProcessingOptions, tenant.AllowedDomains, now).Scan(&tenant.ID)
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
	if find.GUID != nil {
		add("guid = $%d", *find.GUID)
	}
	if find.IsActive != nil {
		add("is_active = $%d", *find.IsActive)
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, slug, company_name, guid, widget_key, vertical, is_active, processing_options, allowed_domains, transcript_signing_key, transcript_signing_key_nonce, created_at, updated_at
		FROM agent_tenants WHERE `+strings.Join(where, " AND ")+` ORDER BY created_at DESC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*store.AgentTenant
	for rows.Next() {
		var tenant store.AgentTenant
		var guid, widgetKey, vertical, processing, domains sql.NullString
		if err := rows.Scan(&tenant.ID, &tenant.Slug, &tenant.CompanyName, &guid, &widgetKey, &vertical, &tenant.IsActive, &processing, &domains, &tenant.TranscriptSigningKey, &tenant.TranscriptSigningKeyNonce, &tenant.CreatedAt, &tenant.UpdatedAt); err != nil {
			return nil, err
		}
		tenant.GUID, tenant.WidgetKey, tenant.Vertical = guid.String, widgetKey.String, vertical.String
		tenant.ProcessingOptions, tenant.AllowedDomains = processing.String, domains.String
		result = append(result, &tenant)
	}
	return result, rows.Err()
}

func (d *DB) UpdateAgentTenant(ctx context.Context, tenant *store.AgentTenant) (*store.AgentTenant, error) {
	now := time.Now()
	_, err := d.db.ExecContext(ctx, `
		UPDATE agent_tenants SET company_name=$1, vertical=$2, is_active=$3,
			processing_options=NULLIF($4,''), allowed_domains=NULLIF($5,''), widget_key=NULLIF($6,''),
			transcript_signing_key=$7, transcript_signing_key_nonce=$8, updated_at=$9
		WHERE id=$10
	`, tenant.CompanyName, tenant.Vertical, tenant.IsActive, tenant.ProcessingOptions, tenant.AllowedDomains, tenant.WidgetKey, tenant.TranscriptSigningKey, tenant.TranscriptSigningKeyNonce, now, tenant.ID)
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
			escalation_confidence_threshold,rate_limit_rpm,require_contact_on_fallback,
			max_message_length,updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING id
	`, audience.TenantID, audience.AudienceType, audience.Role, audience.Tone, audience.BrandVoice,
		string(guidelines), audience.EmergencyPhone, string(phones), audience.Email, audience.Address,
		audience.EmergencyUrgencyThreshold, audience.EscalationConfidenceThreshold, audience.RateLimitRPM,
		audience.RequireContactOnFallback, audience.MaxMessageLength, now,
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
			escalation_confidence_threshold,rate_limit_rpm,require_contact_on_fallback,
			max_message_length,updated_at
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
			&audience.RateLimitRPM, &audience.RequireContactOnFallback, &audience.MaxMessageLength,
			&audience.UpdatedAt); err != nil {
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
			rate_limit_rpm=$11,require_contact_on_fallback=$12,
			max_message_length=$13,updated_at=$14
		WHERE tenant_id=$15 AND audience_type=$16
	`, audience.Role, audience.Tone, audience.BrandVoice, string(guidelines), audience.EmergencyPhone,
		string(phones), audience.Email, audience.Address, audience.EmergencyUrgencyThreshold,
		audience.EscalationConfidenceThreshold, audience.RateLimitRPM, audience.RequireContactOnFallback,
		audience.MaxMessageLength, now, audience.TenantID, audience.AudienceType)
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
	stmt := `
		INSERT INTO agent_services (tenant_id, audience_type, code, name, description, is_emergency, response_time, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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
	where := []string{"TRUE"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if find.TenantID != nil {
		add("tenant_id = $%d", *find.TenantID)
	}
	if find.AudienceType != nil {
		add("audience_type = $%d", *find.AudienceType)
	}
	if find.Code != nil {
		add("code = $%d", *find.Code)
	}
	if find.IsActive != nil {
		add("is_active = $%d", *find.IsActive)
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, tenant_id, audience_type, code, name, description, is_emergency, response_time, is_active
		FROM agent_services WHERE `+strings.Join(where, " AND ")+`
		ORDER BY name
	`, args...)
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
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_services WHERE tenant_id = $1 AND audience_type = $2", tenantID, audienceType)
	return err
}

// Agent Exclusion methods

func (d *DB) CreateAgentExclusion(ctx context.Context, exclusion *store.AgentExclusion) (*store.AgentExclusion, error) {
	stmt := `
		INSERT INTO agent_exclusions (tenant_id, audience_type, code, name, description, exception_rule, referral, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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
	where := []string{"TRUE"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if find.TenantID != nil {
		add("tenant_id = $%d", *find.TenantID)
	}
	if find.AudienceType != nil {
		add("audience_type = $%d", *find.AudienceType)
	}
	if find.IsActive != nil {
		add("is_active = $%d", *find.IsActive)
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, tenant_id, audience_type, code, name, description, exception_rule, referral, is_active
		FROM agent_exclusions WHERE `+strings.Join(where, " AND "),
		args...)
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
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_exclusions WHERE tenant_id = $1 AND audience_type = $2", tenantID, audienceType)
	return err
}

// Agent Coverage methods

func (d *DB) CreateAgentCoverage(ctx context.Context, coverage *store.AgentCoverage) (*store.AgentCoverage, error) {
	stmt := `
		INSERT INTO agent_coverage (tenant_id, area_type, area_name, state_code, is_included)
		VALUES ($1, $2, $3, $4, $5)
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
	where := []string{"TRUE"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if find.TenantID != nil {
		add("tenant_id = $%d", *find.TenantID)
	}
	if find.IsIncluded != nil {
		add("is_included = $%d", *find.IsIncluded)
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, tenant_id, area_type, area_name, state_code, is_included
		FROM agent_coverage WHERE `+strings.Join(where, " AND "),
		args...)
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
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_coverage WHERE tenant_id = $1", tenantID)
	return err
}

// Agent FAQ methods

func (d *DB) CreateAgentFAQ(ctx context.Context, faq *store.AgentFAQ) (*store.AgentFAQ, error) {
	stmt := `
		INSERT INTO agent_faqs (tenant_id, audience_type, code, question, answer, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
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
	where := []string{"TRUE"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if find.TenantID != nil {
		add("tenant_id = $%d", *find.TenantID)
	}
	if find.AudienceType != nil {
		add("audience_type = $%d", *find.AudienceType)
	}
	if find.IsActive != nil {
		add("is_active = $%d", *find.IsActive)
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, tenant_id, audience_type, code, question, answer, is_active
		FROM agent_faqs WHERE `+strings.Join(where, " AND "),
		args...)
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
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_faqs WHERE tenant_id = $1 AND audience_type = $2", tenantID, audienceType)
	return err
}

// Agent Safety Protocol methods

func (d *DB) CreateAgentSafetyProtocol(ctx context.Context, protocol *store.AgentSafetyProtocol) (*store.AgentSafetyProtocol, error) {
	triggerIntentsJSON, _ := json.Marshal(protocol.TriggerIntents)
	instructionsJSON, _ := json.Marshal(protocol.Instructions)

	stmt := `
		INSERT INTO agent_safety_protocols (tenant_id, audience_type, code, name, trigger_intents, instructions, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
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
	where := []string{"TRUE"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if find.TenantID != nil {
		add("tenant_id = $%d", *find.TenantID)
	}
	if find.AudienceType != nil {
		add("audience_type = $%d", *find.AudienceType)
	}
	if find.IsActive != nil {
		add("is_active = $%d", *find.IsActive)
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, tenant_id, audience_type, code, name, trigger_intents, instructions, is_active
		FROM agent_safety_protocols WHERE `+strings.Join(where, " AND "),
		args...)
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
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_safety_protocols WHERE tenant_id = $1 AND audience_type = $2", tenantID, audienceType)
	return err
}

// Agent KB Section methods

func (d *DB) CreateAgentKBSection(ctx context.Context, section *store.AgentKBSection) (*store.AgentKBSection, error) {
	stmt := `
		INSERT INTO agent_kb_sections (tenant_id, audience_type, code, title, content, section_type, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
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
	where := []string{"TRUE"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if find.TenantID != nil {
		add("tenant_id = $%d", *find.TenantID)
	}
	if find.AudienceType != nil {
		add("audience_type = $%d", *find.AudienceType)
	}
	if find.SectionType != nil {
		add("section_type = $%d", *find.SectionType)
	}
	if find.IsActive != nil {
		add("is_active = $%d", *find.IsActive)
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, tenant_id, audience_type, code, title, content, section_type, is_active
		FROM agent_kb_sections WHERE `+strings.Join(where, " AND "),
		args...)
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
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_kb_sections WHERE tenant_id = $1 AND audience_type = $2", tenantID, audienceType)
	return err
}

// Agent Intent methods

func (d *DB) CreateAgentIntent(ctx context.Context, intent *store.AgentIntent) (*store.AgentIntent, error) {
	examplesJSON, _ := json.Marshal(intent.Examples)
	counterExamplesJSON, _ := json.Marshal(intent.CounterExamples)

	stmt := `
		INSERT INTO agent_intents (
			tenant_id, audience_type, code, name, category, description,
			examples, counter_examples, urgency, action, confidence_threshold, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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
	where := []string{"TRUE"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if find.TenantID != nil {
		add("(tenant_id = $%d OR tenant_id IS NULL)", *find.TenantID)
	}
	if find.AudienceType != nil {
		add("(audience_type = $%d OR audience_type IS NULL)", *find.AudienceType)
	}
	if find.Category != nil {
		add("category = $%d", *find.Category)
	}
	if find.IsActive != nil {
		add("is_active = $%d", *find.IsActive)
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, tenant_id, audience_type, code, name, category, description,
			examples, counter_examples, urgency, action, confidence_threshold, is_active
		FROM agent_intents WHERE `+strings.Join(where, " AND ")+`
		ORDER BY urgency DESC, name
	`, args...)
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
		_, err := d.db.ExecContext(ctx, "DELETE FROM agent_intents WHERE tenant_id = $1 AND audience_type = $2", tenantID, *audienceType)
		return err
	}
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_intents WHERE tenant_id = $1", tenantID)
	return err
}

// Agent Rule methods

func (d *DB) CreateAgentRule(ctx context.Context, rule *store.AgentRule) (*store.AgentRule, error) {
	stmt := `
		INSERT INTO agent_rules (tenant_id, audience_type, code, name, description, priority, applies_to, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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
	where := []string{"TRUE"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if find.TenantID != nil {
		add("tenant_id = $%d", *find.TenantID)
	}
	if find.AudienceType != nil {
		add("audience_type = $%d", *find.AudienceType)
	}
	if find.IsActive != nil {
		add("is_active = $%d", *find.IsActive)
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, tenant_id, audience_type, code, name, description, priority, applies_to, is_active
		FROM agent_rules WHERE `+strings.Join(where, " AND ")+`
		ORDER BY priority
	`, args...)
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
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_rules WHERE tenant_id = $1 AND audience_type = $2", tenantID, audienceType)
	return err
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
	messagesJSON, _ := json.Marshal(session.Messages)

	stmt := `
		INSERT INTO agent_sessions (
			id, tenant_id, user_id, audience_type, phase, current_intent,
			urgency_level, coverage_status, customer_name, customer_phone,
			customer_location, detected_service, message_count, messages,
			created_at, updated_at, is_completed
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
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
	if find.UserID != nil {
		add("user_id = $%d", *find.UserID)
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, tenant_id, user_id, audience_type, phase, current_intent,
			urgency_level, coverage_status, customer_name, customer_phone,
			customer_location, detected_service, message_count, messages,
			created_at, updated_at, completed_at, is_completed, completion_reason
		FROM agent_sessions WHERE `+strings.Join(where, " AND ")+`
		ORDER BY updated_at DESC
	`, args...)
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
	set, args := []string{}, []any{}
	if update.Phase != nil {
		set = append(set, fmt.Sprintf("phase = $%d", len(args)+1))
		args = append(args, *update.Phase)
	}
	if update.CurrentIntent != nil {
		set = append(set, fmt.Sprintf("current_intent = $%d", len(args)+1))
		args = append(args, *update.CurrentIntent)
	}
	if update.UrgencyLevel != nil {
		set = append(set, fmt.Sprintf("urgency_level = $%d", len(args)+1))
		args = append(args, *update.UrgencyLevel)
	}
	if update.CoverageStatus != nil {
		set = append(set, fmt.Sprintf("coverage_status = $%d", len(args)+1))
		args = append(args, *update.CoverageStatus)
	}
	if update.CustomerName != nil {
		set = append(set, fmt.Sprintf("customer_name = $%d", len(args)+1))
		args = append(args, *update.CustomerName)
	}
	if update.CustomerPhone != nil {
		set = append(set, fmt.Sprintf("customer_phone = $%d", len(args)+1))
		args = append(args, *update.CustomerPhone)
	}
	if update.CustomerLocation != nil {
		set = append(set, fmt.Sprintf("customer_location = $%d", len(args)+1))
		args = append(args, *update.CustomerLocation)
	}
	if update.DetectedService != nil {
		set = append(set, fmt.Sprintf("detected_service = $%d", len(args)+1))
		args = append(args, *update.DetectedService)
	}
	if update.MessageCount != nil {
		set = append(set, fmt.Sprintf("message_count = $%d", len(args)+1))
		args = append(args, *update.MessageCount)
	}
	if update.Messages != nil {
		messagesJSON, _ := json.Marshal(update.Messages)
		set = append(set, fmt.Sprintf("messages = $%d", len(args)+1))
		args = append(args, string(messagesJSON))
	}
	if update.CompletedAt != nil {
		set = append(set, fmt.Sprintf("completed_at = $%d", len(args)+1))
		args = append(args, *update.CompletedAt)
	}
	if update.IsCompleted != nil {
		set = append(set, fmt.Sprintf("is_completed = $%d", len(args)+1))
		args = append(args, *update.IsCompleted)
	}
	if update.CompletionReason != nil {
		set = append(set, fmt.Sprintf("completion_reason = $%d", len(args)+1))
		args = append(args, *update.CompletionReason)
	}

	now := time.Now()
	set = append(set, fmt.Sprintf("updated_at = $%d", len(args)+1))
	args = append(args, now)
	args = append(args, update.ID)

	stmt := fmt.Sprintf("UPDATE agent_sessions SET %s WHERE id = $%d", strings.Join(set, ", "), len(args))
	_, err := d.db.ExecContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}

	return d.GetAgentSession(ctx, &store.FindAgentSession{ID: &update.ID})
}

func (d *DB) DeleteAgentSession(ctx context.Context, id string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_sessions WHERE id = $1", id)
	return err
}

// Agent Source File methods

func (d *DB) UpsertAgentSourceFile(ctx context.Context, file *store.AgentSourceFile) (*store.AgentSourceFile, error) {
	var nextVersion int32 = 1
	err := d.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM agent_source_files
		WHERE tenant_id = $1 AND audience_type = $2 AND file_type = $3
	`, file.TenantID, file.AudienceType, file.FileType).Scan(&nextVersion)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get next version: %w", err)
	}

	stmt := `
		INSERT INTO agent_source_files (tenant_id, audience_type, file_type, content, content_hash, version, imported_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
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
	if find.AudienceType != nil {
		add("audience_type = $%d", *find.AudienceType)
	}
	if find.FileType != nil {
		add("file_type = $%d", *find.FileType)
	}
	if find.Version != nil {
		add("version = $%d", *find.Version)
	}

	whereClause := strings.Join(where, " AND ")
	if find.LatestOnly {
		query := fmt.Sprintf(`
			SELECT id, tenant_id, audience_type, file_type, content, content_hash, COALESCE(version, 1), imported_at
			FROM agent_source_files
			WHERE %s
			AND (tenant_id, audience_type, file_type, version) IN (
				SELECT tenant_id, audience_type, file_type, MAX(version)
				FROM agent_source_files
				GROUP BY tenant_id, audience_type, file_type
			)
			ORDER BY imported_at DESC
		`, whereClause)
		rows, err := d.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanAgentSourceFiles(rows)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, audience_type, file_type, content, content_hash, COALESCE(version, 1), imported_at
		FROM agent_source_files
		WHERE %s
		ORDER BY version DESC, imported_at DESC
	`, whereClause)
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAgentSourceFiles(rows)
}

func scanAgentSourceFiles(rows *sql.Rows) ([]*store.AgentSourceFile, error) {
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
		_, err := d.db.ExecContext(ctx, "DELETE FROM agent_source_files WHERE tenant_id = $1 AND audience_type = $2", tenantID, *audienceType)
		return err
	}
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_source_files WHERE tenant_id = $1", tenantID)
	return err
}

func (d *DB) CountTenantSourceFiles(ctx context.Context, tenantID int32) (count int, totalContentLen int, maxTrimmedLen int, err error) {
	// TRIM only strips space characters (0x20), not tabs/newlines.
	// Tab-only whitespace files are an accepted edge case.
	err = d.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(LENGTH(content)), 0), COALESCE(MAX(LENGTH(TRIM(content))), 0)
		 FROM agent_source_files
		 WHERE tenant_id = $1`, tenantID,
	).Scan(&count, &totalContentLen, &maxTrimmedLen)
	return
}

// ============================================================================
// AGENT RAG ACTIVE-VERSION OPERATIONS (versioned RAG index pointer)
// ============================================================================

func (d *DB) UpsertAgentRAGActiveVersion(ctx context.Context, v *store.AgentRAGActiveVersion) (*store.AgentRAGActiveVersion, error) {
	now := time.Now()
	v.UpdatedAt = now
	stmt := `
		INSERT INTO agent_rag_active_versions (tenant_id, audience_type, file_type, version, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT(tenant_id, audience_type, file_type) DO UPDATE SET
			version = excluded.version,
			updated_at = excluded.updated_at
		RETURNING id
	`
	if err := d.db.QueryRowContext(ctx, stmt, v.TenantID, v.AudienceType, v.FileType, v.Version, now).Scan(&v.ID); err != nil {
		return nil, fmt.Errorf("failed to upsert rag active version: %w", err)
	}
	return v, nil
}

func (d *DB) GetAgentRAGActiveVersion(ctx context.Context, find *store.FindAgentRAGActiveVersion) (*store.AgentRAGActiveVersion, error) {
	where := []string{"TRUE"}
	args := []any{}
	if find.TenantID != nil {
		args = append(args, *find.TenantID)
		where = append(where, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if find.AudienceType != nil {
		args = append(args, *find.AudienceType)
		where = append(where, fmt.Sprintf("audience_type = $%d", len(args)))
	}
	if find.FileType != nil {
		args = append(args, *find.FileType)
		where = append(where, fmt.Sprintf("file_type = $%d", len(args)))
	}
	query := `
		SELECT id, tenant_id, audience_type, file_type, version, updated_at
		FROM agent_rag_active_versions
		WHERE ` + strings.Join(where, " AND ") + `
		LIMIT 1
	`
	var v store.AgentRAGActiveVersion
	if err := d.db.QueryRowContext(ctx, query, args...).Scan(
		&v.ID, &v.TenantID, &v.AudienceType, &v.FileType, &v.Version, &v.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get rag active version: %w", err)
	}
	return &v, nil
}

func (d *DB) ListAgentRAGActiveVersions(ctx context.Context, tenantID int32) ([]*store.AgentRAGActiveVersion, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT id, tenant_id, audience_type, file_type, version, updated_at
		FROM agent_rag_active_versions
		WHERE tenant_id = $1
		ORDER BY audience_type, file_type
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []*store.AgentRAGActiveVersion{}
	for rows.Next() {
		var v store.AgentRAGActiveVersion
		if err := rows.Scan(&v.ID, &v.TenantID, &v.AudienceType, &v.FileType, &v.Version, &v.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &v)
	}
	return list, rows.Err()
}

func (d *DB) DeleteAgentRAGActiveVersion(ctx context.Context, tenantID int32, audienceType, fileType string) error {
	_, err := d.db.ExecContext(ctx,
		"DELETE FROM agent_rag_active_versions WHERE tenant_id = $1 AND audience_type = $2 AND file_type = $3",
		tenantID, audienceType, fileType,
	)
	return err
}

// Agent Rate Limit methods

func (d *DB) GetOrCreateAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) (*store.AgentRateLimit, error) {
	var rl store.AgentRateLimit
	err := d.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, audience_type, client_ip, request_count, window_start
		FROM agent_rate_limits
		WHERE tenant_id = $1 AND audience_type = $2 AND client_ip = $3
	`, tenantID, audienceType, clientIP).Scan(
		&rl.ID, &rl.TenantID, &rl.AudienceType, &rl.ClientIP, &rl.RequestCount, &rl.WindowStart,
	)
	if err == nil {
		return &rl, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	now := time.Now()
	err = d.db.QueryRowContext(ctx, `
		INSERT INTO agent_rate_limits (tenant_id, audience_type, client_ip, request_count, window_start)
		VALUES ($1, $2, $3, 0, $4)
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
		WHERE tenant_id = $1 AND audience_type = $2 AND client_ip = $3
	`, tenantID, audienceType, clientIP)
	return err
}

func (d *DB) ResetAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string) error {
	now := time.Now()
	_, err := d.db.ExecContext(ctx, `
		UPDATE agent_rate_limits
		SET request_count = 0, window_start = $1
		WHERE tenant_id = $2 AND audience_type = $3 AND client_ip = $4
	`, now, tenantID, audienceType, clientIP)
	return err
}

// CheckAndIncrementAgentRateLimit atomically checks the rate limit and increments
// the counter in a single SQL statement, eliminating the TOCTOU race condition.
// Returns true if the request is allowed, false if rate limited.
func (d *DB) CheckAndIncrementAgentRateLimit(ctx context.Context, tenantID int32, audienceType, clientIP string, rpm int) (bool, error) {
	if rpm <= 0 {
		rpm = 60
	}
	now := time.Now()
	windowSeconds := 60.0

	var allowed bool
	err := d.db.QueryRowContext(ctx, `
		WITH old AS (
			SELECT request_count FROM agent_rate_limits
			WHERE tenant_id = $1 AND audience_type = $2 AND client_ip = $3
		)
		INSERT INTO agent_rate_limits (tenant_id, audience_type, client_ip, request_count, window_start)
		VALUES ($1, $2, $3, 1, $4)
		ON CONFLICT(tenant_id, audience_type, client_ip) DO UPDATE SET
			request_count = CASE
				WHEN EXTRACT(EPOCH FROM ($5 - agent_rate_limits.window_start)) > $6 THEN 1
				WHEN agent_rate_limits.request_count < $7 THEN agent_rate_limits.request_count + 1
				ELSE agent_rate_limits.request_count
			END,
			window_start = CASE
				WHEN EXTRACT(EPOCH FROM ($5 - agent_rate_limits.window_start)) > $6 THEN $5
				ELSE agent_rate_limits.window_start
			END
		RETURNING CASE
			WHEN EXTRACT(EPOCH FROM ($5 - window_start)) > $6 THEN 1
			WHEN NOT EXISTS (SELECT 1 FROM old) THEN 1
			WHEN (SELECT request_count FROM old) < $7 THEN 1
			ELSE 0
		END
	`, tenantID, audienceType, clientIP, now,
		now, windowSeconds, rpm,
	).Scan(&allowed)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

const tenantGlobalIPSentinel = "__tenant_global__"

// CheckAndIncrementTenantGlobalRateLimit atomically checks the per-tenant global
// rate limit (ignoring IP) to cap total requests per tenant per minute.
func (d *DB) CheckAndIncrementTenantGlobalRateLimit(ctx context.Context, tenantID int32, audienceType string, rpm int) (bool, error) {
	if rpm <= 0 {
		rpm = 300 // default global tenant cap
	}
	now := time.Now()
	windowSeconds := 60.0
	clientIP := tenantGlobalIPSentinel

	var allowed bool
	err := d.db.QueryRowContext(ctx, `
		WITH old AS (
			SELECT request_count FROM agent_rate_limits
			WHERE tenant_id = $1 AND audience_type = $2 AND client_ip = $3
		)
		INSERT INTO agent_rate_limits (tenant_id, audience_type, client_ip, request_count, window_start)
		VALUES ($1, $2, $3, 1, $4)
		ON CONFLICT(tenant_id, audience_type, client_ip) DO UPDATE SET
			request_count = CASE
				WHEN EXTRACT(EPOCH FROM ($5 - agent_rate_limits.window_start)) > $6 THEN 1
				WHEN agent_rate_limits.request_count < $7 THEN agent_rate_limits.request_count + 1
				ELSE agent_rate_limits.request_count
			END,
			window_start = CASE
				WHEN EXTRACT(EPOCH FROM ($5 - agent_rate_limits.window_start)) > $6 THEN $5
				ELSE agent_rate_limits.window_start
			END
		RETURNING CASE
			WHEN EXTRACT(EPOCH FROM ($5 - window_start)) > $6 THEN 1
			WHEN NOT EXISTS (SELECT 1 FROM old) THEN 1
			WHEN (SELECT request_count FROM old) < $7 THEN 1
			ELSE 0
		END
	`, tenantID, audienceType, clientIP, now,
		now, windowSeconds, rpm,
	).Scan(&allowed)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

// Agent Simulation Transcript methods

func (d *DB) CreateAgentSimulationTranscript(ctx context.Context, transcript *store.AgentSimulationTranscript) (*store.AgentSimulationTranscript, error) {
	messagesJSON, _ := json.Marshal(transcript.Messages)

	stmt := `
		INSERT INTO agent_simulation_transcripts (
			id, tenant_id, user_id, initial_prompt, persona_hint,
			total_turns, end_reason, messages, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
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
	if find.UserID != nil {
		add("user_id = $%d", *find.UserID)
	}

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM agent_simulation_transcripts WHERE %s
	`, strings.Join(where, " AND "))

	var total int
	if err := d.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, user_id, initial_prompt, persona_hint,
			total_turns, end_reason, messages, created_at
		FROM agent_simulation_transcripts
		WHERE %s
		ORDER BY created_at DESC
	`, strings.Join(where, " AND "))

	limitPos := 0
	if find.Limit > 0 {
		limitPos = len(args) + 1
		query += fmt.Sprintf(" LIMIT $%d", limitPos)
		if find.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", limitPos+1)
		}
	}

	argsCopy := append([]any{}, args...)
	if find.Limit > 0 {
		argsCopy = append(argsCopy, find.Limit)
		if find.Offset > 0 {
			argsCopy = append(argsCopy, find.Offset)
		}
	}

	rows, err := d.db.QueryContext(ctx, query, argsCopy...)
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
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_simulation_transcripts WHERE id = $1", id)
	return err
}

// Agent Tenant Script methods

func (d *DB) UpsertAgentTenantScript(ctx context.Context, script *store.AgentTenantScript) (*store.AgentTenantScript, error) {
	var existingID int32
	err := d.db.QueryRowContext(ctx, "SELECT id FROM agent_tenant_scripts WHERE tenant_id = $1", script.TenantID).Scan(&existingID)

	now := time.Now()
	if err == sql.ErrNoRows {
		stmt := `
			INSERT INTO agent_tenant_scripts (tenant_id, content, content_hash, summary, imported_at, version)
			VALUES ($1, $2, $3, $4, $5, 1)
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
		stmt := `
			UPDATE agent_tenant_scripts
			SET content = $1, content_hash = $2, summary = $3, imported_at = $4, version = version + 1
			WHERE tenant_id = $5
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

	var s store.AgentTenantScript
	var summary sql.NullString
	err := d.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, content, content_hash, summary, imported_at, version
		FROM agent_tenant_scripts WHERE `+strings.Join(where, " AND "),
		args...).Scan(
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
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_tenant_scripts WHERE tenant_id = $1", tenantID)
	return err
}

// Agent Analysis Result methods

func (d *DB) CreateAgentAnalysisResult(ctx context.Context, result *store.AgentAnalysisResult) (*store.AgentAnalysisResult, error) {
	breakdownJSON, _ := json.Marshal(result.Breakdown)
	issuesJSON, _ := json.Marshal(result.Issues)
	suggestionsJSON, _ := json.Marshal(result.Suggestions)

	stmt := `
		INSERT INTO agent_analysis_results (
			id, tenant_id, conversation_id, conversation_type, user_id,
			score, grade, breakdown, issues, suggestions, benchmark_version, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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
	if find.ConversationID != nil {
		add("conversation_id = $%d", *find.ConversationID)
	}
	if find.UserID != nil {
		add("user_id = $%d", *find.UserID)
	}

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM agent_analysis_results WHERE %s
	`, strings.Join(where, " AND "))

	var total int
	if err := d.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, conversation_id, conversation_type, user_id,
			score, grade, breakdown, issues, suggestions, benchmark_version, created_at
		FROM agent_analysis_results
		WHERE %s
		ORDER BY created_at DESC
	`, strings.Join(where, " AND "))

	if find.Limit > 0 {
		limitPos := len(args) + 1
		query += fmt.Sprintf(" LIMIT $%d", limitPos)
		if find.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", limitPos+1)
		}
	}

	argsCopy := append([]any{}, args...)
	if find.Limit > 0 {
		argsCopy = append(argsCopy, find.Limit)
		if find.Offset > 0 {
			argsCopy = append(argsCopy, find.Offset)
		}
	}

	rows, err := d.db.QueryContext(ctx, query, argsCopy...)
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

// Agent Learning Memory methods

func (d *DB) GetOrCreateAgentLearningMemory(ctx context.Context, tenantID int32) (*store.AgentLearningMemory, error) {
	query := `
		SELECT id, tenant_id, common_issues, learned_behaviors, improvement_areas,
			pending_suggestions, analysis_count, last_updated, version
		FROM agent_learning_memory
		WHERE tenant_id = $1
	`
	var m store.AgentLearningMemory
	var commonIssuesJSON, learnedBehaviorsJSON, improvementAreasJSON, pendingSuggestionsJSON string
	err := d.db.QueryRowContext(ctx, query, tenantID).Scan(
		&m.ID, &m.TenantID, &commonIssuesJSON, &learnedBehaviorsJSON,
		&improvementAreasJSON, &pendingSuggestionsJSON, &m.AnalysisCount,
		&m.LastUpdated, &m.Version,
	)
	if err == nil {
		json.Unmarshal([]byte(commonIssuesJSON), &m.CommonIssues)
		json.Unmarshal([]byte(learnedBehaviorsJSON), &m.LearnedBehaviors)
		json.Unmarshal([]byte(improvementAreasJSON), &m.ImprovementAreas)
		json.Unmarshal([]byte(pendingSuggestionsJSON), &m.PendingSuggestions)
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

	stmt := `
		INSERT INTO agent_learning_memory (
			tenant_id, common_issues, learned_behaviors, improvement_areas,
			pending_suggestions, analysis_count, last_updated, version
		) VALUES ($1, '[]', '[]', '[]', '[]', 0, $2, 1)
		RETURNING id
	`
	now := time.Now()
	var id int64
	err = d.db.QueryRowContext(ctx, stmt, tenantID, now).Scan(&id)
	if err != nil {
		return nil, err
	}
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
			common_issues = $1,
			learned_behaviors = $2,
			improvement_areas = $3,
			pending_suggestions = $4,
			analysis_count = $5,
			last_updated = $6,
			version = version + 1
		WHERE tenant_id = $7
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
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_learning_memory WHERE tenant_id = $1", tenantID)
	return err
}

// Agent Compliance Audit methods

func (d *DB) CreateAgentComplianceAudit(ctx context.Context, audit *store.AgentComplianceAudit) error {
	stmt := `
		INSERT INTO agent_compliance_audits (
			id, tenant_id, conversation_id, conversation_type,
			score, checks, overall_passed, audited_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := d.db.ExecContext(ctx, stmt,
		audit.ID, audit.TenantID, audit.ConversationID, audit.ConversationType,
		audit.Score, audit.Checks, audit.OverallPassed, audit.AuditedAt,
	)
	return err
}

func (d *DB) GetAgentComplianceAudit(ctx context.Context, find *store.FindAgentComplianceAudit) (*store.AgentComplianceAudit, error) {
	where := []string{"TRUE"}
	args := []any{}
	if find.ID != nil {
		args = append(args, *find.ID)
		where = append(where, fmt.Sprintf("id = $%d", len(args)))
	}
	if find.TenantID != nil {
		args = append(args, *find.TenantID)
		where = append(where, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if find.ConversationID != nil {
		args = append(args, *find.ConversationID)
		where = append(where, fmt.Sprintf("conversation_id = $%d", len(args)))
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
	where := []string{"TRUE"}
	args := []any{}
	if find.TenantID != nil {
		args = append(args, *find.TenantID)
		where = append(where, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if find.ConversationType != nil {
		args = append(args, *find.ConversationType)
		where = append(where, fmt.Sprintf("conversation_type = $%d", len(args)))
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, conversation_id, conversation_type,
			   score, checks, overall_passed, audited_at
		FROM agent_compliance_audits
		WHERE %s
		ORDER BY audited_at DESC
	`, strings.Join(where, " AND "))

	if find.Limit != nil && *find.Limit > 0 {
		limitPos := len(args) + 1
		query += fmt.Sprintf(" LIMIT $%d", limitPos)
		args = append(args, *find.Limit)
		if find.Offset != nil && *find.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", limitPos+1)
			args = append(args, *find.Offset)
		}
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

// Agent Scoring Config methods

func (d *DB) GetOrCreateAgentScoringConfig(ctx context.Context, tenantID int32) (*store.AgentScoringConfig, error) {
	query := `
		SELECT id, tenant_id, version, config, created_at, updated_at
		FROM agent_scoring_config
		WHERE tenant_id = $1
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
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	var id int64
	err = d.db.QueryRowContext(ctx, stmt, tenantID, "1.0", defaultConfig, now, now).Scan(&id)
	if err != nil {
		return nil, err
	}
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
			version = $1,
			config = $2,
			updated_at = $3
		WHERE tenant_id = $4
	`
	_, err := d.db.ExecContext(ctx, stmt, config.Version, config.Config, now, config.TenantID)
	if err != nil {
		return nil, err
	}
	config.UpdatedAt = now
	return config, nil
}

// Agent Q&A Pair methods

func (d *DB) CreateAgentQAPair(ctx context.Context, pair *store.AgentQAPair) (*store.AgentQAPair, error) {
	now := time.Now()
	stmt := `
		INSERT INTO agent_qa_pairs (
			tenant_id, question, expected_answer, source_section, source_chunk_id,
			difficulty, category, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	var id int64
	err := d.db.QueryRowContext(ctx, stmt,
		pair.TenantID, pair.Question, pair.ExpectedAnswer, pair.SourceSection, pair.SourceChunkID,
		pair.Difficulty, pair.Category, pair.IsActive, now, now,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	pair.ID = int32(id)
	pair.CreatedAt = now
	pair.UpdatedAt = now
	return pair, nil
}

func (d *DB) ListAgentQAPairs(ctx context.Context, find *store.FindAgentQAPair) ([]*store.AgentQAPair, error) {
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
	if find.Category != nil {
		add("category = $%d", *find.Category)
	}
	if find.IsActive != nil {
		add("is_active = $%d", *find.IsActive)
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

func (d *DB) UpdateAgentQAPair(ctx context.Context, pair *store.AgentQAPair, tenantID int32) (*store.AgentQAPair, error) {
	now := time.Now()
	stmt := `
		UPDATE agent_qa_pairs SET
			question = $1,
			expected_answer = $2,
			source_section = $3,
			source_chunk_id = $4,
			difficulty = $5,
			category = $6,
			is_active = $7,
			updated_at = $8
		WHERE id = $9 AND tenant_id = $10
	`
	result, err := d.db.ExecContext(ctx, stmt,
		pair.Question, pair.ExpectedAnswer, pair.SourceSection, pair.SourceChunkID,
		pair.Difficulty, pair.Category, pair.IsActive, now, pair.ID, tenantID,
	)
	if err != nil {
		return nil, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("QA pair not found or not owned by tenant")
	}
	pair.UpdatedAt = now
	return pair, nil
}

func (d *DB) DeleteAgentQAPair(ctx context.Context, id int32, tenantID int32) error {
	result, err := d.db.ExecContext(ctx, "DELETE FROM agent_qa_pairs WHERE id = $1 AND tenant_id = $2", id, tenantID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("QA pair not found or not owned by tenant")
	}
	return nil
}

func (d *DB) DeleteAgentQAPairsByTenant(ctx context.Context, tenantID int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_qa_pairs WHERE tenant_id = $1", tenantID)
	return err
}

// Agent Transcript methods

func (d *DB) CreateAgentTranscript(ctx context.Context, transcript *store.AgentTranscript) (*store.AgentTranscript, error) {
	messagesJSON, err := json.Marshal(transcript.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal messages: %w", err)
	}

	now := time.Now()
	if transcript.StartedAt.IsZero() {
		transcript.StartedAt = now
	}
	transcript.LastMessageAt = now

	stmt := `
		INSERT INTO agent_transcripts (
			id, tenant_id, session_id, audience_type, messages, message_count,
			client_ip, user_agent, customer_name, customer_phone, customer_email,
			customer_location, detected_intent, started_at, last_message_at,
			is_completed, completion_reason
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`
	_, err = d.db.ExecContext(ctx, stmt,
		transcript.ID, transcript.TenantID, transcript.SessionID, transcript.AudienceType,
		string(messagesJSON), transcript.MessageCount,
		transcript.ClientIP, transcript.UserAgent,
		transcript.CustomerName, transcript.CustomerPhone, transcript.CustomerEmail,
		transcript.CustomerLocation, transcript.DetectedIntent,
		transcript.StartedAt, transcript.LastMessageAt,
		transcript.IsCompleted, transcript.CompletionReason,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transcript: %w", err)
	}

	return transcript, nil
}

func (d *DB) GetAgentTranscript(ctx context.Context, find *store.FindAgentTranscript) (*store.AgentTranscript, error) {
	where := []string{"TRUE"}
	args := []any{}
	if find.ID != nil {
		args = append(args, *find.ID)
		where = append(where, fmt.Sprintf("id = $%d", len(args)))
	}
	if find.TenantID != nil {
		args = append(args, *find.TenantID)
		where = append(where, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if find.SessionID != nil {
		args = append(args, *find.SessionID)
		where = append(where, fmt.Sprintf("session_id = $%d", len(args)))
	}

	row := d.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, session_id, audience_type, messages, message_count,
			client_ip, user_agent, customer_name, customer_phone, customer_email,
			customer_location, detected_intent, started_at, ended_at, last_message_at,
			is_completed, completion_reason
		FROM agent_transcripts
		WHERE `+strings.Join(where, " AND ")+`
		LIMIT 1
	`, args...)

	var t store.AgentTranscript
	var messagesJSON string
	var clientIP, userAgent, customerName, customerPhone, customerEmail sql.NullString
	var customerLocation, detectedIntent, completionReason sql.NullString
	var endedAt sql.NullTime

	err := row.Scan(
		&t.ID, &t.TenantID, &t.SessionID, &t.AudienceType, &messagesJSON, &t.MessageCount,
		&clientIP, &userAgent, &customerName, &customerPhone, &customerEmail,
		&customerLocation, &detectedIntent, &t.StartedAt, &endedAt, &t.LastMessageAt,
		&t.IsCompleted, &completionReason,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan transcript: %w", err)
	}

	if err := json.Unmarshal([]byte(messagesJSON), &t.Messages); err != nil {
		return nil, fmt.Errorf("failed to unmarshal messages: %w", err)
	}

	t.ClientIP = clientIP.String
	t.UserAgent = userAgent.String
	t.CustomerName = customerName.String
	t.CustomerPhone = customerPhone.String
	t.CustomerEmail = customerEmail.String
	t.CustomerLocation = customerLocation.String
	t.DetectedIntent = detectedIntent.String
	t.CompletionReason = completionReason.String
	if endedAt.Valid {
		t.EndedAt = &endedAt.Time
	}

	return &t, nil
}

func (d *DB) ListAgentTranscripts(ctx context.Context, find *store.FindAgentTranscript) ([]*store.AgentTranscript, error) {
	where := []string{"TRUE"}
	args := []any{}
	if find.TenantID != nil {
		args = append(args, *find.TenantID)
		where = append(where, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if find.AudienceType != nil {
		args = append(args, *find.AudienceType)
		where = append(where, fmt.Sprintf("audience_type = $%d", len(args)))
	}

	limit := 100
	if find.Limit > 0 {
		limit = find.Limit
	}

	limitPos := len(args) + 1
	offsetPos := limitPos + 1

	query := fmt.Sprintf(`
		SELECT id, tenant_id, session_id, audience_type, messages, message_count,
			client_ip, user_agent, customer_name, customer_phone, customer_email,
			customer_location, detected_intent, started_at, ended_at, last_message_at,
			is_completed, completion_reason
		FROM agent_transcripts
		WHERE %s
		ORDER BY started_at DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(where, " AND "), limitPos, offsetPos)

	args = append(args, limit, find.Offset)

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list transcripts: %w", err)
	}
	defer rows.Close()

	var transcripts []*store.AgentTranscript
	for rows.Next() {
		var t store.AgentTranscript
		var messagesJSON string
		var clientIP, userAgent, customerName, customerPhone, customerEmail sql.NullString
		var customerLocation, detectedIntent, completionReason sql.NullString
		var endedAt sql.NullTime

		err := rows.Scan(
			&t.ID, &t.TenantID, &t.SessionID, &t.AudienceType, &messagesJSON, &t.MessageCount,
			&clientIP, &userAgent, &customerName, &customerPhone, &customerEmail,
			&customerLocation, &detectedIntent, &t.StartedAt, &endedAt, &t.LastMessageAt,
			&t.IsCompleted, &completionReason,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transcript: %w", err)
		}

		if err := json.Unmarshal([]byte(messagesJSON), &t.Messages); err != nil {
			return nil, fmt.Errorf("failed to unmarshal messages: %w", err)
		}

		t.ClientIP = clientIP.String
		t.UserAgent = userAgent.String
		t.CustomerName = customerName.String
		t.CustomerPhone = customerPhone.String
		t.CustomerEmail = customerEmail.String
		t.CustomerLocation = customerLocation.String
		t.DetectedIntent = detectedIntent.String
		t.CompletionReason = completionReason.String
		if endedAt.Valid {
			t.EndedAt = &endedAt.Time
		}

		transcripts = append(transcripts, &t)
	}

	return transcripts, nil
}

func (d *DB) UpdateAgentTranscript(ctx context.Context, transcript *store.AgentTranscript) error {
	messagesJSON, err := json.Marshal(transcript.Messages)
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}

	transcript.LastMessageAt = time.Now()

	stmt := `
		UPDATE agent_transcripts SET
			messages = $1,
			message_count = $2,
			customer_name = $3,
			customer_phone = $4,
			customer_email = $5,
			customer_location = $6,
			detected_intent = $7,
			last_message_at = $8,
			ended_at = $9,
			is_completed = $10,
			completion_reason = $11
		WHERE id = $12
	`
	_, err = d.db.ExecContext(ctx, stmt,
		string(messagesJSON), transcript.MessageCount,
		transcript.CustomerName, transcript.CustomerPhone, transcript.CustomerEmail,
		transcript.CustomerLocation, transcript.DetectedIntent,
		transcript.LastMessageAt, transcript.EndedAt,
		transcript.IsCompleted, transcript.CompletionReason,
		transcript.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update transcript: %w", err)
	}

	return nil
}

func (d *DB) DeleteAgentTranscript(ctx context.Context, id string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_transcripts WHERE id = $1", id)
	return err
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
	now := time.Now()
	checkpoint.UpdatedAt = now

	if checkpoint.Status == "completed" && checkpoint.CompletedAt == nil {
		checkpoint.CompletedAt = &now
	}

	var fileType sql.NullString
	if checkpoint.FileType != nil {
		fileType = sql.NullString{String: *checkpoint.FileType, Valid: true}
	}
	var version sql.NullInt32
	if checkpoint.Version != nil {
		version = sql.NullInt32{Int32: *checkpoint.Version, Valid: true}
	}

	stmt := `
		INSERT INTO agent_reindex_checkpoints (
			tenant_id, audience, file_type, version, total_chunks, processed_chunks, current_batch,
			total_batches, batch_size, status, error_message, last_message, error_batch,
			started_at, updated_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT(tenant_id, audience, file_type, version) DO UPDATE SET
			total_chunks = excluded.total_chunks,
			processed_chunks = excluded.processed_chunks,
			current_batch = excluded.current_batch,
			total_batches = excluded.total_batches,
			batch_size = excluded.batch_size,
			status = excluded.status,
			error_message = excluded.error_message,
			last_message = excluded.last_message,
			error_batch = excluded.error_batch,
			updated_at = excluded.updated_at,
			completed_at = excluded.completed_at
		RETURNING id
	`

	err := d.db.QueryRowContext(ctx, stmt,
		checkpoint.TenantID, checkpoint.Audience, fileType, version, checkpoint.TotalChunks,
		checkpoint.ProcessedChunks, checkpoint.CurrentBatch, checkpoint.TotalBatches,
		checkpoint.BatchSize, checkpoint.Status, checkpoint.ErrorMessage,
		checkpoint.LastMessage, checkpoint.ErrorBatch, checkpoint.StartedAt,
		checkpoint.UpdatedAt, checkpoint.CompletedAt,
	).Scan(&checkpoint.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert reindex checkpoint: %w", err)
	}

	return checkpoint, nil
}

func (d *DB) GetReindexCheckpoint(ctx context.Context, find *store.FindReindexCheckpoint) (*store.ReindexCheckpoint, error) {
	where := []string{"TRUE"}
	args := []any{}
	if find.TenantID != nil {
		args = append(args, *find.TenantID)
		where = append(where, fmt.Sprintf("tenant_id = $%d", len(args)))
	}
	if find.Audience != nil {
		args = append(args, *find.Audience)
		where = append(where, fmt.Sprintf("audience = $%d", len(args)))
	}
	if find.FileType != nil {
		args = append(args, *find.FileType)
		where = append(where, fmt.Sprintf("file_type = $%d", len(args)))
	}
	if find.Version != nil {
		args = append(args, *find.Version)
		where = append(where, fmt.Sprintf("version = $%d", len(args)))
	}
	if find.Status != nil {
		args = append(args, *find.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}

	var c store.ReindexCheckpoint
	var errorMessage, errorBatch, fileType sql.NullString
	var version sql.NullInt32
	var completedAt sql.NullTime

	err := d.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, audience, file_type, version, total_chunks, processed_chunks, current_batch,
			total_batches, batch_size, status, error_message, last_message, error_batch,
			started_at, updated_at, completed_at
		FROM agent_reindex_checkpoints
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY updated_at DESC
		LIMIT 1
	`, args...).Scan(
		&c.ID, &c.TenantID, &c.Audience, &fileType, &version, &c.TotalChunks, &c.ProcessedChunks,
		&c.CurrentBatch, &c.TotalBatches, &c.BatchSize, &c.Status,
		&errorMessage, &c.LastMessage, &errorBatch, &c.StartedAt, &c.UpdatedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get reindex checkpoint: %w", err)
	}

	if fileType.Valid {
		ft := fileType.String
		c.FileType = &ft
	}
	if version.Valid {
		v := version.Int32
		c.Version = &v
	}
	c.ErrorMessage = errorMessage.String
	if errorBatch.Valid {
		batch := int32(0)
		fmt.Sscanf(errorBatch.String, "%d", &batch)
		c.ErrorBatch = &batch
	}
	if completedAt.Valid {
		c.CompletedAt = &completedAt.Time
	}

	return &c, nil
}

func (d *DB) DeleteReindexCheckpoint(ctx context.Context, tenantID int32, audience string) error {
	_, err := d.db.ExecContext(ctx,
		"DELETE FROM agent_reindex_checkpoints WHERE tenant_id = $1 AND audience = $2",
		tenantID, audience,
	)
	return err
}

func (d *DB) SupportsBridgeDelivery() bool {
	return true
}

// ============================================================================
// AGENT INTEGRATIONS
// ============================================================================

func (d *DB) CreateAgentIntegration(ctx context.Context, integration *store.AgentIntegration) (*store.AgentIntegration, error) {
	stmt := `
		INSERT INTO agent_integrations (tenant_id, integration_type, label, config, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT)
		RETURNING id
	`
	var id int32
	err := d.db.QueryRowContext(ctx, stmt,
		integration.TenantID, integration.IntegrationType, integration.Label,
		integration.Config, integration.IsActive,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent integration: %w", err)
	}
	integration.ID = id
	return integration, nil
}

func (d *DB) GetAgentIntegration(ctx context.Context, find *store.FindAgentIntegration) (*store.AgentIntegration, error) {
	query := "SELECT id, tenant_id, integration_type, label, config, is_active, created_at, updated_at FROM agent_integrations WHERE 1=1"
	args := []any{}
	argIdx := 1
	if find.ID != nil {
		query += fmt.Sprintf(" AND id = $%d", argIdx)
		args = append(args, *find.ID)
		argIdx++
	}
	if find.TenantID != nil {
		query += fmt.Sprintf(" AND tenant_id = $%d", argIdx)
		args = append(args, *find.TenantID)
		argIdx++
	}
	if find.IntegrationType != nil {
		query += fmt.Sprintf(" AND integration_type = $%d", argIdx)
		args = append(args, *find.IntegrationType)
		argIdx++
	}
	query += " LIMIT 1"

	var integration store.AgentIntegration
	err := d.db.QueryRowContext(ctx, query, args...).Scan(
		&integration.ID, &integration.TenantID, &integration.IntegrationType,
		&integration.Label, &integration.Config, &integration.IsActive,
		&integration.CreatedAt, &integration.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent integration: %w", err)
	}
	return &integration, nil
}

func (d *DB) ListAgentIntegrations(ctx context.Context, find *store.FindAgentIntegration) ([]*store.AgentIntegration, error) {
	query := "SELECT id, tenant_id, integration_type, label, config, is_active, created_at, updated_at FROM agent_integrations WHERE 1=1"
	args := []any{}
	argIdx := 1
	if find.TenantID != nil {
		query += fmt.Sprintf(" AND tenant_id = $%d", argIdx)
		args = append(args, *find.TenantID)
		argIdx++
	}
	if find.IntegrationType != nil {
		query += fmt.Sprintf(" AND integration_type = $%d", argIdx)
		args = append(args, *find.IntegrationType)
		argIdx++
	}
	query += " ORDER BY created_at DESC"

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent integrations: %w", err)
	}
	defer rows.Close()

	var integrations []*store.AgentIntegration
	for rows.Next() {
		var integration store.AgentIntegration
		if err := rows.Scan(
			&integration.ID, &integration.TenantID, &integration.IntegrationType,
			&integration.Label, &integration.Config, &integration.IsActive,
			&integration.CreatedAt, &integration.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan agent integration: %w", err)
		}
		integrations = append(integrations, &integration)
	}
	return integrations, nil
}

func (d *DB) UpdateAgentIntegration(ctx context.Context, update *store.AgentIntegration) error {
	stmt := `
		UPDATE agent_integrations
		SET label = $1, config = $2, is_active = $3, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
		WHERE id = $4
	`
	_, err := d.db.ExecContext(ctx, stmt, update.Label, update.Config, update.IsActive, update.ID)
	if err != nil {
		return fmt.Errorf("failed to update agent integration: %w", err)
	}
	return nil
}

func (d *DB) DeleteAgentIntegration(ctx context.Context, id int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM agent_integrations WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete agent integration: %w", err)
	}
	return nil
}

// ============================================================================
// AGENT EVENTS
// ============================================================================

func (d *DB) CreateAgentEvent(ctx context.Context, event *store.AgentEvent) (*store.AgentEvent, error) {
	stmt := `
		INSERT INTO agent_events (tenant_id, integration_id, event_type, payload, status, claimed_at, attempts, last_error, idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, EXTRACT(EPOCH FROM NOW())::BIGINT)
		RETURNING id
	`
	var id int32
	err := d.db.QueryRowContext(ctx, stmt,
		event.TenantID, event.IntegrationID, event.EventType,
		event.Payload, event.Status, event.ClaimedAt,
		event.Attempts, event.LastError, event.IdempotencyKey,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent event: %w", err)
	}
	event.ID = id
	return event, nil
}

func (d *DB) ListAgentEvents(ctx context.Context, find *store.FindAgentEvent) ([]*store.AgentEvent, error) {
	query := "SELECT id, tenant_id, integration_id, event_type, payload, status, claimed_at, attempts, last_error, idempotency_key, created_at FROM agent_events WHERE 1=1"
	args := []any{}
	argIdx := 1
	if find.TenantID != nil {
		query += fmt.Sprintf(" AND tenant_id = $%d", argIdx)
		args = append(args, *find.TenantID)
		argIdx++
	}
	if find.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *find.Status)
		argIdx++
	}
	query += " ORDER BY created_at DESC"

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent events: %w", err)
	}
	defer rows.Close()

	var events []*store.AgentEvent
	for rows.Next() {
		var event store.AgentEvent
		if err := rows.Scan(
			&event.ID, &event.TenantID, &event.IntegrationID,
			&event.EventType, &event.Payload, &event.Status,
			&event.ClaimedAt, &event.Attempts, &event.LastError,
			&event.IdempotencyKey, &event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan agent event: %w", err)
		}
		events = append(events, &event)
	}
	return events, nil
}

func (d *DB) ClaimPendingEvents(ctx context.Context, limit int32) ([]*store.AgentEvent, error) {
	// Postgres claim with FOR UPDATE SKIP LOCKED for concurrent safety
	stmt := `
		UPDATE agent_events
		SET status = 'processing', claimed_at = EXTRACT(EPOCH FROM NOW())::BIGINT, attempts = attempts + 1
		WHERE id IN (
			SELECT id FROM agent_events
			WHERE (status = 'pending' AND attempts < 5)
			   OR (status = 'processing' AND claimed_at < EXTRACT(EPOCH FROM NOW())::BIGINT - 300 AND attempts < 5)
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		RETURNING id, tenant_id, integration_id, event_type, payload, status, claimed_at, attempts, last_error, idempotency_key, created_at
	`
	rows, err := d.db.QueryContext(ctx, stmt, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to claim pending events: %w", err)
	}
	defer rows.Close()

	var events []*store.AgentEvent
	for rows.Next() {
		var event store.AgentEvent
		if err := rows.Scan(
			&event.ID, &event.TenantID, &event.IntegrationID,
			&event.EventType, &event.Payload, &event.Status,
			&event.ClaimedAt, &event.Attempts, &event.LastError,
			&event.IdempotencyKey, &event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan claimed event: %w", err)
		}
		events = append(events, &event)
	}
	return events, nil
}

func (d *DB) UpdateAgentEvent(ctx context.Context, update *store.AgentEvent) error {
	stmt := `
		UPDATE agent_events
		SET status = $1, last_error = $2, attempts = $3
		WHERE id = $4
	`
	_, err := d.db.ExecContext(ctx, stmt, update.Status, update.LastError, update.Attempts, update.ID)
	if err != nil {
		return fmt.Errorf("failed to update agent event: %w", err)
	}
	return nil
}
