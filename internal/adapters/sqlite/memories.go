package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jorgeluis594/happy-memory/internal/memory"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type memoryRow struct {
	ID                                string
	ProjectID                         string
	CurrentVersion                    int
	Type, Title, Content              string
	Importance, Confidence            int
	AttributesJSON                    *string
	ContentHash, CreatedAt, UpdatedAt string
	DeletedAt                         *string
}

func (memoryRow) TableName() string { return "memories" }

type tagRow struct {
	ID, ProjectID, Name, NormalizedName string
	Description                         *string
	CreatedAt, UpdatedAt                string
}

func (tagRow) TableName() string { return "tags" }

type memoryTagRow struct{ ProjectID, MemoryID, TagID, CreatedAt string }

func (memoryTagRow) TableName() string { return "memory_tags" }

type revisionRow struct {
	MemoryID, ProjectID                string
	Version                            int
	Operation, SnapshotJSON            string
	AgentName                          *string
	AgentRole, WorktreeRoot, CreatedAt string
}

func (revisionRow) TableName() string { return "memory_revisions" }

// MemoryRepository stores memories and their initial audit data atomically through GORM.
type MemoryRepository struct{ db *gorm.DB }

// NewMemoryRepository creates a memory repository over GORM.
func NewMemoryRepository(db *gorm.DB) *MemoryRepository { return &MemoryRepository{db: db} }

// Create writes a memory and all initial related records atomically.
func (r *MemoryRepository) Create(ctx context.Context, record memory.CreateRecord) (memory.Memory, error) {
	m := record.Memory
	err := transaction(ctx, r.db, func(tx *gorm.DB) error {
		row := toMemoryRow(m)
		if err := tx.Create(&row).Error; err != nil {
			var existing memoryRow
			lookup := tx.Where("project_id = ? AND content_hash = ? AND deleted_at IS NULL", m.ProjectID, m.ContentHash).Take(&existing)
			if lookup.Error == nil {
				return &memory.Error{Code: memory.CodeDuplicate, Details: map[string]any{"memory_id": existing.ID}, Err: errors.New("duplicate memory")}
			}
			return memoryStoreError(err)
		}
		for i := range m.Tags {
			candidate := toTagRow(m.ProjectID, m.Tags[i])
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "project_id"}, {Name: "normalized_name"}}, DoNothing: true}).Create(&candidate).Error; err != nil {
				return memoryStoreError(err)
			}
			var canonical tagRow
			if err := tx.Where("project_id = ? AND normalized_name = ?", m.ProjectID, m.Tags[i].NormalizedName).Take(&canonical).Error; err != nil {
				return memoryStoreError(err)
			}
			m.Tags[i] = canonical.tag()
			if err := tx.Create(&memoryTagRow{ProjectID: m.ProjectID, MemoryID: m.ID, TagID: canonical.ID, CreatedAt: formatTime(m.CreatedAt)}).Error; err != nil {
				return memoryStoreError(err)
			}
		}
		rev := record.Revision
		snapshot, snapshotErr := memory.SnapshotJSON(m, rev.AgentName, rev.AgentRole, rev.WorktreeRoot)
		if snapshotErr != nil {
			return memoryStoreError(snapshotErr)
		}
		rev.Snapshot = snapshot
		if err := tx.Create(&revisionRow{MemoryID: rev.MemoryID, ProjectID: rev.ProjectID, Version: rev.Version, Operation: rev.Operation, SnapshotJSON: string(rev.Snapshot), AgentName: rev.AgentName, AgentRole: rev.AgentRole, WorktreeRoot: rev.WorktreeRoot, CreatedAt: formatTime(rev.CreatedAt)}).Error; err != nil {
			return memoryStoreError(err)
		}
		if err := tx.Exec(`INSERT INTO memory_fts (memory_id,project_id,title,content) VALUES (?,?,?,?)`, m.ID, m.ProjectID, m.Title, m.Content).Error; err != nil {
			return memoryStoreError(err)
		}
		return nil
	})
	if err != nil {
		return memory.Memory{}, err
	}
	return m, nil
}

