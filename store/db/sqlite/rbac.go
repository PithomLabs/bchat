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
// USER-TENANT PERMISSION OPERATIONS
// ============================================================================

func (d *DB) CreateUserTenantPermission(ctx context.Context, perm *store.UserTenantPermission) (*store.UserTenantPermission, error) {
	stmt := `
		INSERT INTO user_tenant_permission (user_id, tenant_id, permissions, granted_by, granted_at)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id
	`
	now := time.Now()
	permissionsStr := strings.Join(perm.Permissions, ",")
	if err := d.db.QueryRowContext(ctx, stmt,
		perm.UserID, perm.TenantID, permissionsStr, perm.GrantedBy, now.Unix(),
	).Scan(&perm.ID); err != nil {
		return nil, err
	}
	perm.GrantedAt = now
	return perm, nil
}

func (d *DB) GetUserTenantPermission(ctx context.Context, find *store.FindUserTenantPermission) (*store.UserTenantPermission, error) {
	perms, err := d.ListUserTenantPermissions(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(perms) == 0 {
		return nil, nil
	}
	return perms[0], nil
}

func (d *DB) ListUserTenantPermissions(ctx context.Context, find *store.FindUserTenantPermission) ([]*store.UserTenantPermission, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if find.ID != nil {
		where = append(where, "id = ?")
		args = append(args, *find.ID)
	}
	if find.UserID != nil {
		where = append(where, "user_id = ?")
		args = append(args, *find.UserID)
	}
	if find.TenantID != nil {
		where = append(where, "tenant_id = ?")
		args = append(args, *find.TenantID)
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, tenant_id, permissions, granted_by, granted_at
		FROM user_tenant_permission
		WHERE %s
		ORDER BY granted_at DESC
	`, strings.Join(where, " AND "))

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []*store.UserTenantPermission
	for rows.Next() {
		var p store.UserTenantPermission
		var permissionsStr sql.NullString
		var grantedBy sql.NullInt32
		var grantedAtUnix int64
		if err := rows.Scan(&p.ID, &p.UserID, &p.TenantID, &permissionsStr, &grantedBy, &grantedAtUnix); err != nil {
			return nil, err
		}
		if permissionsStr.Valid && permissionsStr.String != "" {
			p.Permissions = strings.Split(permissionsStr.String, ",")
		} else {
			p.Permissions = []string{}
		}
		if grantedBy.Valid {
			p.GrantedBy = &grantedBy.Int32
		}
		p.GrantedAt = time.Unix(grantedAtUnix, 0)
		perms = append(perms, &p)
	}
	return perms, rows.Err()
}

func (d *DB) UpdateUserTenantPermission(ctx context.Context, perm *store.UserTenantPermission) (*store.UserTenantPermission, error) {
	stmt := `
		UPDATE user_tenant_permission
		SET permissions = ?, granted_by = ?, granted_at = ?
		WHERE id = ?
	`
	now := time.Now()
	permissionsStr := strings.Join(perm.Permissions, ",")
	_, err := d.db.ExecContext(ctx, stmt, permissionsStr, perm.GrantedBy, now.Unix(), perm.ID)
	if err != nil {
		return nil, err
	}
	perm.GrantedAt = now
	return perm, nil
}

func (d *DB) DeleteUserTenantPermission(ctx context.Context, userID, tenantID int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM user_tenant_permission WHERE user_id = ? AND tenant_id = ?", userID, tenantID)
	return err
}

// ============================================================================
// TENANT CONFIG OPERATIONS
// ============================================================================

func (d *DB) GetTenantConfig(ctx context.Context, find *store.FindTenantConfig) (*store.TenantConfig, error) {
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
		SELECT id, tenant_id, llm_model, simulation_human_model, reasoning_model, openrouter_api_key_encrypted, openrouter_api_key_nonce, features, retrieval_mode, content_tokens, record_transcripts, updated_at, updated_by
		FROM tenant_config
		WHERE %s
		LIMIT 1
	`, strings.Join(where, " AND "))

	var config store.TenantConfig
	var llmModel sql.NullString
	var simulationHumanModel sql.NullString
	var reasoningModel sql.NullString
	var apiKeyEncrypted, apiKeyNonce []byte
	var featuresJSON sql.NullString
	var retrievalMode sql.NullString
	var contentTokens sql.NullInt32
	var recordTranscripts sql.NullInt64
	var updatedAtUnix int64
	var updatedBy sql.NullInt32

	err := d.db.QueryRowContext(ctx, query, args...).Scan(
		&config.ID, &config.TenantID, &llmModel, &simulationHumanModel, &reasoningModel,
		&apiKeyEncrypted, &apiKeyNonce, &featuresJSON,
		&retrievalMode, &contentTokens, &recordTranscripts,
		&updatedAtUnix, &updatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if llmModel.Valid {
		config.LLMModel = llmModel.String
	}
	if simulationHumanModel.Valid {
		config.SimulationHumanModel = simulationHumanModel.String
	}
	if reasoningModel.Valid {
		config.ReasoningModel = reasoningModel.String
	}
	config.OpenRouterAPIKeyEncrypted = apiKeyEncrypted
	config.OpenRouterAPIKeyNonce = apiKeyNonce
	if featuresJSON.Valid && featuresJSON.String != "" {
		json.Unmarshal([]byte(featuresJSON.String), &config.Features)
	} else {
		config.Features = make(map[string]interface{})
	}
	if retrievalMode.Valid {
		config.RetrievalMode = retrievalMode.String
	} else {
		config.RetrievalMode = "long_context" // Default
	}
	if contentTokens.Valid {
		config.ContentTokens = contentTokens.Int32
	}
	// Default to true if not set (NULL or 1)
	config.RecordTranscripts = !recordTranscripts.Valid || recordTranscripts.Int64 == 1
	config.UpdatedAt = time.Unix(updatedAtUnix, 0)
	if updatedBy.Valid {
		config.UpdatedBy = &updatedBy.Int32
	}

	return &config, nil
}

func (d *DB) UpsertTenantConfig(ctx context.Context, config *store.TenantConfig) (*store.TenantConfig, error) {
	featuresJSON, _ := json.Marshal(config.Features)
	if config.Features == nil {
		featuresJSON = []byte("{}")
	}
	now := time.Now()

	// Default retrieval mode if not set
	if config.RetrievalMode == "" {
		config.RetrievalMode = "long_context"
	}

	// Convert bool to int for SQLite
	recordTranscriptsInt := 0
	if config.RecordTranscripts {
		recordTranscriptsInt = 1
	}

	stmt := `
		INSERT INTO tenant_config (tenant_id, llm_model, simulation_human_model, reasoning_model, openrouter_api_key_encrypted, openrouter_api_key_nonce, features, retrieval_mode, content_tokens, record_transcripts, updated_at, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id) DO UPDATE SET
			llm_model = excluded.llm_model,
			simulation_human_model = excluded.simulation_human_model,
			reasoning_model = excluded.reasoning_model,
			openrouter_api_key_encrypted = COALESCE(excluded.openrouter_api_key_encrypted, tenant_config.openrouter_api_key_encrypted),
			openrouter_api_key_nonce = COALESCE(excluded.openrouter_api_key_nonce, tenant_config.openrouter_api_key_nonce),
			features = excluded.features,
			retrieval_mode = excluded.retrieval_mode,
			content_tokens = excluded.content_tokens,
			record_transcripts = excluded.record_transcripts,
			updated_at = excluded.updated_at,
			updated_by = excluded.updated_by
		RETURNING id
	`
	if err := d.db.QueryRowContext(ctx, stmt,
		config.TenantID, config.LLMModel, config.SimulationHumanModel, config.ReasoningModel, config.OpenRouterAPIKeyEncrypted, config.OpenRouterAPIKeyNonce,
		string(featuresJSON), config.RetrievalMode, config.ContentTokens, recordTranscriptsInt, now.Unix(), config.UpdatedBy,
	).Scan(&config.ID); err != nil {
		return nil, err
	}
	config.UpdatedAt = now
	return config, nil
}

func (d *DB) DeleteTenantConfig(ctx context.Context, tenantID int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM tenant_config WHERE tenant_id = ?", tenantID)
	return err
}

// ============================================================================
// SYSTEM SECRET OPERATIONS
// ============================================================================

func (d *DB) GetSystemSecret(ctx context.Context) (*store.SystemSecret, error) {
	query := `
		SELECT id, encryption_salt, key_version, created_at, rotated_at
		FROM system_secret
		WHERE id = 1
	`

	var secret store.SystemSecret
	var createdAtUnix int64
	var rotatedAtUnix sql.NullInt64

	err := d.db.QueryRowContext(ctx, query).Scan(
		&secret.ID, &secret.EncryptionSalt, &secret.KeyVersion, &createdAtUnix, &rotatedAtUnix,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	secret.CreatedAt = time.Unix(createdAtUnix, 0)
	if rotatedAtUnix.Valid {
		t := time.Unix(rotatedAtUnix.Int64, 0)
		secret.RotatedAt = &t
	}

	return &secret, nil
}

func (d *DB) UpsertSystemSecret(ctx context.Context, secret *store.SystemSecret) (*store.SystemSecret, error) {
	now := time.Now()

	stmt := `
		INSERT INTO system_secret (id, encryption_salt, key_version, created_at)
		VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			encryption_salt = excluded.encryption_salt,
			key_version = excluded.key_version,
			rotated_at = ?
		RETURNING id
	`
	if err := d.db.QueryRowContext(ctx, stmt,
		secret.EncryptionSalt, secret.KeyVersion, now.Unix(), now.Unix(),
	).Scan(&secret.ID); err != nil {
		return nil, err
	}
	secret.CreatedAt = now
	return secret, nil
}
