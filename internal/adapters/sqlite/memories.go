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

type tagRow struct{ ID, ProjectID, Name, NormalizedName, CreatedAt string }

func (tagRow) TableName() string { return "tags" }

type memoryTagRow struct{ ProjectID, MemoryID, TagID string }

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
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
			if err := tx.Create(&memoryTagRow{ProjectID: m.ProjectID, MemoryID: m.ID, TagID: canonical.ID}).Error; err != nil {
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
		names := make([]string, len(m.Tags))
		for i := range m.Tags {
			names[i] = m.Tags[i].Name
		}
		if err := tx.Exec(`INSERT INTO memory_fts (memory_id,project_id,title,content,tags) VALUES (?,?,?,?,?)`, m.ID, m.ProjectID, m.Title, m.Content, strings.Join(names, " ")).Error; err != nil {
			return memoryStoreError(err)
		}
		return nil
	})
	if err != nil {
		return memory.Memory{}, err
	}
	return m, nil
}

// Get retrieves one active project-scoped memory.
func (r *MemoryRepository) Get(ctx context.Context, projectID, id string) (memory.Memory, error) {
	var row memoryRow
	err := r.db.WithContext(ctx).Where("project_id = ? AND id = ? AND deleted_at IS NULL", projectID, id).Take(&row).Error
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
	query := r.db.WithContext(ctx).Where("project_id = ? AND deleted_at IS NULL", projectID)
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

type taggedRow struct{ MemoryID, ID, Name, NormalizedName, CreatedAt string }

func (r *MemoryRepository) loadTags(ctx context.Context, projectID string, ids []string) (map[string][]memory.Tag, error) {
	out := map[string][]memory.Tag{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []taggedRow
	err := r.db.WithContext(ctx).Table("tags AS t").Select("mt.memory_id,t.id,t.name,t.normalized_name,t.created_at").Joins("JOIN memory_tags AS mt ON mt.project_id = t.project_id AND mt.tag_id = t.id").Where("mt.project_id = ? AND mt.memory_id IN ?", projectID, ids).Order("t.normalized_name ASC").Order("t.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, memoryStoreError(err)
	}
	for _, row := range rows {
		stamp, parseErr := time.Parse(time.RFC3339, row.CreatedAt)
		if parseErr != nil {
			return nil, memoryStoreError(parseErr)
		}
		out[row.MemoryID] = append(out[row.MemoryID], memory.Tag{ID: row.ID, Name: row.Name, NormalizedName: row.NormalizedName, CreatedAt: stamp})
	}
	return out, nil
}
func toMemoryRow(m memory.Memory) memoryRow {
	var attrs *string
	if len(m.Attributes) > 0 {
		value := string(m.Attributes)
		attrs = &value
	}
	return memoryRow{ID: m.ID, ProjectID: m.ProjectID, CurrentVersion: m.Version, Type: m.Type, Title: m.Title, Content: m.Content, Importance: m.Importance, Confidence: m.Confidence, AttributesJSON: attrs, ContentHash: m.ContentHash, CreatedAt: formatTime(m.CreatedAt), UpdatedAt: formatTime(m.UpdatedAt)}
}
func toTagRow(projectID string, t memory.Tag) tagRow {
	return tagRow{ID: t.ID, ProjectID: projectID, Name: t.Name, NormalizedName: t.NormalizedName, CreatedAt: formatTime(t.CreatedAt)}
}
func (row tagRow) tag() memory.Tag {
	stamp, _ := time.Parse(time.RFC3339, row.CreatedAt)
	return memory.Tag{ID: row.ID, Name: row.Name, NormalizedName: row.NormalizedName, CreatedAt: stamp}
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
