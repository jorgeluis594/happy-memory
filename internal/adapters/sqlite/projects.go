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
	var result project.Project
	err := transaction(ctx, r.db, func(tx *gorm.DB) error {
		var row projectRow
		err := tx.Where("id = ?", id).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row = projectRow{ID: id, Name: name, LastKnownGitCommonDir: path, CreatedAt: formatTime(now), UpdatedAt: formatTime(now)}
			if createErr := tx.Create(&row).Error; createErr != nil {
				return createErr
			}
		} else if err != nil {
			return err
		} else if row.Name != name || row.LastKnownGitCommonDir != path {
			if updateErr := tx.Model(&projectRow{}).Where("id = ?", id).Updates(map[string]any{"name": name, "last_known_git_common_dir": path, "updated_at": formatTime(now)}).Error; updateErr != nil {
				return updateErr
			}
			row.Name, row.LastKnownGitCommonDir, row.UpdatedAt = name, path, formatTime(now)
		}
		var convertErr error
		result, convertErr = row.project()
		return convertErr
	})
	if err != nil {
		if project.Code(err) == storeBusyCode {
			return project.Project{}, err
		}
		return project.Project{}, storeError(err)
	}
	return result, nil
}

// UpdatePath updates the last known Git common directory.
func (r *ProjectRepository) UpdatePath(ctx context.Context, id, path string, now time.Time) (project.Project, error) {
	var value project.Project
	err := transaction(ctx, r.db, func(tx *gorm.DB) error {
		result := tx.Model(&projectRow{}).Where("id = ?", id).Updates(map[string]any{"last_known_git_common_dir": path, "updated_at": formatTime(now)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return project.NewError(project.CodeProjectNotInitialized, errors.New("project is not initialized"))
		}
		var row projectRow
		if queryErr := tx.Where("id = ?", id).Take(&row).Error; queryErr != nil {
			return queryErr
		}
		var convertErr error
		value, convertErr = row.project()
		return convertErr
	})
	if err != nil {
		if project.Code(err) == project.CodeProjectNotInitialized || project.Code(err) == storeBusyCode {
			return project.Project{}, err
		}
		return project.Project{}, storeError(err)
	}
	return value, nil
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
