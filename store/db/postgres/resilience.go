package postgres

import (
	"errors"
	"log/slog"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// isTransientError checks if the error is a transient Postgres error
// that may succeed on retry.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "57P01": // admin_shutdown
		return true
	case "08006": // connection_failure
		return true
	case "08003": // connection_does_not_exist
		return true
	case "08001": // sqlclient_unable_to_establish_sqlconnection
		return true
	case "08004": // sqlserver_rejected_establishment_of_sqlconnection
		return true
	}
	return false
}

// isUniqueViolation checks if the error is a unique constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505"
}

// RunResiliently executes fn with exponential backoff retry for transient errors.
// Unique violations (23505) are treated as success.
func RunResiliently(fn func() error) error {
	maxRetries := 5
	backoff := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		// Unique violation is treated as success (idempotent insert)
		if isUniqueViolation(err) {
			slog.Warn("unique violation treated as success", "error", err)
			return nil
		}
		// Non-transient error: fail immediately
		if !isTransientError(err) {
			return err
		}
		// Transient error: retry with backoff
		if attempt == maxRetries {
			return err
		}
		jitter := time.Duration(rand.Int63n(int64(backoff) / 2))
		sleepDuration := backoff + jitter
		slog.Warn("transient error, retrying",
			"error", err,
			"attempt", attempt+1,
			"maxRetries", maxRetries,
			"sleep", sleepDuration,
		)
		time.Sleep(sleepDuration)
		backoff *= 2
	}
	return errors.New("max retries exceeded")
}
