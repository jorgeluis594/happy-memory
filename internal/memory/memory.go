// Package memory implements creation and active-memory queries.
package memory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Stable public memory error codes.
const (
	CodeValidationError = "VALIDATION_ERROR"
	CodeNotFound        = "MEMORY_NOT_FOUND"
	CodeDuplicate       = "DUPLICATE_MEMORY"
	CodeStoreError      = "STORE_ERROR"
)

var validTypes = map[string]bool{"fact": true, "decision": true, "constraint": true, "preference": true, "procedure": true, "lesson": true}

// Error carries a stable public code, optional details, and its cause.
type Error struct {
	Code    string
	Details map[string]any
	Err     error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// NewError creates a coded memory error.
func NewError(code string, err error) error { return &Error{Code: code, Err: err} }

// ErrorCode extracts a public memory error code.
func ErrorCode(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeStoreError
}

// ErrorDetails extracts safe public error details.
func ErrorDetails(err error) map[string]any {
	var e *Error
	if errors.As(err, &e) && e.Details != nil {
		return e.Details
	}
	return map[string]any{}
}

// Agent describes free-form creation provenance.
type Agent struct {
	Name *string `json:"name,omitempty"`
	Role *string `json:"role,omitempty"`
}

// CreateInput is the accepted memory creation payload.
type CreateInput struct {
	Type       string          `json:"type"`
	Title      string          `json:"title"`
	Content    string          `json:"content"`
	Importance int             `json:"importance"`
	Confidence int             `json:"confidence"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
	Tags       []string        `json:"tags"`
	Agent      *Agent          `json:"agent,omitempty"`
}

// Tag is a project-scoped canonical tag.
type Tag struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	NormalizedName string    `json:"normalized_name"`
	CreatedAt      time.Time `json:"created_at"`
}

// Memory is the current persisted state of one memory.
type Memory struct {
	ID          string
	ProjectID   string
	Version     int
	Type        string
	Title       string
	Content     string
	Importance  int
	Confidence  int
	Attributes  json.RawMessage
	Tags        []Tag
	ContentHash string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// Revision is an immutable snapshot and its provenance.
type Revision struct {
	MemoryID, ProjectID     string
	Version                 int
	Operation               string
	Snapshot                json.RawMessage
	AgentName               *string
	AgentRole, WorktreeRoot string
	CreatedAt               time.Time
}

// ListFilter restricts active memory queries.
type ListFilter struct {
	Type          string
	Tags          []string
	MinImportance int
	MinConfidence int
}

// CreateRecord groups the state written by one atomic creation.
type CreateRecord struct {
	Memory   Memory
	Revision Revision
}

// Repository persists and queries project-scoped memories.
type Repository interface {
	Create(context.Context, CreateRecord) (Memory, error)
	Get(context.Context, string, string) (Memory, error)
	List(context.Context, string, ListFilter) ([]Memory, error)
}

// ProjectContext identifies the current project and worktree.
type ProjectContext struct{ ID, WorktreeRoot string }

// ProjectResolver resolves the active project for a use case.
type ProjectResolver interface {
	Current(context.Context) (ProjectContext, error)
}

// Clock supplies timestamps.
type Clock interface{ Now() time.Time }

// IDGenerator supplies public identifiers.
type IDGenerator interface{ New() string }

// Service implements memory use cases.
type Service struct {
	projects ProjectResolver
	repo     Repository
	clock    Clock
	ids      IDGenerator
}

// NewService composes memory use cases.
func NewService(p ProjectResolver, r Repository, c Clock, ids IDGenerator) *Service {
	return &Service{projects: p, repo: r, clock: c, ids: ids}
}

// Create validates and atomically persists a new memory.
func (s *Service) Create(ctx context.Context, in CreateInput) (Memory, error) {
	if err := NormalizeAndValidate(&in); err != nil {
		return Memory{}, err
	}
	p, err := s.projects.Current(ctx)
	if err != nil {
		return Memory{}, err
	}
	now := s.clock.Now().UTC().Truncate(time.Second)
	m := Memory{ID: s.ids.New(), ProjectID: p.ID, Version: 1, Type: in.Type, Title: in.Title, Content: in.Content, Importance: in.Importance, Confidence: in.Confidence, Attributes: cloneJSON(in.Attributes), ContentHash: ContentHash(in.Type, in.Title, in.Content), CreatedAt: now, UpdatedAt: now}
	for _, name := range in.Tags {
		m.Tags = append(m.Tags, Tag{ID: s.ids.New(), Name: name, NormalizedName: NormalizeTag(name), CreatedAt: now})
	}
	role := "unknown"
	var agentName *string
	if in.Agent != nil {
		agentName = in.Agent.Name
		if in.Agent.Role != nil {
			role = *in.Agent.Role
		}
	}
	snapshot, err := SnapshotJSON(m, agentName, role, p.WorktreeRoot)
	if err != nil {
		return Memory{}, NewError(CodeStoreError, err)
	}
	r := Revision{MemoryID: m.ID, ProjectID: p.ID, Version: 1, Operation: "create", Snapshot: snapshot, AgentName: agentName, AgentRole: role, WorktreeRoot: p.WorktreeRoot, CreatedAt: now}
	return s.repo.Create(ctx, CreateRecord{Memory: m, Revision: r})
}

// Get returns one active memory from the current project.
func (s *Service) Get(ctx context.Context, id string) (Memory, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Memory{}, validation()
	}
	p, err := s.projects.Current(ctx)
	if err != nil {
		return Memory{}, err
	}
	return s.repo.Get(ctx, p.ID, id)
}

// List returns filtered active memories from the current project.
func (s *Service) List(ctx context.Context, f ListFilter) ([]Memory, error) {
	if err := ValidateFilter(&f); err != nil {
		return nil, err
	}
	p, err := s.projects.Current(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, p.ID, f)
}

// NormalizeAndValidate canonicalizes and validates creation input.
func NormalizeAndValidate(in *CreateInput) error {
	in.Type = strings.TrimSpace(in.Type)
	in.Title = normalizeText(in.Title)
	in.Content = normalizeText(in.Content)
	if !validTypes[in.Type] || in.Title == "" || in.Content == "" || in.Importance < 1 || in.Importance > 5 || in.Confidence < 1 || in.Confidence > 5 {
		return validation()
	}
	if len(in.Attributes) > 0 && !bytes.Equal(bytes.TrimSpace(in.Attributes), []byte("null")) {
		var obj map[string]any
		d := json.NewDecoder(bytes.NewReader(in.Attributes))
		d.UseNumber()
		if d.Decode(&obj) != nil || obj == nil {
			return validation()
		}
		canonical, _ := json.Marshal(obj)
		in.Attributes = canonical
	} else {
		in.Attributes = nil
	}
	seen := map[string]bool{}
	for i, name := range in.Tags {
		name = strings.TrimSpace(name)
		norm := NormalizeTag(name)
		if name == "" || seen[norm] {
			return validation()
		}
		seen[norm] = true
		in.Tags[i] = name
	}
	if in.Agent != nil {
		if in.Agent.Name != nil {
			v := strings.TrimSpace(*in.Agent.Name)
			if v == "" {
				return validation()
			}
			in.Agent.Name = &v
		}
		if in.Agent.Role != nil {
			v := strings.TrimSpace(*in.Agent.Role)
			if v == "" {
				return validation()
			}
			in.Agent.Role = &v
		}
	}
	return nil
}

// ValidateFilter canonicalizes and validates list filters.
func ValidateFilter(f *ListFilter) error {
	f.Type = strings.TrimSpace(f.Type)
	if f.Type != "" && !validTypes[f.Type] {
		return validation()
	}
	if f.MinImportance < 0 || f.MinImportance > 5 || f.MinConfidence < 0 || f.MinConfidence > 5 {
		return validation()
	}
	seen := map[string]bool{}
	for i, t := range f.Tags {
		n := NormalizeTag(t)
		if n == "" || seen[n] {
			return validation()
		}
		seen[n] = true
		f.Tags[i] = n
	}
	return nil
}

// NormalizeTag produces a project tag lookup key.
func NormalizeTag(v string) string { return strings.ToLower(strings.TrimSpace(v)) }

// ContentHash returns the deterministic identity hash for normalized content.
func ContentHash(kind, title, content string) string {
	b, _ := json.Marshal([]string{kind, title, content})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func normalizeText(v string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(v, "\r\n", "\n"), "\r", "\n"))
}
func validation() error                           { return NewError(CodeValidationError, errors.New("invalid input")) }
func cloneJSON(v json.RawMessage) json.RawMessage { return append(json.RawMessage(nil), v...) }

// SnapshotJSON serializes the complete immutable state captured by a revision.
func SnapshotJSON(m Memory, name *string, role, path string) (json.RawMessage, error) {
	type snapshotTag struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		NormalizedName string `json:"normalized_name"`
	}
	tags := make([]snapshotTag, 0, len(m.Tags))
	for _, tag := range m.Tags {
		tags = append(tags, snapshotTag{ID: tag.ID, Name: tag.Name, NormalizedName: tag.NormalizedName})
	}
	return json.Marshal(struct {
		ID           string          `json:"id"`
		ProjectID    string          `json:"project_id"`
		Version      int             `json:"version"`
		Type         string          `json:"type"`
		Title        string          `json:"title"`
		Content      string          `json:"content"`
		Importance   int             `json:"importance"`
		Confidence   int             `json:"confidence"`
		Attributes   json.RawMessage `json:"attributes"`
		Tags         []snapshotTag   `json:"tags"`
		ContentHash  string          `json:"content_hash"`
		CreatedAt    string          `json:"created_at"`
		UpdatedAt    string          `json:"updated_at"`
		DeletedAt    *string         `json:"deleted_at"`
		AgentName    *string         `json:"agent_name"`
		AgentRole    string          `json:"agent_role"`
		WorktreeRoot string          `json:"worktree_root"`
	}{m.ID, m.ProjectID, m.Version, m.Type, m.Title, m.Content, m.Importance, m.Confidence, nullableJSON(m.Attributes), tags, m.ContentHash, m.CreatedAt.UTC().Format(time.RFC3339), m.UpdatedAt.UTC().Format(time.RFC3339), nil, name, role, path})
}

func nullableJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage("null")
	}
	return value
}
