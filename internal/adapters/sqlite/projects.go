package sqlite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jorgeluis594/happy-memory/internal/project"
	"gorm.io/gorm"
)

type projectRow struct {
	ID, Name, LastKnownGitCommonDir, CreatedAt, UpdatedAt string
}

func (projectRow) TableName() string { return "projects" }

// ProjectRepository stores projects through GORM.
type ProjectRepository struct{ db *gorm.DB }

// NewProjectRepository creates a project repository over GORM.
func NewProjectRepository(db *gorm.DB) *ProjectRepository { return &ProjectRepository{db: db} }

// Find retrieves a project by ID or nil when absent.
func (r *ProjectRepository) Find(ctx context.Context, id string) (*project.Project, error) {
	var row projectRow
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, storeError(err)
	}
	value, err := row.project()
	if err != nil {
		return nil, storeError(err)
	}
	return &value, nil
}

// Reconcile creates or updates a project only when values changed.
func (r *ProjectRepository) Reconcile(ctx context.Context, id, name, path string, now time.Time) (project.Project, error) {
	existing, err := r.Find(ctx, id)
	if err != nil {
		return project.Project{}, err
	}
	if existing == nil {
		row := projectRow{ID: id, Name: name, LastKnownGitCommonDir: path, CreatedAt: formatTime(now), UpdatedAt: formatTime(now)}
		if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
			return project.Project{}, storeError(err)
		}
		return row.project()
	}
	if existing.Name == name && existing.LastKnownGitCommonDir == path {
		return *existing, nil
	}
	if err := r.db.WithContext(ctx).Model(&projectRow{}).Where("id = ?", id).Updates(map[string]any{"name": name, "last_known_git_common_dir": path, "updated_at": formatTime(now)}).Error; err != nil {
		return project.Project{}, storeError(err)
	}
	existing.Name = name
	existing.LastKnownGitCommonDir = path
	existing.UpdatedAt = now.UTC()
	return *existing, nil
}

// UpdatePath updates the last known Git common directory.
func (r *ProjectRepository) UpdatePath(ctx context.Context, id, path string, now time.Time) (project.Project, error) {
	result := r.db.WithContext(ctx).Model(&projectRow{}).Where("id = ?", id).Updates(map[string]any{"last_known_git_common_dir": path, "updated_at": formatTime(now)})
	if result.Error != nil {
		return project.Project{}, storeError(result.Error)
	}
	if result.RowsAffected == 0 {
		return project.Project{}, project.NewError(project.CodeProjectNotInitialized, errors.New("project is not initialized"))
	}
	value, err := r.Find(ctx, id)
	if err != nil {
		return project.Project{}, err
	}
	return *value, nil
}

// List returns projects ordered by name and ID.
func (r *ProjectRepository) List(ctx context.Context) ([]project.Project, error) {
	var rows []projectRow
	if err := r.db.WithContext(ctx).Order("name ASC").Order("id ASC").Find(&rows).Error; err != nil {
		return nil, storeError(err)
	}
	values := make([]project.Project, 0, len(rows))
	for _, row := range rows {
		value, err := row.project()
		if err != nil {
			return nil, storeError(err)
		}
		values = append(values, value)
	}
	return values, nil
}

func (row projectRow) project() (project.Project, error) {
	created, err := time.Parse(time.RFC3339, row.CreatedAt)
	if err != nil {
		return project.Project{}, fmt.Errorf("parse created_at: %w", err)
	}
	updated, err := time.Parse(time.RFC3339, row.UpdatedAt)
	if err != nil {
		return project.Project{}, fmt.Errorf("parse updated_at: %w", err)
	}
	return project.Project{ID: row.ID, Name: row.Name, LastKnownGitCommonDir: row.LastKnownGitCommonDir, CreatedAt: created, UpdatedAt: updated}, nil
}
func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339) }
func storeError(err error) error {
	return project.NewError(project.CodeStoreError, fmt.Errorf("storage operation failed: %w", err))
}
