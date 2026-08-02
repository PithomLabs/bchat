package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/usememos/memos/internal/version"
	storepb "github.com/usememos/memos/proto/gen/store"
)

//go:embed migration
var MigrationFS embed.FS

//go:embed seed
var seedFS embed.FS

const (
	// MigrateFileNameSplit is the split character between the patch version and the description in the migration file name.
	// For example, "1__create_table.sql".
	MigrateFileNameSplit = "__"
	// LatestSchemaFileName is the name of the latest schema file.
	// This file is used to apply the latest schema when no migration history is found.
	LatestSchemaFileName = "LATEST.sql"
)

// Migrate applies the latest schema to the database.
func (s *Store) Migrate(ctx context.Context) error {
	if err := s.preMigrate(ctx); err != nil {
		return errors.Wrap(err, "failed to pre-migrate")
	}

	// Validate version consistency (warn-only; build gate is the real enforcement)
	if err := s.validateSchemaVersionConsistency(); err != nil {
		return errors.Wrap(err, "failed to validate schema version consistency")
	}

	// Validate data integrity before migration
	// This specifically checks for orphaned ticket references before enabling foreign keys
	if s.profile.Driver == "sqlite" {
		// First ensure required columns exist (fixes databases created before these columns were added)
		if err := EnsureTicketTypeColumn(ctx, s.driver.GetDB()); err != nil {
			slog.Warn("failed to ensure ticket type column", "error", err)
			// Don't fail migration, just warn - the column might be added by migrations
		}

		if err := ValidateTicketReferences(ctx, s.driver.GetDB()); err != nil {
			return errors.Wrap(err, "data validation failed")
		}
	}

	if s.profile.Mode == "prod" || s.profile.Mode == "dev" {
		migrationHistoryList, err := s.driver.FindMigrationHistoryList(ctx, &FindMigrationHistory{})
		if err != nil {
			return errors.Wrap(err, "failed to find migration history")
		}
		if len(migrationHistoryList) == 0 {
			return errors.Errorf("no migration history found")
		}

		migrationHistoryVersions := []string{}
		for _, migrationHistory := range migrationHistoryList {
			migrationHistoryVersions = append(migrationHistoryVersions, migrationHistory.Version)
		}
		sort.Sort(version.SortVersion(migrationHistoryVersions))
		latestMigrationHistoryVersion := migrationHistoryVersions[len(migrationHistoryVersions)-1]

		schemaVersion, err := s.GetCurrentSchemaVersion()
		if err != nil {
			return errors.Wrap(err, "failed to get current schema version")
		}

		if version.IsVersionGreaterThan(schemaVersion, latestMigrationHistoryVersion) {
			filePaths, err := fs.Glob(MigrationFS, fmt.Sprintf("%s*/*.sql", s.getMigrationBasePath()))
			if err != nil {
				return errors.Wrap(err, "failed to read migration files")
			}
			sort.Strings(filePaths)

			slog.Info("start migration", slog.String("currentSchemaVersion", latestMigrationHistoryVersion), slog.String("targetSchemaVersion", schemaVersion))
			// Cockroach: DDL in explicit transactions is unsupported (online schema
			// changes run as background jobs; autocommit_before_ddl commits prior
			// statements anyway) — skip the tx entirely for cockroach. Under A1 this
			// loop never executes for cockroach (inert mirror files are ≤ history
			// version), but it must not Begin() if it ever does.
			var tx *sql.Tx
			if s.profile.Driver != "cockroach" {
				tx, err = s.driver.GetDB().Begin()
				if err != nil {
					return errors.Wrap(err, "failed to start transaction")
				}
				defer tx.Rollback()
			}
			for _, filePath := range filePaths {
				fileSchemaVersion, err := s.getSchemaVersionOfMigrateScript(filePath)
				if err != nil {
					return errors.Wrap(err, "failed to get schema version of migrate script")
				}
				// Skip migrations already applied. migration_history stores only the batch
				// target version, not every individual file version. Any file whose version
				// is <= the latest applied version was already executed — either during
				// incremental migration or when the database was first created via LATEST.sql.
				if !version.IsVersionGreaterThan(fileSchemaVersion, latestMigrationHistoryVersion) {
					continue
				}
				// Dead code after Plan 5+6: fileSchemaVersion is always <= schemaVersion (FS max).
				// Retained as defense-in-depth.
				if !version.IsVersionGreaterOrEqualThan(schemaVersion, fileSchemaVersion) {
					msg := "migration file skipped: schema version too low"
					slog.Warn(msg,
						"file", filePath,
						"file_version", fileSchemaVersion,
						"schema_version", schemaVersion,
						"latest_applied", latestMigrationHistoryVersion)
					if s.profile.Mode == "prod" && os.Getenv("MIGRATE_SKIP_ERROR") == "" {
						return errors.Errorf("%s: file=%s file_version=%s schema_version=%s",
							msg, filePath, fileSchemaVersion, schemaVersion)
					}
					continue
				}
				bytes, err := MigrationFS.ReadFile(filePath)
				if err != nil {
					return errors.Wrapf(err, "failed to read minor version migration file: %s", filePath)
				}
				stmt := string(bytes)
				if s.profile.Driver == "cockroach" {
					// SET + whole file as one statement (P0-verified, bugs/057 §4.0.1).
					// The cockroach migration files are fully idempotent (IF NOT EXISTS
					// on all DDL), so a failed boot re-runs cleanly (bugs/057 §4.4.3).
					stmt = "SET serial_normalization = 'sql_sequence';\n" + stmt
					if _, err := s.driver.GetDB().ExecContext(ctx, stmt); err != nil {
						// Tolerance inlined from execute() (see below) so the shared tx
						// path stays byte-identical. Deliberate duplication.
						errMsg := err.Error()
						if strings.Contains(errMsg, "duplicate column") ||
							strings.Contains(errMsg, "already exists") ||
							strings.Contains(errMsg, "column already exists") {
							slog.Warn("migration: column already exists, skipping", slog.String("error", errMsg))
						} else {
							return errors.Wrapf(err, "migrate error: %s", stmt)
						}
					}
				} else {
					if err := s.execute(ctx, tx, stmt); err != nil {
						return errors.Wrapf(err, "migrate error: %s", stmt)
					}
				}
			}

			if s.profile.Driver != "cockroach" {
				if err := tx.Commit(); err != nil {
					return errors.Wrap(err, "failed to commit transaction")
				}
			}
			slog.Info("end migrate")

			// Upsert the current schema version to migration_history.
			// TODO: retire using migration history later.
			if _, err = s.driver.UpsertMigrationHistory(ctx, &UpsertMigrationHistory{
				Version: schemaVersion,
			}); err != nil {
				return errors.Wrapf(err, "failed to upsert migration history with version: %s", schemaVersion)
			}
			if err := s.updateCurrentSchemaVersion(ctx, schemaVersion); err != nil {
				return errors.Wrap(err, "failed to update current schema version")
			}
		}
	} else if s.profile.Mode == "demo" {
		// In demo mode, we should seed the database.
		if err := s.seed(ctx); err != nil {
			return errors.Wrap(err, "failed to seed")
		}
	}
	return nil
}

