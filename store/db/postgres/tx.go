package postgres

import (
	"context"
	"database/sql"

	"github.com/cockroachdb/cockroach-go/v2/crdb"
)

// execTx runs fn inside a single database transaction.
//
// Postgres: one BeginTx attempt; the caller's retry loop (where present)
// handles "deadlock"/"could not serialize" via isPostgresRetryable.
//
// Cockroach: crdb.ExecuteTx (SAVEPOINT cockroach_restart protocol) retries
// serialization failures (SQLSTATE 40001) internally with backoff. fn must
// NOT commit or roll back mid-body — execTx owns commit/rollback — otherwise
// the savepoint protocol breaks (RELEASE on an already-committed tx).
//
// Every call site carries a `// retry-safe: ...` comment (bugs/057 §4.5)
// explaining why a 40001 retry cannot double-write.
func (d *DB) execTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	if d.profile.Driver == "cockroach" {
		return crdb.ExecuteTx(ctx, d.db, nil, fn)
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
