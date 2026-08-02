package postgres

import (
	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/store"
)

// NewCockroachDB creates a store.Driver backed by CockroachDB.
//
// CockroachDB is a deployment profile, not a new architecture: it shares the
// entire store/db/postgres implementation (store.Driver conformance), the pgx
// driver, and the simple-protocol connection layer. Today the connect-time
// requirements are identical to PostgreSQL (DSN injection, pool sizing, ping
// guard), so this delegates to NewDB. The function exists as the explicit
// capability seam: if the Cockroach profile ever diverges at connect time
// (protocol, pool, capability probing), the divergence lives here, not in
// shared code. See bugs/057/plan6.md §1.2 (divergence seams).
func NewCockroachDB(profile *profile.Profile) (store.Driver, error) {
	return NewDB(profile)
}