func (s *Store) preMigrate(ctx context.Context) error {
	// TODO: using schema version in basic setting instead of migration history.
	migrationHistoryList, err := s.driver.FindMigrationHistoryList(ctx, &FindMigrationHistory{})
	// If any error occurs or no migration history found, apply the latest schema.
	if err != nil || len(migrationHistoryList) == 0 {
		if err != nil {
			slog.Warn("failed to find migration history in pre-migrate", slog.String("error", err.Error()))
		}
		filePath := s.getMigrationBasePath() + LatestSchemaFileName
		bytes, err := MigrationFS.ReadFile(filePath)
		if err != nil {
			return errors.Errorf("failed to read latest schema file: %s", err)
		}
		schemaVersion, err := s.GetCurrentSchemaVersion()
		if err != nil {
			return errors.Wrap(err, "failed to get current schema version")
		}

		if s.profile.Driver == "cockroach" {
			// Cockroach does not support DDL in explicit transactions (online schema
			// changes run as background jobs; autocommit_before_ddl commits prior
			// statements anyway), so no Begin/Commit here. The SET + whole file is
			// one statement (P0-verified, bugs/057 §4.0.1). The cockroach LATEST.sql
			// mirror is fully idempotent (IF NOT EXISTS on all DDL), so a failed
			// boot re-runs cleanly (bugs/057 §4.4.3).
			stmt := "SET serial_normalization = 'sql_sequence';\n" + string(bytes)
			if _, err := s.driver.GetDB().ExecContext(ctx, stmt); err != nil {
				// Tolerance inlined from execute() (see below) so the shared tx path
				// stays byte-identical. Deliberate duplication.
				errMsg := err.Error()
				if strings.Contains(errMsg, "duplicate column") ||
					strings.Contains(errMsg, "already exists") ||
					strings.Contains(errMsg, "column already exists") {
					slog.Warn("migration: column already exists, skipping", slog.String("error", errMsg))
				} else {
					return errors.Errorf("failed to execute SQL file %s, err %s", filePath, err)
				}
			}
		} else {
			// Start a transaction to apply the latest schema.
			tx, err := s.driver.GetDB().Begin()
			if err != nil {
				return errors.Wrap(err, "failed to start transaction")
			}
			defer tx.Rollback()
			if err := s.execute(ctx, tx, string(bytes)); err != nil {
				return errors.Errorf("failed to execute SQL file %s, err %s", filePath, err)
			}
			if err := tx.Commit(); err != nil {
				return errors.Wrap(err, "failed to commit transaction")
			}
		}

		// TODO: using schema version in basic setting instead of migration history.
		if _, err := s.driver.UpsertMigrationHistory(ctx, &UpsertMigrationHistory{
			Version: schemaVersion,
		}); err != nil {
			return errors.Wrap(err, "failed to upsert migration history")
		}
		if err := s.updateCurrentSchemaVersion(ctx, schemaVersion); err != nil {
			return errors.Wrap(err, "failed to update current schema version")
		}
	}
	if s.profile.Mode == "prod" || s.profile.Mode == "dev" {
		if err := s.normalizedMigrationHistoryList(ctx); err != nil {
			return errors.Wrap(err, "failed to normalize migration history list")
		}
	}
	return nil
}

