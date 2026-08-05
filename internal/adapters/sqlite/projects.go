package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jorgeluis594/happy-memory/internal/project"
)

// ProjectRepository stores projects in SQLite.
type ProjectRepository struct{ db *sql.DB }

// NewProjectRepository creates a repository over db.
func NewProjectRepository(db *sql.DB) *ProjectRepository { return &ProjectRepository{db: db} }

// Find retrieves a project by ID or nil when absent.
func (repository *ProjectRepository) Find(ctx context.Context, id string) (*project.Project, error) {
	row := repository.db.QueryRowContext(ctx, `SELECT id, name, last_known_git_common_dir, created_at, updated_at FROM projects WHERE id = ?`, id)
	value, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, storeError(err)
	}
	return &value, nil
}

// Reconcile creates or updates a project only when values changed.
func (repository *ProjectRepository) Reconcile(ctx context.Context, id, name, path string, now time.Time) (project.Project, error) {
	existing, err := repository.Find(ctx, id)
	if err != nil {
		return project.Project{}, err
	}
	if existing == nil {
		stamp := formatTime(now)
		_, err = repository.db.ExecContext(ctx, `INSERT INTO projects (id, name, last_known_git_common_dir, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, id, name, path, stamp, stamp)
		if err != nil {
			return project.Project{}, storeError(err)
		}
		return project.Project{ID: id, Name: name, LastKnownGitCommonDir: path, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
	}
	if existing.Name == name && existing.LastKnownGitCommonDir == path {
		return *existing, nil
	}
	_, err = repository.db.ExecContext(ctx, `UPDATE projects SET name = ?, last_known_git_common_dir = ?, updated_at = ? WHERE id = ?`, name, path, formatTime(now), id)
	if err != nil {
		return project.Project{}, storeError(err)
	}
	existing.Name, existing.LastKnownGitCommonDir, existing.UpdatedAt = name, path, now.UTC()
	return *existing, nil
}

// UpdatePath updates the last known Git common directory.
func (repository *ProjectRepository) UpdatePath(ctx context.Context, id, path string, now time.Time) (project.Project, error) {
	_, err := repository.db.ExecContext(ctx, `UPDATE projects SET last_known_git_common_dir = ?, updated_at = ? WHERE id = ?`, path, formatTime(now), id)
	if err != nil {
		return project.Project{}, storeError(err)
	}
	value, err := repository.Find(ctx, id)
	if err != nil {
		return project.Project{}, err
	}
	if value == nil {
		return project.Project{}, project.NewError(project.CodeProjectNotInitialized, errors.New("project is not initialized"))
	}
	return *value, nil
}

// List returns projects ordered by name and ID.
func (repository *ProjectRepository) List(ctx context.Context) ([]project.Project, error) {
	rows, err := repository.db.QueryContext(ctx, `SELECT id, name, last_known_git_common_dir, created_at, updated_at FROM projects ORDER BY name, id`)
	if err != nil {
		return nil, storeError(err)
	}
	defer func() { _ = rows.Close() }()
	projects := make([]project.Project, 0)
	for rows.Next() {
		value, scanErr := scanProject(rows)
		if scanErr != nil {
			return nil, storeError(scanErr)
		}
		projects = append(projects, value)
	}
	if err := rows.Err(); err != nil {
		return nil, storeError(err)
	}
	return projects, nil
}

type scanner interface{ Scan(...any) error }

func scanProject(source scanner) (project.Project, error) {
	var value project.Project
	var created, updated string
	if err := source.Scan(&value.ID, &value.Name, &value.LastKnownGitCommonDir, &created, &updated); err != nil {
		return project.Project{}, err
	}
	var err error
	value.CreatedAt, err = time.Parse(time.RFC3339, created)
	if err != nil {
		return project.Project{}, fmt.Errorf("parse created_at: %w", err)
	}
	value.UpdatedAt, err = time.Parse(time.RFC3339, updated)
	if err != nil {
		return project.Project{}, fmt.Errorf("parse updated_at: %w", err)
	}
	return value, nil
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339) }

func storeError(err error) error {
	return project.NewError(project.CodeStoreError, fmt.Errorf("storage operation failed: %w", err))
}
