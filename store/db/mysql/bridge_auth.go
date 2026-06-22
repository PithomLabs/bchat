package mysql

import (
	"context"
	"time"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateBridgeAuthKey(ctx context.Context, key *store.BridgeAuthKey) (*store.BridgeAuthKey, error) {
	return nil, store.ErrBridgeUnsupportedDatabase
}

func (d *DB) GetBridgeAuthKey(ctx context.Context, tenantID int32, keyID string) (*store.BridgeAuthKey, error) {
	return nil, store.ErrBridgeUnsupportedDatabase
}

func (d *DB) ListBridgeAuthKeys(ctx context.Context, tenantID int32) ([]*store.BridgeAuthKey, error) {
	return nil, store.ErrBridgeUnsupportedDatabase
}

func (d *DB) UpdateBridgeAuthKeyLastUsed(ctx context.Context, tenantID int32, keyID string, now time.Time) error {
	return store.ErrBridgeUnsupportedDatabase
}

func (d *DB) RevokeBridgeAuthKey(ctx context.Context, tenantID int32, keyID string, now time.Time) error {
	return store.ErrBridgeUnsupportedDatabase
}

func (d *DB) StoreBridgeAuthNonce(ctx context.Context, nonce *store.BridgeAuthNonce) error {
	return store.ErrBridgeUnsupportedDatabase
}

func (d *DB) CleanupBridgeAuthNonces(ctx context.Context, now time.Time) error {
	return store.ErrBridgeUnsupportedDatabase
}