func (s *Store) getMigrationBasePath() string {
	return fmt.Sprintf("migration/%s/", s.profile.Driver)
}

func (s *Store) getSeedBasePath() string {
	return fmt.Sprintf("seed/%s/", s.profile.Driver)
}

func (s *Store) seed(ctx context.Context) error {
	// Only seed for SQLite.
	if s.profile.Driver != "sqlite" {
		slog.Warn("seed is only supported for SQLite")
		return nil
	}

	filenames, err := fs.Glob(seedFS, fmt.Sprintf("%s*.sql", s.getSeedBasePath()))
	if err != nil {
		return errors.Wrap(err, "failed to read seed files")
	}

	// Sort seed files by name. This is important to ensure that seed files are applied in order.
	sort.Strings(filenames)
	// Start a transaction to apply the seed files.
	tx, err := s.driver.GetDB().Begin()
	if err != nil {
		return errors.Wrap(err, "failed to start transaction")
	}
	defer tx.Rollback()
	// Loop over all seed files and execute them in order.
	for _, filename := range filenames {
		bytes, err := seedFS.ReadFile(filename)
		if err != nil {
			return errors.Wrapf(err, "failed to read seed file, filename=%s", filename)
		}
		if err := s.execute(ctx, tx, string(bytes)); err != nil {
			return errors.Wrapf(err, "seed error: %s", filename)
		}
	}
	return tx.Commit()
}

// GetCurrentSchemaVersion scans ALL migration subdirectories in the embedded FS
// to find the highest version file. The filesystem is the single source of truth.
//
// NOTE: This function does NOT call itself recursively. The */*.sql glob does NOT
// match LATEST.sql (which sits at the base path, not inside a subdirectory).
// getSchemaVersionOfMigrateScript() calls GetCurrentSchemaVersion() when it
// encounters LATEST.sql, but that path is never reached from this glob.
func (s *Store) GetCurrentSchemaVersion() (string, error) {
	filePaths, err := fs.Glob(MigrationFS, fmt.Sprintf("%s*/*.sql", s.getMigrationBasePath()))
	if err != nil {
		return "", errors.Wrap(err, "failed to glob migration files")
	}
	if len(filePaths) == 0 {
		return "", errors.Errorf("no migration files found in %s", s.getMigrationBasePath())
	}
	sort.Strings(filePaths) // fs.Glob does not guarantee sorted order

	var maxVersion string
	for _, filePath := range filePaths {
		fileVer, err := s.getSchemaVersionOfMigrateScript(filePath)
		if err != nil {
			continue // skip files that can't be parsed (e.g., LATEST.sql at base path)
		}
		if maxVersion == "" || version.IsVersionGreaterThan(fileVer, maxVersion) {
			maxVersion = fileVer
		}
	}
	if maxVersion == "" {
		return "", errors.Errorf("could not determine schema version from migration files")
	}
	return maxVersion, nil
}

func (s *Store) validateSchemaVersionConsistency() error {
	fsVersion, err := s.GetCurrentSchemaVersion()
	if err != nil {
		return errors.Wrap(err, "failed to get FS schema version")
	}
	codeVersion := version.GetCurrentVersion(s.profile.Mode)
	codeMinor := version.GetMinorVersion(codeVersion)
	fsMinor := version.GetMinorVersion(fsVersion)

	if version.IsVersionGreaterThan(fsMinor, codeMinor) {
		slog.Warn("migration FS has directories newer than code version; bump Version/DevVersion",
			"fs_minor", fsMinor, "code_minor", codeMinor,
			"fs_version", fsVersion, "code_version", codeVersion)
	}
	return nil // warn-only at runtime; build-time gate catches this earlier
}

