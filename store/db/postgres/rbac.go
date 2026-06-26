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
		INSERT INTO user_tenant_permission(user_id,tenant_id,permissions,granted_by,granted_at)
		VALUES($1,$2,$3,$4,$5) RETURNING id
	`, perm.UserID, perm.TenantID, strings.Join(perm.Permissions, ","), perm.GrantedBy, now.Unix()).Scan(&perm.ID)
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
	rows, err := d.db.QueryContext(ctx, `
		SELECT id,user_id,tenant_id,permissions,granted_by,granted_at
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
		if err := rows.Scan(&perm.ID, &perm.UserID, &perm.TenantID, &permissions, &grantedBy, &grantedAt); err != nil {
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
		perm.GrantedAt = time.Unix(grantedAt, 0)
		result = append(result, &perm)
	}
	return result, rows.Err()
}

func (d *DB) UpdateUserTenantPermission(ctx context.Context, perm *store.UserTenantPermission) (*store.UserTenantPermission, error) {
	now := time.Now()
	_, err := d.db.ExecContext(ctx, `
		UPDATE user_tenant_permission SET permissions=$1,granted_by=$2,granted_at=$3 WHERE id=$4
	`, strings.Join(perm.Permissions, ","), perm.GrantedBy, now.Unix(), perm.ID)
	if err != nil {
		return nil, err
	}
	perm.GrantedAt = now
	return perm, nil
}

func (d *DB) DeleteUserTenantPermission(ctx context.Context, userID, tenantID int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM user_tenant_permission WHERE user_id=$1 AND tenant_id=$2", userID, tenantID)
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
	err := d.db.QueryRowContext(ctx, `
		SELECT id,tenant_id,llm_model,simulation_human_model,reasoning_model,
			openrouter_api_key_encrypted,openrouter_api_key_nonce,features,retrieval_mode,
			content_tokens,record_transcripts,updated_at,updated_by
		FROM tenant_config WHERE `+strings.Join(where, " AND ")+` LIMIT 1
	`, args...).Scan(&config.ID, &config.TenantID, &config.LLMModel, &config.SimulationHumanModel,
		&config.ReasoningModel, &config.OpenRouterAPIKeyEncrypted, &config.OpenRouterAPIKeyNonce,
		&features, &config.RetrievalMode, &config.ContentTokens, &config.RecordTranscripts,
		&updatedAt, &config.UpdatedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(features, &config.Features)
	config.UpdatedAt = time.Unix(updatedAt, 0)
	return &config, nil
}

func (d *DB) UpsertTenantConfig(ctx context.Context, config *store.TenantConfig) (*store.TenantConfig, error) {
	features, err := json.Marshal(config.Features)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	err = d.db.QueryRowContext(ctx, `
		INSERT INTO tenant_config(tenant_id,llm_model,simulation_human_model,reasoning_model,
			openrouter_api_key_encrypted,openrouter_api_key_nonce,features,retrieval_mode,
			content_tokens,record_transcripts,updated_at,updated_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT(tenant_id) DO UPDATE SET
			llm_model=EXCLUDED.llm_model,simulation_human_model=EXCLUDED.simulation_human_model,
			reasoning_model=EXCLUDED.reasoning_model,
			openrouter_api_key_encrypted=COALESCE(EXCLUDED.openrouter_api_key_encrypted,tenant_config.openrouter_api_key_encrypted),
			openrouter_api_key_nonce=COALESCE(EXCLUDED.openrouter_api_key_nonce,tenant_config.openrouter_api_key_nonce),
			features=EXCLUDED.features,retrieval_mode=EXCLUDED.retrieval_mode,
			content_tokens=EXCLUDED.content_tokens,record_transcripts=EXCLUDED.record_transcripts,
			updated_at=EXCLUDED.updated_at,updated_by=EXCLUDED.updated_by
		RETURNING id
	`, config.TenantID, config.LLMModel, config.SimulationHumanModel, config.ReasoningModel,
		config.OpenRouterAPIKeyEncrypted, config.OpenRouterAPIKeyNonce, features, config.RetrievalMode,
		config.ContentTokens, config.RecordTranscripts, now.Unix(), config.UpdatedBy).Scan(&config.ID)
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
