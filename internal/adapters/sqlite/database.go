// Package sqlite provides the application's SQLite storage adapter.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"

	glebarezsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	applicationDirectory = "happy-memory"
	databaseFilename     = "happy-memory.db"
)

// Database owns the single database/sql connection pool used by both direct
// SQL queries and GORM.
type Database struct {
	path string
	sql  *sql.DB
	gorm *gorm.DB
}

// Open creates the application's private data directory and database file when
// necessary, then opens the database with the required SQLite settings.
func Open(ctx context.Context) (*Database, error) {
	path, err := databasePath(runtime.GOOS, os.Getenv, os.UserHomeDir)
	if err != nil {
		return nil, fmt.Errorf("resolve SQLite database path: %w", err)
	}

	return open(ctx, path)
}

func open(ctx context.Context, path string) (*Database, error) {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create SQLite data directory %q: %w", directory, err)
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("create SQLite database file %q: %w", path, err)
	}
	if err == nil {
		if closeErr := file.Close(); closeErr != nil {
			return nil, fmt.Errorf("close newly created SQLite database file %q: %w", path, closeErr)
		}
	}

	sqlDB, err := sql.Open(glebarezsqlite.DriverName, dataSourceName(path))
	if err != nil {
		return nil, fmt.Errorf("open SQLite database %q: %w", path, err)
	}
	// SQLite pragmas are connection-local. A single connection also guarantees
	// that database/sql and GORM observe the same configured connection.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	pingErr := sqlDB.PingContext(ctx)
	if pingErr != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("connect to SQLite database %q: %w", path, pingErr)
	}

	gormDB, err := gorm.Open(&glebarezsqlite.Dialector{Conn: sqlDB}, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("initialize GORM for SQLite database %q: %w", path, err)
	}

	return &Database{path: path, sql: sqlDB, gorm: gormDB}, nil
}

// Path returns the absolute database file path.
func (database *Database) Path() string {
	return database.path
}

// SQL returns the shared database/sql connection pool.
func (database *Database) SQL() *sql.DB {
	return database.sql
}

// GORM returns the GORM handle backed by the shared database/sql pool.
func (database *Database) GORM() *gorm.DB {
	return database.gorm
}

// Close releases the database connection.
func (database *Database) Close() error {
	if err := database.sql.Close(); err != nil {
		return fmt.Errorf("close SQLite database %q: %w", database.path, err)
	}

	return nil
}

func dataSourceName(path string) string {
	query := make(url.Values)
	query.Add("_pragma", "foreign_keys(ON)")
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", "busy_timeout(5000)")

	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path), RawQuery: query.Encode()}).String()
}
