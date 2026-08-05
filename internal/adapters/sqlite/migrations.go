package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

// EmbeddedMigrations contains all versioned schema migrations.
//
//go:embed migrations/*.sql
var EmbeddedMigrations embed.FS

// Migrate applies all pending SQLite migrations from migrations. The caller
// retains ownership of db; this helper does not close it.
func Migrate(ctx context.Context, db *sql.DB, migrations fs.FS) error {
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations)
	if err != nil {
		return fmt.Errorf("create SQLite migration provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply SQLite migrations: %w", err)
	}

	return nil
}
