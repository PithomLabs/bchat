package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateUserTenantPermission(ctx context.Context, perm *store.UserTenantPermission) (*store.UserTenantPermission, error) {
	now := time.Now()
	err := d.db.QueryRowContext(ctx, `
		INSERT INTO user_tenant_permission(user_id,tenant_id,permissions,granted_by,granted_at,source_template_id)
		VALUES($1,$2,$3,$4,$5,$6) RETURNING id
	`, perm.UserID, perm.TenantID, strings.Join(perm.Permissions, ","), perm.GrantedBy, now.Unix(), perm.SourceTemplateID).Scan(&perm.ID)
	if err != nil {
		return nil, err
	}
	perm.GrantedAt = now
	return perm, nil
}

func (d *DB) GetUserTenantPermission(ctx context.Context, find *store.FindUserTenantPermission) (*store.UserTenantPermission, error) {
	list, err := d.ListUserTenantPermissions(ctx, find)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return list[0], nil
}

func (d *DB) ListUserTenantPermissions(ctx context.Context, find *store.FindUserTenantPermission) ([]*store.UserTenantPermission, error) {
	where := []string{"TRUE"}
	args := []any{}
	if find.ID != nil {
		args = append(args, *find.ID)
		where = append(where, fmt.Sprintf("id=$%d", len(args)))
	}
	if find.UserID != nil {
		args = append(args, *find.UserID)
		where = append(where, fmt.Sprintf("user_id=$%d", len(args)))
	}
	if find.TenantID != nil {
		args = append(args, *find.TenantID)
		where = append(where, fmt.Sprintf("tenant_id=$%d", len(args)))
	}
	if find.SourceTemplateID != nil {
		if *find.SourceTemplateID == 0 {
			where = append(where, "source_template_id IS NULL")
		} else {
			args = append(args, *find.SourceTemplateID)
			where = append(where, fmt.Sprintf("source_template_id=$%d", len(args)))
		}
	}
	rows, err := d.db.QueryContext(ctx, `
		SELECT id,user_id,tenant_id,permissions,granted_by,granted_at,source_template_id
		FROM user_tenant_permission WHERE `+strings.Join(where, " AND ")+` ORDER BY granted_at DESC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*store.UserTenantPermission
	for rows.Next() {
		var perm store.UserTenantPermission
		var permissions string
		var grantedBy sql.NullInt32
		var grantedAt int64
		var sourceTemplateID sql.NullInt32
		if err := rows.Scan(&perm.ID, &perm.UserID, &perm.TenantID, &permissions, &grantedBy, &grantedAt, &sourceTemplateID); err != nil {
			return nil, err
		}
		if permissions == "" {
			perm.Permissions = []string{}
		} else {
			perm.Permissions = strings.Split(permissions, ",")
		}
		if grantedBy.Valid {
			perm.GrantedBy = &grantedBy.Int32
		}
		if sourceTemplateID.Valid {
			id := sourceTemplateID.Int32
			perm.SourceTemplateID = &id
		}
		perm.GrantedAt = time.Unix(grantedAt, 0)
		result = append(result, &perm)
	}
	return result, rows.Err()
}

func (d *DB) UpdateUserTenantPermission(ctx context.Context, perm *store.UserTenantPermission) (*store.UserTenantPermission, error) {
	now := time.Now()
	_, err := d.db.ExecContext(ctx, `
		UPDATE user_tenant_permission SET permissions=$1,granted_by=$2,granted_at=$3,source_template_id=$4 WHERE id=$5
	`, strings.Join(perm.Permissions, ","), perm.GrantedBy, now.Unix(), perm.SourceTemplateID, perm.ID)
	if err != nil {
		return nil, err
	}
	perm.GrantedAt = now
	return perm, nil
}

func (d *DB) DeleteUserTenantPermission(ctx context.Context, userID, tenantID, id int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM user_tenant_permission WHERE user_id=$1 AND tenant_id=$2 AND id=$3", userID, tenantID, id)
	return err
}

// DeleteAllUserTenantPermissions removes every permission row for the given user/tenant,
// including template-linked assignments. Prefer the more specific methods below.
func (d *DB) DeleteAllUserTenantPermissions(ctx context.Context, userID, tenantID int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM user_tenant_permission WHERE user_id=$1 AND tenant_id=$2", userID, tenantID)
	return err
}

func (d *DB) DeleteExplicitUserTenantPermissions(ctx context.Context, userID, tenantID int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM user_tenant_permission WHERE user_id=$1 AND tenant_id=$2 AND source_template_id IS NULL", userID, tenantID)
	return err
}

func (d *DB) GetTenantConfig(ctx context.Context, find *store.FindTenantConfig) (*store.TenantConfig, error) {
	where, args := []string{"TRUE"}, []any{}
	if find.ID != nil {
		args = append(args, *find.ID)
		where = append(where, fmt.Sprintf("id=$%d", len(args)))
	}
	if find.TenantID != nil {
		args = append(args, *find.TenantID)
		where = append(where, fmt.Sprintf("tenant_id=$%d", len(args)))
	}
	var config store.TenantConfig
	var features []byte
	var updatedAt int64
	var adminMutationRateLimit sql.NullInt32
	var vectorDBS3Override sql.NullString
	err := d.db.QueryRowContext(ctx, `
		SELECT id,tenant_id,llm_model,simulation_human_model,reasoning_model,
			openrouter_api_key_encrypted,openrouter_api_key_nonce,features,retrieval_mode,
			content_tokens,record_transcripts,admin_mutation_rate_limit_rpm,
			vector_db_s3_override,updated_at,updated_by
		FROM tenant_config WHERE `+strings.Join(where, " AND ")+` LIMIT 1
	`, args...).Scan(&config.ID, &config.TenantID, &config.LLMModel, &config.SimulationHumanModel,
		&config.ReasoningModel, &config.OpenRouterAPIKeyEncrypted, &config.OpenRouterAPIKeyNonce,
		&features, &config.RetrievalMode, &config.ContentTokens, &config.RecordTranscripts,
		&adminMutationRateLimit, &vectorDBS3Override, &updatedAt, &config.UpdatedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(features, &config.Features)
	config.UpdatedAt = time.Unix(updatedAt, 0)
	if adminMutationRateLimit.Valid {
		config.AdminMutationRateLimitRPM = int(adminMutationRateLimit.Int32)
	} else {
		config.AdminMutationRateLimitRPM = 30
	}
	if vectorDBS3Override.Valid {
		config.VectorDBS3Override = vectorDBS3Override.String
	}
	return &config, nil
}

func (d *DB) UpsertTenantConfig(ctx context.Context, config *store.TenantConfig) (*store.TenantConfig, error) {
	if config.Features == nil {
		config.Features = map[string]interface{}{}
	}
	features, err := json.Marshal(config.Features)
	if err != nil {
		return nil, err
	}
	featuresStr := string(features)
	now := time.Now()
	if config.AdminMutationRateLimitRPM == 0 {
		config.AdminMutationRateLimitRPM = 30
	}
	err = d.db.QueryRowContext(ctx, `
		INSERT INTO tenant_config(tenant_id,llm_model,simulation_human_model,reasoning_model,
			openrouter_api_key_encrypted,openrouter_api_key_nonce,features,retrieval_mode,
			content_tokens,record_transcripts,admin_mutation_rate_limit_rpm,
			vector_db_s3_override,updated_at,updated_by)
		VALUES($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT(tenant_id) DO UPDATE SET
			llm_model=EXCLUDED.llm_model,simulation_human_model=EXCLUDED.simulation_human_model,
			reasoning_model=EXCLUDED.reasoning_model,
			openrouter_api_key_encrypted=COALESCE(EXCLUDED.openrouter_api_key_encrypted,tenant_config.openrouter_api_key_encrypted),
			openrouter_api_key_nonce=COALESCE(EXCLUDED.openrouter_api_key_nonce,tenant_config.openrouter_api_key_nonce),
			features=EXCLUDED.features::jsonb,retrieval_mode=EXCLUDED.retrieval_mode,
			content_tokens=EXCLUDED.content_tokens,record_transcripts=EXCLUDED.record_transcripts,
			admin_mutation_rate_limit_rpm=EXCLUDED.admin_mutation_rate_limit_rpm,
			vector_db_s3_override=COALESCE(EXCLUDED.vector_db_s3_override,tenant_config.vector_db_s3_override),
			updated_at=EXCLUDED.updated_at,updated_by=EXCLUDED.updated_by
		RETURNING id
	`, config.TenantID, config.LLMModel, config.SimulationHumanModel, config.ReasoningModel,
		config.OpenRouterAPIKeyEncrypted, config.OpenRouterAPIKeyNonce, featuresStr, config.RetrievalMode,
		config.ContentTokens, config.RecordTranscripts, config.AdminMutationRateLimitRPM,
		config.VectorDBS3Override, now.Unix(), config.UpdatedBy).Scan(&config.ID)
	if err != nil {
		return nil, err
	}
	config.UpdatedAt = now
	return config, nil
}

func (d *DB) DeleteTenantConfig(ctx context.Context, tenantID int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM tenant_config WHERE tenant_id=$1", tenantID)
	return err
}

func (d *DB) GetSystemSecret(ctx context.Context) (*store.SystemSecret, error) {
	return nil, nil
}

func (d *DB) UpsertSystemSecret(ctx context.Context, secret *store.SystemSecret) (*store.SystemSecret, error) {
	return nil, nil
}

func (d *DB) CreateTenantRoleTemplate(ctx context.Context, template *store.TenantRoleTemplate) (*store.TenantRoleTemplate, error) {
	var tenantID interface{}
	if template.TenantID != nil {
		tenantID = *template.TenantID
	} else {
		tenantID = nil
	}
	var createdBy interface{}
	if template.CreatedBy != nil {
		createdBy = *template.CreatedBy
	}
	permissionsJSON, _ := json.Marshal(template.Permissions)
	err := d.db.QueryRowContext(ctx, `
		INSERT INTO tenant_role_templates(tenant_id,name,code,permissions,created_by)
		VALUES($1,$2,$3,$4,$5)
		RETURNING id,created_at,updated_at
	`, tenantID, template.Name, template.Code, string(permissionsJSON), createdBy).Scan(
		&template.ID, &template.CreatedAt, &template.UpdatedAt)
	if err != nil {
		return nil, err
	}
	template.TenantID = nil
	if tenantID != nil {
		tid := tenantID.(int32)
		template.TenantID = &tid
	}
	return template, nil
}

func (d *DB) GetTenantRoleTemplate(ctx context.Context, find *store.FindTenantRoleTemplate) (*store.TenantRoleTemplate, error) {
	where := []string{"TRUE"}
	args := []any{}
	if find.ID != nil {
		args = append(args, *find.ID)
		where = append(where, fmt.Sprintf("id=$%d", len(args)))
	}
	if find.ID == nil {
		if find.TenantID != nil {
			if *find.TenantID == -1 {
				where = append(where, "tenant_id IS NULL")
			} else {
				args = append(args, *find.TenantID)
				where = append(where, fmt.Sprintf("tenant_id=$%d", len(args)))
			}
		} else {
			where = append(where, "tenant_id IS NULL")
		}
	}
	if find.Code != nil {
		args = append(args, *find.Code)
		where = append(where, fmt.Sprintf("code=$%d", len(args)))
	}
	if find.Name != nil {
		args = append(args, *find.Name)
		where = append(where, fmt.Sprintf("name=$%d", len(args)))
	}

	var template store.TenantRoleTemplate
	var tenantID sql.NullInt32
	var permissionsJSON []byte
	var createdBy sql.NullInt32
	var createdAtUnix, updatedAtUnix int64

	err := d.db.QueryRowContext(ctx, `
		SELECT id,tenant_id,name,code,permissions,created_by,created_at,updated_at
		FROM tenant_role_templates WHERE `+strings.Join(where, " AND ")+` LIMIT 1
	`, args...).Scan(
		&template.ID, &tenantID, &template.Name, &template.Code,
		&permissionsJSON, &createdBy, &createdAtUnix, &updatedAtUnix,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if tenantID.Valid && tenantID.Int32 != -1 {
		tid := tenantID.Int32
		template.TenantID = &tid
	}
	if createdBy.Valid {
		id := createdBy.Int32
		template.CreatedBy = &id
	}
	if len(permissionsJSON) > 0 {
		json.Unmarshal(permissionsJSON, &template.Permissions)
	} else {
		template.Permissions = []string{}
	}
	template.CreatedAt = time.Unix(createdAtUnix, 0)
	template.UpdatedAt = time.Unix(updatedAtUnix, 0)
	return &template, nil
}

func (d *DB) ListTenantRoleTemplates(ctx context.Context, find *store.FindTenantRoleTemplate) ([]*store.TenantRoleTemplate, error) {
	where := []string{"TRUE"}
	args := []any{}
	if find.ID != nil {
		args = append(args, *find.ID)
		where = append(where, fmt.Sprintf("id=$%d", len(args)))
	}
	if find.TenantID != nil {
		if *find.TenantID == -1 {
			where = append(where, "tenant_id IS NULL")
		} else {
			args = append(args, *find.TenantID)
			where = append(where, fmt.Sprintf("tenant_id=$%d", len(args)))
		}
	} else {
		where = append(where, "tenant_id IS NULL")
	}
	if find.Code != nil {
		args = append(args, *find.Code)
		where = append(where, fmt.Sprintf("code=$%d", len(args)))
	}
	if find.Name != nil {
		args = append(args, *find.Name)
		where = append(where, fmt.Sprintf("name=$%d", len(args)))
	}

	rows, err := d.db.QueryContext(ctx, `
		SELECT id,tenant_id,name,code,permissions,created_by,created_at,updated_at
		FROM tenant_role_templates WHERE `+strings.Join(where, " AND ")+` ORDER BY code ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*store.TenantRoleTemplate
	for rows.Next() {
		var template store.TenantRoleTemplate
		var tenantID sql.NullInt32
		var permissionsJSON []byte
		var createdBy sql.NullInt32
		var createdAtUnix, updatedAtUnix int64
		if err := rows.Scan(&template.ID, &tenantID, &template.Name, &template.Code,
			&permissionsJSON, &createdBy, &createdAtUnix, &updatedAtUnix); err != nil {
			return nil, err
		}
		if tenantID.Valid && tenantID.Int32 != -1 {
			tid := tenantID.Int32
			template.TenantID = &tid
		}
		if createdBy.Valid {
			id := createdBy.Int32
			template.CreatedBy = &id
		}
		if len(permissionsJSON) > 0 {
			json.Unmarshal(permissionsJSON, &template.Permissions)
		} else {
			template.Permissions = []string{}
		}
		template.CreatedAt = time.Unix(createdAtUnix, 0)
		template.UpdatedAt = time.Unix(updatedAtUnix, 0)
		templates = append(templates, &template)
	}
	return templates, rows.Err()
}

func (d *DB) UpdateTenantRoleTemplate(ctx context.Context, template *store.TenantRoleTemplate) (*store.TenantRoleTemplate, error) {
	existing, err := d.GetTenantRoleTemplate(ctx, &store.FindTenantRoleTemplate{ID: &template.ID})
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, sql.ErrNoRows
	}

	if template.Name != "" {
		existing.Name = template.Name
	}
	if template.Code != "" {
		existing.Code = template.Code
	}
	if template.Permissions != nil {
		existing.Permissions = template.Permissions
	}

	now := time.Now()
	permissionsJSON, _ := json.Marshal(existing.Permissions)
	_, err = d.db.ExecContext(ctx, `
		UPDATE tenant_role_templates SET name=$1, code=$2, permissions=$3, updated_at=$4 WHERE id=$5
	`, existing.Name, existing.Code, string(permissionsJSON), now.Unix(), template.ID)
	if err != nil {
		return nil, err
	}
	existing.UpdatedAt = now
	return existing, nil
}

func (d *DB) DeleteTenantRoleTemplate(ctx context.Context, id int32) error {
	var count int
	if err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_tenant_permission WHERE source_template_id=$1", id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("cannot delete role template with active assignments")
	}
	_, err := d.db.ExecContext(ctx, "DELETE FROM tenant_role_templates WHERE id=$1", id)
	return err
}