func (s *Store) getSchemaVersionOfMigrateScript(filePath string) (string, error) {
	// If the file is the latest schema file, return the current schema version.
	if strings.HasSuffix(filePath, LatestSchemaFileName) {
		return s.GetCurrentSchemaVersion()
	}

	normalizedPath := filepath.ToSlash(filePath)
	elements := strings.Split(normalizedPath, "/")
	if len(elements) < 2 {
		return "", errors.Errorf("invalid file path: %s", filePath)
	}
	minorVersion := elements[len(elements)-2]
	rawPatchVersion := strings.Split(elements[len(elements)-1], MigrateFileNameSplit)[0]
	patchVersion, err := strconv.Atoi(rawPatchVersion)
	if err != nil {
		return "", errors.Wrapf(err, "failed to convert patch version to int: %s", rawPatchVersion)
	}
	return fmt.Sprintf("%s.%d", minorVersion, patchVersion+1), nil
}

// execute runs a single SQL statement within a transaction.
func (*Store) execute(ctx context.Context, tx *sql.Tx, stmt string) error {
	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		// Tolerate "duplicate column" errors for ALTER TABLE ADD COLUMN.
		// This makes migrations idempotent if re-run (e.g., corrupted history).
		errMsg := err.Error()
		if strings.Contains(errMsg, "duplicate column") ||
			strings.Contains(errMsg, "already exists") ||
			strings.Contains(errMsg, "column already exists") {
			slog.Warn("migration: column already exists, skipping", slog.String("error", errMsg))
			return nil
		}
		return errors.Wrap(err, "failed to execute statement")
	}
	return nil
}

func (s *Store) normalizedMigrationHistoryList(ctx context.Context) error {
	migrationHistoryList, err := s.driver.FindMigrationHistoryList(ctx, &FindMigrationHistory{})
	if err != nil {
		return errors.Wrap(err, "failed to find migration history")
	}
	versions := []string{}
	for _, migrationHistory := range migrationHistoryList {
		versions = append(versions, migrationHistory.Version)
	}
	sort.Sort(version.SortVersion(versions))
	latestVersion := versions[len(versions)-1]
	latestMinorVersion := version.GetMinorVersion(latestVersion)

	// If the latest version is greater than 0.22, return.
	// As of 0.22, the migration history is already normalized.
	if version.IsVersionGreaterThan(latestMinorVersion, "0.22") {
		return nil
	}

	schemaVersionMap := map[string]string{}
	filePaths, err := fs.Glob(MigrationFS, fmt.Sprintf("%s*/*.sql", s.getMigrationBasePath()))
	if err != nil {
		return errors.Wrap(err, "failed to read migration files")
	}
	sort.Strings(filePaths)
	for _, filePath := range filePaths {
		fileSchemaVersion, err := s.getSchemaVersionOfMigrateScript(filePath)
		if err != nil {
			return errors.Wrap(err, "failed to get schema version of migrate script")
		}
		schemaVersionMap[version.GetMinorVersion(fileSchemaVersion)] = fileSchemaVersion
	}

	latestSchemaVersion := schemaVersionMap[latestMinorVersion]
	if latestSchemaVersion == "" {
		return errors.Errorf("latest schema version not found")
	}
	if version.IsVersionGreaterOrEqualThan(latestVersion, latestSchemaVersion) {
		return nil
	}

	// Start a transaction to insert the latest schema version to migration_history.
	tx, err := s.driver.GetDB().Begin()
	if err != nil {
		return errors.Wrap(err, "failed to start transaction")
	}
	defer tx.Rollback()
	if err := s.execute(ctx, tx, fmt.Sprintf("INSERT INTO migration_history (version) VALUES ('%s')", latestSchemaVersion)); err != nil {
		return errors.Wrap(err, "failed to insert migration history")
	}
	return tx.Commit()
}

func (s *Store) updateCurrentSchemaVersion(ctx context.Context, schemaVersion string) error {
	workspaceBasicSetting, err := s.GetWorkspaceBasicSetting(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get workspace basic setting")
	}
	workspaceBasicSetting.SchemaVersion = schemaVersion
	if _, err := s.UpsertWorkspaceSetting(ctx, &storepb.WorkspaceSetting{
		Key:   storepb.WorkspaceSettingKey_BASIC,
		Value: &storepb.WorkspaceSetting_BasicSetting{BasicSetting: workspaceBasicSetting},
	}); err != nil {
		return errors.Wrap(err, "failed to upsert workspace setting")
	}
	return nil
}
