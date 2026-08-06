package sqlite

import (
	"context"

	"github.com/jorgeluis594/happy-memory/internal/memory"
	"github.com/jorgeluis594/happy-memory/internal/search"
	"gorm.io/gorm"
)

// SearchRepository retrieves BM25 candidates through SQLite FTS5.
type SearchRepository struct{ db *gorm.DB }

// NewSearchRepository creates an FTS5 search repository.
func NewSearchRepository(db *gorm.DB) *SearchRepository { return &SearchRepository{db: db} }

type searchRow struct {
	ID                                string
	ProjectID                         string
	CurrentVersion                    int
	Type, Title, Content              string
	Importance, Confidence            int
	AttributesJSON                    *string
	ContentHash, CreatedAt, UpdatedAt string
	DeletedAt                         *string
	BM25                              float64
}

// Candidates applies all filters before returning the fixed BM25 window.
func (r *SearchRepository) Candidates(ctx context.Context, projectID string, filter search.CandidateFilter) ([]search.Candidate, error) {
	query := r.db.WithContext(ctx).Table("memory_fts").
		Select("m.*, bm25(memory_fts, 0, 0, 5, 1) AS bm25").
		Joins("JOIN memories AS m ON m.project_id = memory_fts.project_id AND m.id = memory_fts.memory_id").
		Where("memory_fts MATCH ?", filter.Match).
		Where("m.project_id = ? AND m.deleted_at IS NULL", projectID)
	if filter.Type != "" {
		query = query.Where("m.type = ?", filter.Type)
	}
	if filter.MinImportance > 0 {
		query = query.Where("m.importance >= ?", filter.MinImportance)
	}
	if filter.MinConfidence > 0 {
		query = query.Where("m.confidence >= ?", filter.MinConfidence)
	}
	for _, tag := range filter.Tags {
		subquery := r.db.Table("memory_tags AS mt").Select("1").
			Joins("JOIN tags AS t ON t.project_id = mt.project_id AND t.id = mt.tag_id").
			Where("mt.project_id = m.project_id AND mt.memory_id = m.id AND t.normalized_name = ?", tag)
		query = query.Where("EXISTS (?)", subquery)
	}
	var rows []searchRow
	if err := query.Order("bm25 ASC").Order("m.id ASC").Limit(filter.Limit).Scan(&rows).Error; err != nil {
		return nil, memoryStoreError(err)
	}
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	tags, err := (&MemoryRepository{db: r.db}).loadTags(ctx, projectID, ids)
	if err != nil {
		return nil, err
	}
	values := make([]search.Candidate, 0, len(rows))
	for _, row := range rows {
		persisted := memoryRow{ID: row.ID, ProjectID: row.ProjectID, CurrentVersion: row.CurrentVersion, Type: row.Type, Title: row.Title, Content: row.Content, Importance: row.Importance, Confidence: row.Confidence, AttributesJSON: row.AttributesJSON, ContentHash: row.ContentHash, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt}
		value, convertErr := persisted.memory()
		if convertErr != nil {
			return nil, memoryStoreError(convertErr)
		}
		value.Tags = tags[value.ID]
		if value.Tags == nil {
			value.Tags = []memory.Tag{}
		}
		values = append(values, search.Candidate{Memory: value, BM25: row.BM25})
	}
	return values, nil
}
