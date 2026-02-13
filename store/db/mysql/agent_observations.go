package mysql

import (
	"context"
	"fmt"

	"github.com/usememos/memos/store"
)

func (d *DB) UpsertObservationLog(ctx context.Context, log *store.ObservationLog) (*store.ObservationLog, error) {
	return nil, fmt.Errorf("UpsertObservationLog not implemented for MySQL")
}

func (d *DB) GetObservationLog(ctx context.Context, sessionID string) (*store.ObservationLog, error) {
	return nil, fmt.Errorf("GetObservationLog not implemented for MySQL")
}

func (d *DB) GetObservationLogByResource(ctx context.Context, resourceID string) (*store.ObservationLog, error) {
	return nil, fmt.Errorf("GetObservationLogByResource not implemented for MySQL")
}