// Get retrieves one project-scoped memory, optionally including a deleted one.
func (r *MemoryRepository) Get(ctx context.Context, projectID, id string, includeDeleted bool) (memory.Memory, error) {
	var row memoryRow
	query := r.db.WithContext(ctx).Where("project_id = ? AND id = ?", projectID, id)
	if !includeDeleted {
		query = query.Where("deleted_at IS NULL")
	}
	err := query.Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return memory.Memory{}, memory.NewError(memory.CodeNotFound, errors.New("memory not found"))
	}
	if err != nil {
		return memory.Memory{}, memoryStoreError(err)
	}
	value, err := row.memory()
	if err != nil {
		return memory.Memory{}, memoryStoreError(err)
	}
	tags, err := r.loadTags(ctx, projectID, []string{id})
	if err != nil {
		return memory.Memory{}, err
	}
	value.Tags = tags[id]
	if value.Tags == nil {
		value.Tags = []memory.Tag{}
	}
	return value, nil
}

// List retrieves filtered active project-scoped memories.
func (r *MemoryRepository) List(ctx context.Context, projectID string, f memory.ListFilter) ([]memory.Memory, error) {
	query := r.db.WithContext(ctx).Where("project_id = ?", projectID)
	if !f.IncludeDeleted {
		query = query.Where("deleted_at IS NULL")
	}
	if f.Type != "" {
		query = query.Where("type = ?", f.Type)
	}
	if f.MinImportance > 0 {
		query = query.Where("importance >= ?", f.MinImportance)
	}
	if f.MinConfidence > 0 {
		query = query.Where("confidence >= ?", f.MinConfidence)
	}
	for _, tag := range f.Tags {
		sub := r.db.Table("memory_tags AS mt").Select("1").Joins("JOIN tags AS t ON t.project_id = mt.project_id AND t.id = mt.tag_id").Where("mt.project_id = memories.project_id AND mt.memory_id = memories.id AND t.normalized_name = ?", tag)
		query = query.Where("EXISTS (?)", sub)
	}
	var rows []memoryRow
	if err := query.Order("updated_at DESC").Order("id ASC").Find(&rows).Error; err != nil {
		return nil, memoryStoreError(err)
	}
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	tags, err := r.loadTags(ctx, projectID, ids)
	if err != nil {
		return nil, err
	}
	values := make([]memory.Memory, 0, len(rows))
	for _, row := range rows {
		value, convertErr := row.memory()
		if convertErr != nil {
			return nil, memoryStoreError(convertErr)
		}
		value.Tags = tags[value.ID]
		if value.Tags == nil {
			value.Tags = []memory.Tag{}
		}
		values = append(values, value)
	}
	return values, nil
}

