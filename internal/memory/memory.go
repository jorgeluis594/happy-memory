// Package memory implements memory lifecycle rules and queries.
package memory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Stable public memory error codes.
const (
	CodeValidationError = "VALIDATION_ERROR"
	CodeNotFound        = "MEMORY_NOT_FOUND"
	CodeDuplicate       = "DUPLICATE_MEMORY"
	CodeVersionConflict = "VERSION_CONFLICT"
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

// Agent describes optional mutation provenance.
type Agent struct {
	Name *string `json:"name,omitempty"`
	Role *string `json:"role,omitempty"`
}

// TagInput is one tag supplied by a consumer. It accepts a string or an object.
type TagInput struct {
	Name        string
	Description *string
}

// UnmarshalJSON accepts both the legacy string and enriched object forms.
func (in *TagInput) UnmarshalJSON(data []byte) error {
	var name string
	if json.Unmarshal(data, &name) == nil {
		in.Name = name
		in.Description = nil
		return nil
	}
	var object struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&object) != nil || object.Name == "" {
		return validation()
	}
	in.Name, in.Description = object.Name, object.Description
	return nil
}

// CreateInput is the accepted memory creation payload.
type CreateInput struct {
	Type       string          `json:"type"`
	Title      string          `json:"title"`
	Content    string          `json:"content"`
	Importance int             `json:"importance"`
	Confidence int             `json:"confidence"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
	Tags       []TagInput      `json:"tags"`
	Agent      *Agent          `json:"agent,omitempty"`
}

// Field preserves the difference between an absent patch member and its zero value.
type Field[T any] struct {
	Set   bool
	Value T
}

// UpdateInput is a presence-aware partial memory update.
type UpdateInput struct {
	Type       Field[string]
	Title      Field[string]
	Content    Field[string]
	Importance Field[int]
	Confidence Field[int]
	Attributes Field[json.RawMessage]
	Tags       Field[[]TagInput]
	Agent      *Agent
}

// UnmarshalJSON rejects unknown fields and models attributes:null distinctly.
func (in *UpdateInput) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return validation()
	}
	for key, value := range raw {
		switch key {
		case "type":
			in.Type.Set = true
			if json.Unmarshal(value, &in.Type.Value) != nil {
				return validation()
			}
		case "title":
			in.Title.Set = true
			if json.Unmarshal(value, &in.Title.Value) != nil {
				return validation()
			}
		case "content":
			in.Content.Set = true
			if json.Unmarshal(value, &in.Content.Value) != nil {
				return validation()
			}
		case "importance":
			in.Importance.Set = true
			if json.Unmarshal(value, &in.Importance.Value) != nil {
				return validation()
			}
		case "confidence":
			in.Confidence.Set = true
			if json.Unmarshal(value, &in.Confidence.Value) != nil {
				return validation()
			}
		case "attributes":
			in.Attributes.Set = true
			in.Attributes.Value = cloneJSON(value)
		case "tags":
			in.Tags.Set = true
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) || json.Unmarshal(value, &in.Tags.Value) != nil {
				return validation()
			}
		case "agent":
			if json.Unmarshal(value, &in.Agent) != nil {
				return validation()
			}
		default:
			return validation()
		}
	}
	return nil
}

// Tag is a project-scoped canonical tag.
type Tag struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	NormalizedName    string    `json:"normalized_name"`
	Description       *string   `json:"description"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	ActiveMemoryCount int       `json:"active_memory_count,omitempty"`
}

