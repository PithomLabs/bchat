package mysql

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

func (d *DB) UpsertUserSetting(ctx context.Context, upsert *store.UserSetting) (*store.UserSetting, error) {
	stmt := "INSERT INTO `user_setting` (`user_id`, `key`, `value`) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE `value` = ?"
	if _, err := d.db.ExecContext(ctx, stmt, upsert.UserID, upsert.Key.String(), upsert.Value, upsert.Value); err != nil {
		return nil, err
	}
	return upsert, nil
}

func (d *DB) ListUserSettings(ctx context.Context, find *store.FindUserSetting) ([]*store.UserSetting, error) {
	where, args := []string{"1 = 1"}, []any{}

	if v := find.Key; v != storepb.UserSettingKey_USER_SETTING_KEY_UNSPECIFIED {
		where, args = append(where, "`key` = ?"), append(args, v.String())
	}
	if v := find.UserID; v != nil {
		where, args = append(where, "`user_id` = ?"), append(args, *find.UserID)
	}

	query := "SELECT `user_id`, `key`, `value` FROM `user_setting` WHERE " + strings.Join(where, " AND ")
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	userSettingList := make([]*store.UserSetting, 0)
	for rows.Next() {
		userSetting := &store.UserSetting{}
		var keyString string
		if err := rows.Scan(
			&userSetting.UserID,
			&keyString,
			&userSetting.Value,
		); err != nil {
			return nil, err
		}
		userSetting.Key = storepb.UserSettingKey(storepb.UserSettingKey_value[keyString])
		userSettingList = append(userSettingList, userSetting)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return userSettingList, nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (d *DB) InsertUserAccessTokenLookup(ctx context.Context, userID int32, accessToken, description string) error {
	stmt := "INSERT INTO `user_access_token_lookup` (`token_hash`, `user_id`, `description`) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE `user_id` = VALUES(`user_id`), `description` = VALUES(`description`)"
	_, err := d.db.ExecContext(ctx, stmt, hashToken(accessToken), userID, description)
	return err
}

func (d *DB) DeleteUserAccessTokenLookup(ctx context.Context, accessToken string) error {
	stmt := "DELETE FROM `user_access_token_lookup` WHERE `token_hash` = ?"
	_, err := d.db.ExecContext(ctx, stmt, hashToken(accessToken))
	return err
}

func (d *DB) FindUserByAccessToken(ctx context.Context, accessToken string) (*store.User, string, error) {
	stmt := `
		SELECT u.id, u.created_ts, u.updated_ts, u.row_status, u.username, u.role, u.email,
		       u.nickname, u.password_hash, u.avatar_url, u.description, u.allowed_tenant_ids,
		       l.description
		FROM user_access_token_lookup l
		JOIN user u ON u.id = l.user_id
		WHERE l.token_hash = ?
	`
	row := d.db.QueryRowContext(ctx, stmt, hashToken(accessToken))
	user := &store.User{}
	var description string
	if err := row.Scan(
		&user.ID, &user.CreatedTs, &user.UpdatedTs, &user.RowStatus, &user.Username,
		&user.Role, &user.Email, &user.Nickname, &user.PasswordHash, &user.AvatarURL,
		&user.Description, &user.AllowedTenantIDs, &description,
	); err != nil {
		return nil, "", err
	}
	return user, description, nil
}