// Mutate atomically applies a versioned state change, tags, revision, and FTS state.
func (r *MemoryRepository) Mutate(ctx context.Context, record memory.MutationRecord) (memory.Memory, error) {
	m := record.Memory
	err := transaction(ctx, r.db, func(tx *gorm.DB) error {
		row := toMemoryRow(m)
		result := tx.Model(&memoryRow{}).Where("project_id = ? AND id = ? AND current_version = ?", m.ProjectID, m.ID, record.ExpectedVersion).Updates(map[string]any{
			"current_version": row.CurrentVersion, "type": row.Type, "title": row.Title, "content": row.Content,
			"importance": row.Importance, "confidence": row.Confidence, "attributes_json": row.AttributesJSON,
			"content_hash": row.ContentHash, "updated_at": row.UpdatedAt, "deleted_at": nullableTime(m.DeletedAt),
		})
		if result.Error != nil {
			if duplicateID, ok := activeDuplicate(tx, m.ProjectID, m.ContentHash, m.ID); ok {
				return &memory.Error{Code: memory.CodeDuplicate, Details: map[string]any{"memory_id": duplicateID}, Err: errors.New("duplicate memory")}
			}
			return memoryStoreError(result.Error)
		}
		if result.RowsAffected != 1 {
			var current memoryRow
			if err := tx.Where("project_id = ? AND id = ?", m.ProjectID, m.ID).Take(&current).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return memory.NewError(memory.CodeNotFound, errors.New("memory not found"))
			} else if err != nil {
				return memoryStoreError(err)
			}
			return &memory.Error{Code: memory.CodeVersionConflict, Details: map[string]any{"memory_id": m.ID, "current_version": current.CurrentVersion}, Err: errors.New("version conflict")}
		}
		if err := tx.Where("project_id = ? AND memory_id = ?", m.ProjectID, m.ID).Delete(&memoryTagRow{}).Error; err != nil {
			return memoryStoreError(err)
		}
		for i := range m.Tags {
			candidate := toTagRow(m.ProjectID, m.Tags[i])
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "project_id"}, {Name: "normalized_name"}}, DoNothing: true}).Create(&candidate).Error; err != nil {
				return memoryStoreError(err)
			}
			var canonical tagRow
			if err := tx.Where("project_id = ? AND normalized_name = ?", m.ProjectID, m.Tags[i].NormalizedName).Take(&canonical).Error; err != nil {
				return memoryStoreError(err)
			}
			m.Tags[i] = canonical.tag()
			if err := tx.Create(&memoryTagRow{ProjectID: m.ProjectID, MemoryID: m.ID, TagID: canonical.ID, CreatedAt: formatTime(m.UpdatedAt)}).Error; err != nil {
				return memoryStoreError(err)
			}
		}
		rev := record.Revision
		snapshot, err := memory.SnapshotJSON(m, rev.AgentName, rev.AgentRole, rev.WorktreeRoot)
		if err != nil {
			return memoryStoreError(err)
		}
		if err := tx.Create(&revisionRow{MemoryID: m.ID, ProjectID: m.ProjectID, Version: m.Version, Operation: rev.Operation, SnapshotJSON: string(snapshot), AgentName: rev.AgentName, AgentRole: rev.AgentRole, WorktreeRoot: rev.WorktreeRoot, CreatedAt: formatTime(rev.CreatedAt)}).Error; err != nil {
			return memoryStoreError(err)
		}
		if err := tx.Exec("DELETE FROM memory_fts WHERE project_id = ? AND memory_id = ?", m.ProjectID, m.ID).Error; err != nil {
			return memoryStoreError(err)
		}
		if m.DeletedAt == nil {
			if err := tx.Exec(`INSERT INTO memory_fts (memory_id,project_id,title,content) VALUES (?,?,?,?)`, m.ID, m.ProjectID, m.Title, m.Content).Error; err != nil {
				return memoryStoreError(err)
			}
		}
		return nil
	})
	if err != nil {
		return memory.Memory{}, err
	}
	return m, nil
}

// History returns immutable revisions in ascending version order.
func (r *MemoryRepository) History(ctx context.Context, projectID, id string) ([]memory.Revision, error) {
	if _, err := r.Get(ctx, projectID, id, true); err != nil {
		return nil, err
	}
	var rows []revisionRow
	if err := r.db.WithContext(ctx).Where("project_id = ? AND memory_id = ?", projectID, id).Order("version ASC").Find(&rows).Error; err != nil {
		return nil, memoryStoreError(err)
	}
	values := make([]memory.Revision, 0, len(rows))
	for _, row := range rows {
		value, err := row.revision()
		if err != nil {
			return nil, memoryStoreError(err)
		}
		values = append(values, value)
	}
	return values, nil
}