// Memory is the current persisted state of one memory.
type Memory struct {
	ID, ProjectID          string
	Version                int
	Type, Title, Content   string
	Importance, Confidence int
	Attributes             json.RawMessage
	Tags                   []Tag
	ContentHash            string
	CreatedAt, UpdatedAt   time.Time
	DeletedAt              *time.Time
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

// ListFilter restricts project-scoped memory queries.
type ListFilter struct {
	Type                         string
	Tags                         []string
	MinImportance, MinConfidence int
	IncludeDeleted               bool
}

// CreateRecord groups the state written by one atomic creation.
type CreateRecord struct {
	Memory   Memory
	Revision Revision
}

// MutationRecord groups a CAS mutation and its new revision.
type MutationRecord struct {
	Memory          Memory
	Revision        Revision
	ExpectedVersion int
}

// Repository persists and queries project-scoped memories.
type Repository interface {
	Create(context.Context, CreateRecord) (Memory, error)
	Get(context.Context, string, string, bool) (Memory, error)
	List(context.Context, string, ListFilter) ([]Memory, error)
	Mutate(context.Context, MutationRecord) (Memory, error)
	History(context.Context, string, string) ([]Revision, error)
	Revision(context.Context, string, string, int) (Revision, error)
	ListTags(context.Context, string) ([]Tag, error)
	SearchTags(context.Context, string, string, int) ([]Tag, error)
}

// ProjectContext identifies the current project and worktree.
type ProjectContext struct{ ID, WorktreeRoot string }

// BatchOperation describes one independently atomic lifecycle mutation.
type BatchOperation struct {
	Operation       string
	MemoryID        string
	ExpectedVersion int
	CreateInput     *CreateInput
	UpdateInput     *UpdateInput
}

// BatchResult preserves the input position and outcome of one operation.
type BatchResult struct {
	Index     int
	Operation string
	Memory    Memory
	Err       error
}

// BatchResponse summarizes a fully processed batch.
type BatchResponse struct {
	Total, Succeeded, Failed int
	Results                  []BatchResult
}

// ProjectResolver resolves the active project for a use case.
type ProjectResolver interface {
	Current(context.Context) (ProjectContext, error)
}

// Clock supplies timestamps.
type Clock interface{ Now() time.Time }

// IDGenerator supplies public identifiers.
type IDGenerator interface{ New() string }

// Service implements memory lifecycle use cases.
type Service struct {
	projects ProjectResolver
	repo     Repository
	clock    Clock
	ids      IDGenerator
}

// NewService composes memory lifecycle use cases.
func NewService(p ProjectResolver, r Repository, c Clock, ids IDGenerator) *Service {
	return &Service{p, r, c, ids}
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
	return s.create(ctx, p, in)
}

func (s *Service) create(ctx context.Context, p ProjectContext, in CreateInput) (Memory, error) {
	now := s.now()
	m := Memory{ID: s.ids.New(), ProjectID: p.ID, Version: 1, Type: in.Type, Title: in.Title, Content: in.Content, Importance: in.Importance, Confidence: in.Confidence, Attributes: cloneJSON(in.Attributes), ContentHash: ContentHash(in.Type, in.Title, in.Content), CreatedAt: now, UpdatedAt: now}
	for _, input := range in.Tags {
		m.Tags = append(m.Tags, Tag{ID: s.ids.New(), Name: input.Name, NormalizedName: NormalizeTag(input.Name), Description: cloneString(input.Description), CreatedAt: now, UpdatedAt: now})
	}
	name, role := provenance(in.Agent)
	revision, err := buildRevision(m, "create", name, role, p.WorktreeRoot, now)
	if err != nil {
		return Memory{}, err
	}
	return s.repo.Create(ctx, CreateRecord{m, revision})
}

// Get returns one memory and optionally includes a deleted state.
func (s *Service) Get(ctx context.Context, id string, includeDeleted bool) (Memory, error) {
	if !validID(id) {
		return Memory{}, validation()
	}
	p, err := s.projects.Current(ctx)
	if err != nil {
		return Memory{}, err
	}
	return s.repo.Get(ctx, p.ID, id, includeDeleted)
}

// List returns filtered memories from the current project.
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

// Update applies a normalized partial update with optimistic concurrency.
func (s *Service) Update(ctx context.Context, id string, expected int, in UpdateInput) (Memory, error) {
	if !validID(id) || expected < 1 {
		return Memory{}, validation()
	}
	p, current, err := s.current(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	return s.update(ctx, p, current, expected, in)
}

func (s *Service) update(ctx context.Context, p ProjectContext, current Memory, expected int, in UpdateInput) (Memory, error) {
	if err := checkVersion(current, expected); err != nil {
		return Memory{}, err
	}
	if current.DeletedAt != nil {
		return Memory{}, validation()
	}
	next := cloneMemory(current)
	applyPatch(&next, in)
	create := CreateInput{Type: next.Type, Title: next.Title, Content: next.Content, Importance: next.Importance, Confidence: next.Confidence, Attributes: next.Attributes, Tags: tagInputs(next.Tags), Agent: in.Agent}
	if in.Tags.Set {
		create.Tags = append([]TagInput(nil), in.Tags.Value...)
	}
	if err := NormalizeAndValidate(&create); err != nil {
		return Memory{}, err
	}
	next.Type, next.Title, next.Content, next.Importance, next.Confidence, next.Attributes = create.Type, create.Title, create.Content, create.Importance, create.Confidence, cloneJSON(create.Attributes)
	if in.Tags.Set {
		next.Tags = make([]Tag, 0, len(create.Tags))
		for _, tag := range create.Tags {
			now := s.now()
			next.Tags = append(next.Tags, Tag{ID: s.ids.New(), Name: tag.Name, NormalizedName: NormalizeTag(tag.Name), Description: cloneString(tag.Description), CreatedAt: now, UpdatedAt: now})
		}
	}
	if equivalent(current, next) {
		return current, nil
	}
	next.Version++
	next.UpdatedAt = s.now()
	next.ContentHash = ContentHash(next.Type, next.Title, next.Content)
	name, role := provenance(in.Agent)
	rev, e := buildRevision(next, "update", name, role, p.WorktreeRoot, next.UpdatedAt)
	if e != nil {
		return Memory{}, e
	}
	return s.repo.Mutate(ctx, MutationRecord{next, rev, expected})
}

// Delete logically deletes an active memory.
func (s *Service) Delete(ctx context.Context, id string, expected int) (Memory, error) {
	if !validID(id) || expected < 1 {
		return Memory{}, validation()
	}
	p, current, err := s.current(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	return s.delete(ctx, p, current, expected)
}

func (s *Service) delete(ctx context.Context, p ProjectContext, current Memory, expected int) (Memory, error) {
	if err := checkVersion(current, expected); err != nil {
		return Memory{}, err
	}
	if current.DeletedAt != nil {
		return Memory{}, validation()
	}
	now := s.now()
	next := cloneMemory(current)
	next.Version++
	next.UpdatedAt = now
	next.DeletedAt = &now
	rev, e := buildRevision(next, "delete", nil, "unknown", p.WorktreeRoot, now)
	if e != nil {
		return Memory{}, e
	}
	return s.repo.Mutate(ctx, MutationRecord{next, rev, expected})
}

// Batch validates the envelope, resolves the project once, and runs operations in order.
func (s *Service) Batch(ctx context.Context, operations []BatchOperation) (BatchResponse, error) {
	if err := validateBatch(operations); err != nil {
		return BatchResponse{}, err
	}
	p, err := s.projects.Current(ctx)
	if err != nil {
		return BatchResponse{}, err
	}
	response := BatchResponse{Total: len(operations), Results: make([]BatchResult, 0, len(operations))}
	for index, operation := range operations {
		result := BatchResult{Index: index, Operation: operation.Operation}
		switch operation.Operation {
		case "create":
			input := *operation.CreateInput
			if result.Err = NormalizeAndValidate(&input); result.Err == nil {
				result.Memory, result.Err = s.create(ctx, p, input)
			}
		case "update", "delete":
			var current Memory
			current, result.Err = s.repo.Get(ctx, p.ID, operation.MemoryID, true)
			if result.Err == nil && operation.Operation == "update" {
				result.Memory, result.Err = s.update(ctx, p, current, operation.ExpectedVersion, *operation.UpdateInput)
			} else if result.Err == nil {
				result.Memory, result.Err = s.delete(ctx, p, current, operation.ExpectedVersion)
			}
		}
		if result.Err == nil {
			response.Succeeded++
		} else {
			response.Failed++
		}
		response.Results = append(response.Results, result)
	}
	return response, nil
}

func validateBatch(operations []BatchOperation) error {
	if len(operations) < 1 || len(operations) > 100 {
		return validation()
	}
	seen := make(map[string]bool)
	for _, operation := range operations {
		switch operation.Operation {
		case "create":
			if operation.CreateInput == nil || operation.UpdateInput != nil || operation.MemoryID != "" || operation.ExpectedVersion != 0 {
				return validation()
			}
		case "update":
			if operation.UpdateInput == nil || operation.CreateInput != nil || !validID(operation.MemoryID) || operation.ExpectedVersion < 1 || seen[operation.MemoryID] {
				return validation()
			}
			seen[operation.MemoryID] = true
		case "delete":
			if operation.CreateInput != nil || operation.UpdateInput != nil || !validID(operation.MemoryID) || operation.ExpectedVersion < 1 || seen[operation.MemoryID] {
				return validation()
			}
			seen[operation.MemoryID] = true
		default:
			return validation()
		}
	}
	return nil
}

// Restore reactivates a deleted memory from a historical revision.
func (s *Service) Restore(ctx context.Context, id string, version, expected int) (Memory, error) {
	if !validID(id) || version < 1 || expected < 1 {
		return Memory{}, validation()
	}
	p, current, err := s.current(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	if err = checkVersion(current, expected); err != nil {
		return Memory{}, err
	}
	if current.DeletedAt == nil {
		return Memory{}, validation()
	}
	rev, err := s.repo.Revision(ctx, p.ID, id, version)
	if err != nil {
		return Memory{}, err
	}
	restored, err := FromSnapshot(rev.Snapshot)
	if err != nil {
		return Memory{}, NewError(CodeStoreError, err)
	}
	now := s.now()
	restored.ProjectID = p.ID
	restored.ID = id
	restored.Version = current.Version + 1
	restored.CreatedAt = current.CreatedAt
	restored.UpdatedAt = now
	restored.DeletedAt = nil
	restored.ContentHash = ContentHash(restored.Type, restored.Title, restored.Content)
	newRev, e := buildRevision(restored, "restore", nil, "unknown", p.WorktreeRoot, now)
	if e != nil {
		return Memory{}, e
	}
	return s.repo.Mutate(ctx, MutationRecord{restored, newRev, expected})
}

// History returns all revisions for an active or deleted memory.
func (s *Service) History(ctx context.Context, id string) ([]Revision, error) {
	if !validID(id) {
		return nil, validation()
	}
	p, err := s.projects.Current(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.History(ctx, p.ID, id)
}

// TagsList returns the complete project-scoped tag vocabulary.
func (s *Service) TagsList(ctx context.Context) ([]Tag, error) {
	p, err := s.projects.Current(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.ListTags(ctx, p.ID)
}

// TagsSearch returns project tags matching name or description.
func (s *Service) TagsSearch(ctx context.Context, query string, limit int) ([]Tag, error) {
	query = strings.TrimSpace(query)
	if query == "" || limit < 1 || limit > 100 {
		return nil, validation()
	}
	p, err := s.projects.Current(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.SearchTags(ctx, p.ID, query, limit)
}
func (s *Service) current(ctx context.Context, id string) (ProjectContext, Memory, error) {
	p, err := s.projects.Current(ctx)
	if err != nil {
		return p, Memory{}, err
	}
	m, err := s.repo.Get(ctx, p.ID, id, true)
	return p, m, err
}
func (s *Service) now() time.Time { return s.clock.Now().UTC().Truncate(time.Second) }

// NormalizeAndValidate canonicalizes and validates a complete state.
func NormalizeAndValidate(in *CreateInput) error {
	in.Type = strings.TrimSpace(in.Type)
	in.Title = normalizeText(in.Title)
	in.Content = normalizeText(in.Content)
	if !validTypes[in.Type] || in.Title == "" || in.Content == "" || in.Importance < 1 || in.Importance > 5 || in.Confidence < 1 || in.Confidence > 5 {
		return validation()
	}
	attrs, err := normalizeAttributes(in.Attributes)
	if err != nil {
		return err
	}
	in.Attributes = attrs
	seen := map[string]bool{}
	for i, tag := range in.Tags {
		tag.Name = strings.TrimSpace(tag.Name)
		norm := NormalizeTag(tag.Name)
		if norm == "" || seen[norm] {
			return validation()
		}
		if tag.Description != nil {
			description := strings.TrimSpace(*tag.Description)
			if description == "" {
				tag.Description = nil
			} else {
				tag.Description = &description
			}
		}
		seen[norm] = true
		in.Tags[i] = tag
	}
	return validateAgent(in.Agent)
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
func NormalizeTag(v string) string {
	value := strings.ToLower(strings.Join(strings.Fields(v), "-"))
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return value
}

// ContentHash returns the deterministic identity hash for normalized content.
func ContentHash(kind, title, content string) string {
	b, _ := json.Marshal([]string{kind, title, content})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func applyPatch(m *Memory, in UpdateInput) {
	if in.Type.Set {
		m.Type = in.Type.Value
	}
	if in.Title.Set {
		m.Title = in.Title.Value
	}
	if in.Content.Set {
		m.Content = in.Content.Value
	}
	if in.Importance.Set {
		m.Importance = in.Importance.Value
	}
	if in.Confidence.Set {
		m.Confidence = in.Confidence.Value
	}
	if in.Attributes.Set {
		m.Attributes = cloneJSON(in.Attributes.Value)
	}
}
func equivalent(a, b Memory) bool {
	if a.Type != b.Type || a.Title != b.Title || a.Content != b.Content || a.Importance != b.Importance || a.Confidence != b.Confidence || !bytes.Equal(nullableJSON(a.Attributes), nullableJSON(b.Attributes)) {
		return false
	}
	an, bn := tagNorms(a.Tags), tagNorms(b.Tags)
	slices.Sort(an)
	slices.Sort(bn)
	return slices.Equal(an, bn)
}
func tagNorms(tags []Tag) []string {
	out := make([]string, len(tags))
	for i := range tags {
		out[i] = tags[i].NormalizedName
	}
	return out
}
func tagInputs(tags []Tag) []TagInput {
	out := make([]TagInput, len(tags))
	for i := range tags {
		out[i] = TagInput{Name: tags[i].Name, Description: cloneString(tags[i].Description)}
	}
	return out
}
func checkVersion(m Memory, expected int) error {
	if m.Version != expected {
		return &Error{Code: CodeVersionConflict, Details: map[string]any{"memory_id": m.ID, "current_version": m.Version}, Err: errors.New("version conflict")}
	}
	return nil
}
func provenance(agent *Agent) (*string, string) {
	role := "unknown"
	var name *string
	if agent != nil {
		name = agent.Name
		if agent.Role != nil {
			role = *agent.Role
		}
	}
	return name, role
}
func validateAgent(agent *Agent) error {
	if agent == nil {
		return nil
	}
	if agent.Name != nil {
		v := strings.TrimSpace(*agent.Name)
		if v == "" {
			return validation()
		}
		agent.Name = &v
	}
	if agent.Role != nil {
		v := strings.TrimSpace(*agent.Role)
		if v == "" {
			return validation()
		}
		agent.Role = &v
	}
	return nil
}
func normalizeAttributes(value json.RawMessage) (json.RawMessage, error) {
	if len(value) == 0 || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return nil, nil
	}
	var obj map[string]any
	d := json.NewDecoder(bytes.NewReader(value))
	d.UseNumber()
	if d.Decode(&obj) != nil || obj == nil {
		return nil, validation()
	}
	canonical, _ := json.Marshal(obj)
	return canonical, nil
}
func normalizeText(v string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(v, "\r\n", "\n"), "\r", "\n"))
}
func validID(id string) bool                      { _, err := uuid.Parse(id); return err == nil }
func validation() error                           { return NewError(CodeValidationError, errors.New("invalid input")) }
func cloneJSON(v json.RawMessage) json.RawMessage { return append(json.RawMessage(nil), v...) }
func cloneMemory(m Memory) Memory {
	m.Attributes = cloneJSON(m.Attributes)
	m.Tags = append([]Tag(nil), m.Tags...)
	return m
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

type snapshotTag struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	NormalizedName string  `json:"normalized_name"`
	Description    *string `json:"description"`
	CreatedAt      *string `json:"created_at,omitempty"`
	UpdatedAt      *string `json:"updated_at,omitempty"`
}
type snapshot struct {
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
}

// SnapshotJSON serializes the complete immutable state captured by a revision.
func SnapshotJSON(m Memory, name *string, role, path string) (json.RawMessage, error) {
	tags := make([]snapshotTag, 0, len(m.Tags))
	for _, tag := range m.Tags {
		created := tag.CreatedAt.UTC().Format(time.RFC3339)
		updated := tag.UpdatedAt.UTC().Format(time.RFC3339)
		tags = append(tags, snapshotTag{tag.ID, tag.Name, tag.NormalizedName, tag.Description, &created, &updated})
	}
	var deleted *string
	if m.DeletedAt != nil {
		v := m.DeletedAt.UTC().Format(time.RFC3339)
		deleted = &v
	}
	return json.Marshal(snapshot{m.ID, m.ProjectID, m.Version, m.Type, m.Title, m.Content, m.Importance, m.Confidence, nullableJSON(m.Attributes), tags, m.ContentHash, m.CreatedAt.UTC().Format(time.RFC3339), m.UpdatedAt.UTC().Format(time.RFC3339), deleted, name, role, path})
}

// FromSnapshot decodes the state portion of a stored revision.
func FromSnapshot(data json.RawMessage) (Memory, error) {
	var s snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return Memory{}, err
	}
	created, err := time.Parse(time.RFC3339, s.CreatedAt)
	if err != nil {
		return Memory{}, err
	}
	updated, err := time.Parse(time.RFC3339, s.UpdatedAt)
	if err != nil {
		return Memory{}, err
	}
	m := Memory{ID: s.ID, ProjectID: s.ProjectID, Version: s.Version, Type: s.Type, Title: s.Title, Content: s.Content, Importance: s.Importance, Confidence: s.Confidence, Attributes: cloneJSON(s.Attributes), ContentHash: s.ContentHash, CreatedAt: created, UpdatedAt: updated}
	if bytes.Equal(m.Attributes, []byte("null")) {
		m.Attributes = nil
	}
	for _, t := range s.Tags {
		tag := Tag{ID: t.ID, Name: t.Name, NormalizedName: t.NormalizedName, Description: cloneString(t.Description)}
		if t.CreatedAt != nil {
			tag.CreatedAt, _ = time.Parse(time.RFC3339, *t.CreatedAt)
		}
		if t.UpdatedAt != nil {
			tag.UpdatedAt, _ = time.Parse(time.RFC3339, *t.UpdatedAt)
		} else {
			tag.UpdatedAt = tag.CreatedAt
		}
		m.Tags = append(m.Tags, tag)
	}
	if s.DeletedAt != nil {
		v, e := time.Parse(time.RFC3339, *s.DeletedAt)
		if e != nil {
			return Memory{}, e
		}
		m.DeletedAt = &v
	}
	return m, nil
}
func buildRevision(m Memory, op string, name *string, role, path string, now time.Time) (Revision, error) {
	data, err := SnapshotJSON(m, name, role, path)
	if err != nil {
		return Revision{}, NewError(CodeStoreError, err)
	}
	return Revision{m.ID, m.ProjectID, m.Version, op, data, name, role, path, now}, nil
}
func nullableJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage("null")
	}
	return value
}
