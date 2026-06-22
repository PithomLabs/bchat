package mysql

import (
	"context"
	"time"

	"github.com/usememos/memos/store"
)

func (d *DB) EnsureBridgeExternalSession(context.Context, int32, string, time.Time, time.Time) (*store.BridgeExternalSession, bool, error) {
	return nil, false, store.ErrBridgeUnsupportedDatabase
}

func (d *DB) FindBridgeExternalSession(context.Context, int32, string) (*store.BridgeExternalSession, error) {
	return nil, store.ErrBridgeUnsupportedDatabase
}

func (d *DB) TouchBridgeExternalSession(context.Context, int32, string, time.Time, time.Time) error {
	return store.ErrBridgeUnsupportedDatabase
}

func (d *DB) CreateBridgeHandoff(context.Context, int32, string, time.Time) (*store.BridgeHandoff, error) {
	return nil, store.ErrBridgeUnsupportedDatabase
}

func (d *DB) FindActiveBridgeHandoff(context.Context, int32, string) (*store.BridgeHandoff, error) {
	return nil, store.ErrBridgeUnsupportedDatabase
}

func (d *DB) UpdateBridgeHandoffRoutingModeCAS(context.Context, int32, string, int, string, int, store.BridgeRoutingMode, store.BridgeRoutingMode, string, time.Time) (*store.BridgeHandoff, error) {
	return nil, store.ErrBridgeUnsupportedDatabase
}

func (d *DB) GetBridgeHandoff(context.Context, int32, string, string) (*store.BridgeHandoff, error) {
	return nil, store.ErrBridgeUnsupportedDatabase
}

func (d *DB) CreateBridgeHandoffReplyIfActive(context.Context, *store.CreateBridgeHandoffReply) (*store.BridgeHandoffReply, error) {
	return nil, store.ErrBridgeUnsupportedDatabase
}

func (d *DB) GetBridgeHandoffReplyByClientMessageID(context.Context, int32, string, string, string) (*store.BridgeHandoffReply, error) {
	return nil, store.ErrBridgeUnsupportedDatabase
}