// Revision returns one historical snapshot scoped to its project and memory.
func (r *MemoryRepository) Revision(ctx context.Context, projectID, id string, version int) (memory.Revision, error) {
	var row revisionRow
	err := r.db.WithContext(ctx).Where("project_id = ? AND memory_id = ? AND version = ?", projectID, id, version).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return memory.Revision{}, memory.NewError(memory.CodeNotFound, errors.New("memory not found"))
	}
	if err != nil {
		return memory.Revision{}, memoryStoreError(err)
	}
	value, err := row.revision()
	if err != nil {
		return memory.Revision{}, memoryStoreError(err)
	}
	return value, nil
}

// ListTags returns the complete vocabulary for one project.
func (r *MemoryRepository) ListTags(ctx context.Context, projectID string) ([]memory.Tag, error) {
	return r.queryTags(ctx, projectID, "")
}

// SearchTags returns project tags matching a literal case-insensitive query.
func (r *MemoryRepository) SearchTags(ctx context.Context, projectID, query string) ([]memory.Tag, error) {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	return r.queryTags(ctx, projectID, "%"+escaped+"%")
}

type vocabularyRow struct {
	ID, Name, NormalizedName, CreatedAt, UpdatedAt string
	Description                                    *string
	ActiveMemoryCount                              int
}

func (r *MemoryRepository) queryTags(ctx context.Context, projectID, pattern string) ([]memory.Tag, error) {
	query := r.db.WithContext(ctx).Table("tags AS t").
		Select("t.id,t.name,t.normalized_name,t.description,t.created_at,t.updated_at,count(m.id) AS active_memory_count").
		Joins("LEFT JOIN memory_tags AS mt ON mt.project_id = t.project_id AND mt.tag_id = t.id").
		Joins("LEFT JOIN memories AS m ON m.project_id = mt.project_id AND m.id = mt.memory_id AND m.deleted_at IS NULL").
		Where("t.project_id = ?", projectID)
	if pattern != "" {
		query = query.Where(`lower(t.name) LIKE lower(?) ESCAPE '\' OR lower(coalesce(t.description, '')) LIKE lower(?) ESCAPE '\'`, pattern, pattern)
	}
	query = query.Group("t.id,t.name,t.normalized_name,t.description,t.created_at,t.updated_at")
	if pattern == "" {
		query = query.Order("t.normalized_name ASC")
	} else {
		query = query.Order("active_memory_count DESC").Order("t.normalized_name ASC")
	}
	var rows []vocabularyRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, memoryStoreError(err)
	}
	values := make([]memory.Tag, 0, len(rows))
	for _, row := range rows {
		created, err := time.Parse(time.RFC3339, row.CreatedAt)
		if err != nil {
			return nil, memoryStoreError(err)
		}
		updated, err := time.Parse(time.RFC3339, row.UpdatedAt)
		if err != nil {
			return nil, memoryStoreError(err)
		}
		values = append(values, memory.Tag{ID: row.ID, Name: row.Name, NormalizedName: row.NormalizedName, Description: row.Description, CreatedAt: created, UpdatedAt: updated, ActiveMemoryCount: row.ActiveMemoryCount})
	}
	return values, nil
}

type taggedRow struct {
	MemoryID, ID, Name, NormalizedName, CreatedAt, UpdatedAt string
	Description                                              *string
}

