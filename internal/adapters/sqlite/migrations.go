package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pressly/goose/v3"
)

// EmbeddedMigrations contains all versioned schema migrations.
//
//go:embed migrations/*.sql
var EmbeddedMigrations embed.FS

// ValidateExistingSchema checks an existing database in read-only mode before
// normal opening can enable WAL. A missing database is valid and may be created.
func ValidateExistingSchema(ctx context.Context, path string, migrations fs.FS) error {
	database, err := OpenReadOnly(ctx, path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open existing database for schema validation: %w", err)
	}
	defer func() { _ = database.Close() }()
	return ValidateSchema(ctx, database.SQL(), migrations)
}

// Migrate applies all pending SQLite migrations from migrations. The caller
// retains ownership of db; this helper does not close it.
func Migrate(ctx context.Context, db *sql.DB, migrations fs.FS) error {
	if err := ValidateSchema(ctx, db, migrations); err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations)
	if err != nil {
		return fmt.Errorf("create SQLite migration provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply SQLite migrations: %w", err)
	}

	return nil
}

// ValidateSchema verifies that applied Goose versions form a known prefix.
// It never creates metadata or applies migrations.
func ValidateSchema(ctx context.Context, db *sql.DB, migrations fs.FS) error {
	known, err := migrationVersions(migrations)
	if err != nil {
		return fmt.Errorf("inspect embedded migrations: %w", err)
	}
	var exists int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema WHERE type='table' AND name='goose_db_version'`).Scan(&exists); err != nil {
		return fmt.Errorf("inspect migration metadata: %w", err)
	}
	if exists == 0 {
		return nil
	}
	rows, err := db.QueryContext(ctx, `SELECT id, version_id, is_applied FROM goose_db_version ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read migration metadata: %w", err)
	}
	defer func() { _ = rows.Close() }()
	latest := make(map[int64]bool)
	for rows.Next() {
		var id, version int64
		var applied bool
		if scanErr := rows.Scan(&id, &version, &applied); scanErr != nil {
			return fmt.Errorf("read migration metadata: %w", scanErr)
		}
		if id < 0 || version < 0 {
			return errors.New("invalid migration metadata")
		}
		if version != 0 {
			latest[version] = applied
		}
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("read migration metadata: %w", err)
	}
	applied := make([]int64, 0, len(latest))
	for version, active := range latest {
		if active {
			applied = append(applied, version)
		}
	}
	sort.Slice(applied, func(i, j int) bool { return applied[i] < applied[j] })
	if len(applied) > len(known) {
		return errors.New("unknown migration version")
	}
	for i, version := range applied {
		if version != known[i] {
			return errors.New("migration history is not a known prefix")
		}
	}
	return nil
}

func migrationVersions(migrations fs.FS) ([]int64, error) {
	entries, err := fs.ReadDir(migrations, ".")
	if err != nil {
		return nil, err
	}
	versions := make([]int64, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		version, parseErr := strconv.ParseInt(prefix, 10, 64)
		if parseErr != nil || version <= 0 {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	for i := 1; i < len(versions); i++ {
		if versions[i] == versions[i-1] {
			return nil, fmt.Errorf("duplicate migration version %d", versions[i])
		}
	}
	return versions, nil
}