func (r *MemoryRepository) loadTags(ctx context.Context, projectID string, ids []string) (map[string][]memory.Tag, error) {
	out := map[string][]memory.Tag{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []taggedRow
	err := r.db.WithContext(ctx).Table("tags AS t").Select("mt.memory_id,t.id,t.name,t.normalized_name,t.description,t.created_at,t.updated_at").Joins("JOIN memory_tags AS mt ON mt.project_id = t.project_id AND mt.tag_id = t.id").Where("mt.project_id = ? AND mt.memory_id IN ?", projectID, ids).Order("t.normalized_name ASC").Order("t.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, memoryStoreError(err)
	}
	for _, row := range rows {
		stamp, parseErr := time.Parse(time.RFC3339, row.CreatedAt)
		if parseErr != nil {
			return nil, memoryStoreError(parseErr)
		}
		updated, parseErr := time.Parse(time.RFC3339, row.UpdatedAt)
		if parseErr != nil {
			return nil, memoryStoreError(parseErr)
		}
		out[row.MemoryID] = append(out[row.MemoryID], memory.Tag{ID: row.ID, Name: row.Name, NormalizedName: row.NormalizedName, Description: row.Description, CreatedAt: stamp, UpdatedAt: updated})
	}
	return out, nil
}
func toMemoryRow(m memory.Memory) memoryRow {
	var attrs *string
	if len(m.Attributes) > 0 {
		value := string(m.Attributes)
		attrs = &value
	}
	return memoryRow{ID: m.ID, ProjectID: m.ProjectID, CurrentVersion: m.Version, Type: m.Type, Title: m.Title, Content: m.Content, Importance: m.Importance, Confidence: m.Confidence, AttributesJSON: attrs, ContentHash: m.ContentHash, CreatedAt: formatTime(m.CreatedAt), UpdatedAt: formatTime(m.UpdatedAt), DeletedAt: nullableTime(m.DeletedAt)}
}
func toTagRow(projectID string, t memory.Tag) tagRow {
	return tagRow{ID: t.ID, ProjectID: projectID, Name: t.Name, NormalizedName: t.NormalizedName, Description: t.Description, CreatedAt: formatTime(t.CreatedAt), UpdatedAt: formatTime(t.UpdatedAt)}
}
func (row tagRow) tag() memory.Tag {
	created, _ := time.Parse(time.RFC3339, row.CreatedAt)
	updated, _ := time.Parse(time.RFC3339, row.UpdatedAt)
	return memory.Tag{ID: row.ID, Name: row.Name, NormalizedName: row.NormalizedName, Description: row.Description, CreatedAt: created, UpdatedAt: updated}
}
func (row memoryRow) memory() (memory.Memory, error) {
	created, err := time.Parse(time.RFC3339, row.CreatedAt)
	if err != nil {
		return memory.Memory{}, err
	}
	updated, err := time.Parse(time.RFC3339, row.UpdatedAt)
	if err != nil {
		return memory.Memory{}, err
	}
	var attributes json.RawMessage
	if row.AttributesJSON != nil {
		attributes = json.RawMessage(*row.AttributesJSON)
	}
	value := memory.Memory{ID: row.ID, ProjectID: row.ProjectID, Version: row.CurrentVersion, Type: row.Type, Title: row.Title, Content: row.Content, Importance: row.Importance, Confidence: row.Confidence, Attributes: attributes, ContentHash: row.ContentHash, CreatedAt: created, UpdatedAt: updated}
	if row.DeletedAt != nil {
		deleted, parseErr := time.Parse(time.RFC3339, *row.DeletedAt)
		if parseErr != nil {
			return memory.Memory{}, parseErr
		}
		value.DeletedAt = &deleted
	}
	return value, nil
}
func memoryStoreError(err error) error {
	return memory.NewError(memory.CodeStoreError, fmt.Errorf("storage operation failed: %w", err))
}

func nullableTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}

func activeDuplicate(tx *gorm.DB, projectID, hash, excludedID string) (string, bool) {
	var row memoryRow
	err := tx.Where("project_id = ? AND content_hash = ? AND deleted_at IS NULL AND id <> ?", projectID, hash, excludedID).Take(&row).Error
	return row.ID, err == nil
}

func (row revisionRow) revision() (memory.Revision, error) {
	created, err := time.Parse(time.RFC3339, row.CreatedAt)
	if err != nil {
		return memory.Revision{}, err
	}
	return memory.Revision{MemoryID: row.MemoryID, ProjectID: row.ProjectID, Version: row.Version, Operation: row.Operation, Snapshot: json.RawMessage(row.SnapshotJSON), AgentName: row.AgentName, AgentRole: row.AgentRole, WorktreeRoot: row.WorktreeRoot, CreatedAt: created}, nil
}
