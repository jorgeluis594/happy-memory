package memory

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type projectStub struct{ value ProjectContext }

func (s projectStub) Current(context.Context) (ProjectContext, error) { return s.value, nil }

type countingProjectStub struct {
	value ProjectContext
	calls int
}

func (s *countingProjectStub) Current(context.Context) (ProjectContext, error) {
	s.calls++
	return s.value, nil
}

type clockStub struct{ value time.Time }

func (s clockStub) Now() time.Time { return s.value }

type idsStub struct {
	values []string
	index  int
}

func (s *idsStub) New() string { value := s.values[s.index]; s.index++; return value }

type repoStub struct{ record CreateRecord }

func (r *repoStub) Create(_ context.Context, record CreateRecord) (Memory, error) {
	r.record = record
	return record.Memory, nil
}
func (*repoStub) Get(context.Context, string, string, bool) (Memory, error)   { return Memory{}, nil }
func (*repoStub) List(context.Context, string, ListFilter) ([]Memory, error)  { return nil, nil }
func (*repoStub) Mutate(context.Context, MutationRecord) (Memory, error)      { return Memory{}, nil }
func (*repoStub) History(context.Context, string, string) ([]Revision, error) { return nil, nil }
func (*repoStub) Revision(context.Context, string, string, int) (Revision, error) {
	return Revision{}, nil
}
func (*repoStub) ListTags(context.Context, string) ([]Tag, error)                { return nil, nil }
func (*repoStub) SearchTags(context.Context, string, string, int) ([]Tag, error) { return nil, nil }

func TestCreateNormalizesAndBuildsInitialRevision(t *testing.T) {
	stamp := time.Date(2026, 8, 5, 12, 0, 0, 987, time.FixedZone("x", -5*60*60))
	repo := &repoStub{}
	ids := &idsStub{values: []string{"memory-id", "tag-id"}}
	service := NewService(projectStub{ProjectContext{ID: "project-id", WorktreeRoot: "/repo/worktree"}}, repo, clockStub{stamp}, ids)
	name, role := " Agent One ", " database-specialist "
	description := " Data storage "
	value, err := service.Create(context.Background(), CreateInput{Type: "decision", Title: "  title\r\n", Content: "body\rline  ", Importance: 5, Confidence: 4, Attributes: json.RawMessage(`{"z":1,"a":true}`), Tags: []TagInput{{Name: " DataBase ", Description: &description}}, Agent: &Agent{Name: &name, Role: &role}})
	if err != nil {
		t.Fatal(err)
	}
	if value.Title != "title" || value.Content != "body\nline" || string(value.Attributes) != `{"a":true,"z":1}` {
		t.Fatalf("unexpected normalized value: %#v", value)
	}
	if value.ContentHash != ContentHash("decision", "title", "body\nline") {
		t.Fatal("unexpected hash")
	}
	if value.Tags[0].NormalizedName != "database" || value.Tags[0].Description == nil || *value.Tags[0].Description != "Data storage" {
		t.Fatalf("tag = %#v", value.Tags[0])
	}
	if !value.CreatedAt.Equal(time.Date(2026, 8, 5, 17, 0, 0, 0, time.UTC)) {
		t.Fatalf("timestamp = %v", value.CreatedAt)
	}
	if repo.record.Revision.AgentRole != "database-specialist" || repo.record.Revision.AgentName == nil || *repo.record.Revision.AgentName != "Agent One" || repo.record.Revision.WorktreeRoot != "/repo/worktree" {
		t.Fatalf("revision provenance = %#v", repo.record.Revision)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(repo.record.Revision.Snapshot, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["project_id"] != "project-id" || snapshot["agent_role"] != "database-specialist" {
		t.Fatalf("snapshot = %s", repo.record.Revision.Snapshot)
	}
}

func TestCreateValidation(t *testing.T) {
	base := CreateInput{Type: "fact", Title: "title", Content: "content", Importance: 1, Confidence: 1, Tags: []TagInput{}}
	cases := []CreateInput{base, base, base, base, base}
	cases[0].Type = "other"
	cases[1].Title = " \r\n "
	cases[2].Importance = 6
	cases[3].Attributes = json.RawMessage(`[]`)
	cases[4].Tags = []TagInput{{Name: "Tag"}, {Name: " tag "}}
	for i, value := range cases {
		err := NormalizeAndValidate(&value)
		if ErrorCode(err) != CodeValidationError {
			t.Errorf("case %d error = %v", i, err)
		}
	}
	name := " "
	value := base
	value.Agent = &Agent{Name: &name}
	if ErrorCode(NormalizeAndValidate(&value)) != CodeValidationError {
		t.Fatal("empty agent name accepted")
	}
}

func TestMissingRoleUsesUnknown(t *testing.T) {
	repo := &repoStub{}
	ids := &idsStub{values: []string{"memory-id"}}
	service := NewService(projectStub{ProjectContext{ID: "project", WorktreeRoot: "/repo"}}, repo, clockStub{time.Now()}, ids)
	_, err := service.Create(context.Background(), CreateInput{Type: "fact", Title: "t", Content: "c", Importance: 1, Confidence: 1})
	if err != nil {
		t.Fatal(err)
	}
	if repo.record.Revision.AgentRole != "unknown" {
		t.Fatal(repo.record.Revision.AgentRole)
	}
}

func TestErrorDetailsDefaultEmpty(t *testing.T) {
	if len(ErrorDetails(errors.New("x"))) != 0 {
		t.Fatal("details should be empty")
	}
}

type lifecycleRepo struct {
	current   Memory
	revisions []Revision
	mutations int
}

func (r *lifecycleRepo) Create(_ context.Context, record CreateRecord) (Memory, error) {
	r.current = record.Memory
	r.revisions = append(r.revisions, record.Revision)
	return r.current, nil
}
func (r *lifecycleRepo) Get(_ context.Context, projectID, id string, includeDeleted bool) (Memory, error) {
	if r.current.ProjectID != projectID || r.current.ID != id || (!includeDeleted && r.current.DeletedAt != nil) {
		return Memory{}, NewError(CodeNotFound, errors.New("not found"))
	}
	return cloneMemory(r.current), nil
}
func (*lifecycleRepo) List(context.Context, string, ListFilter) ([]Memory, error) { return nil, nil }
func (*lifecycleRepo) ListTags(context.Context, string) ([]Tag, error)            { return nil, nil }
func (*lifecycleRepo) SearchTags(context.Context, string, string, int) ([]Tag, error) {
	return nil, nil
}
func (r *lifecycleRepo) Mutate(_ context.Context, record MutationRecord) (Memory, error) {
	if r.current.Version != record.ExpectedVersion {
		return Memory{}, checkVersion(r.current, record.ExpectedVersion)
	}
	r.current = cloneMemory(record.Memory)
	r.revisions = append(r.revisions, record.Revision)
	r.mutations++
	return cloneMemory(r.current), nil
}
func (r *lifecycleRepo) History(context.Context, string, string) ([]Revision, error) {
	return append([]Revision(nil), r.revisions...), nil
}
func (r *lifecycleRepo) Revision(_ context.Context, projectID, id string, version int) (Revision, error) {
	for _, revision := range r.revisions {
		if revision.ProjectID == projectID && revision.MemoryID == id && revision.Version == version {
			return revision, nil
		}
	}
	return Revision{}, NewError(CodeNotFound, errors.New("not found"))
}

func TestLifecycleNoOpConflictDeleteAndRestore(t *testing.T) {
	id := "28fef1e4-42c5-43ca-a0c8-0c731797c06f"
	repo := &lifecycleRepo{}
	ids := &idsStub{values: []string{id, "tag-1", "tag-2"}}
	clock := clockStub{time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	service := NewService(projectStub{ProjectContext{ID: "project", WorktreeRoot: "/repo"}}, repo, clock, ids)
	created, err := service.Create(context.Background(), CreateInput{Type: "fact", Title: "title", Content: "content", Importance: 3, Confidence: 4, Tags: []TagInput{{Name: "Go"}}})
	if err != nil {
		t.Fatal(err)
	}
	var patch UpdateInput
	if err = json.Unmarshal([]byte(`{"title":" changed ","attributes":null,"tags":["Database"]}`), &patch); err != nil {
		t.Fatal(err)
	}
	updated, err := service.Update(context.Background(), id, 1, patch)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 || updated.Title != "changed" || len(updated.Attributes) != 0 || updated.Tags[0].NormalizedName != "database" {
		t.Fatalf("updated=%#v", updated)
	}
	noOp, err := service.Update(context.Background(), id, 2, UpdateInput{Agent: &Agent{}})
	if err != nil || noOp.Version != 2 || repo.mutations != 1 {
		t.Fatalf("no-op=%#v mutations=%d err=%v", noOp, repo.mutations, err)
	}
	if _, err = service.Delete(context.Background(), id, 1); ErrorCode(err) != CodeVersionConflict {
		t.Fatalf("conflict=%v", err)
	}
	deleted, err := service.Delete(context.Background(), id, 2)
	if err != nil || deleted.Version != 3 || deleted.DeletedAt == nil {
		t.Fatalf("deleted=%#v err=%v", deleted, err)
	}
	restored, err := service.Restore(context.Background(), id, created.Version, 3)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Version != 4 || restored.DeletedAt != nil || restored.Title != "title" {
		t.Fatalf("restored=%#v", restored)
	}
	if len(repo.revisions) != 4 {
		t.Fatalf("revisions=%d", len(repo.revisions))
	}
}

func TestTagInputFormatsNormalizationAndSearchValidation(t *testing.T) {
	var input CreateInput
	if err := json.Unmarshal([]byte(`{"type":"fact","title":"t","content":"c","importance":1,"confidence":1,"tags":[" Git  Operations ",{"name":"Data--Layer","description":" storage "}]}`), &input); err != nil {
		t.Fatal(err)
	}
	if err := NormalizeAndValidate(&input); err != nil {
		t.Fatal(err)
	}
	if NormalizeTag(input.Tags[0].Name) != "git-operations" || NormalizeTag(input.Tags[1].Name) != "data-layer" || input.Tags[1].Description == nil || *input.Tags[1].Description != "storage" {
		t.Fatalf("tags = %#v", input.Tags)
	}
	for _, body := range []string{
		`{"type":"fact","title":"t","content":"c","importance":1,"confidence":1,"tags":[{"name":"x","unknown":true}]}`,
		`{"type":"fact","title":"t","content":"c","importance":1,"confidence":1,"tags":["git operations","git--operations"]}`,
	} {
		var invalid CreateInput
		if err := json.Unmarshal([]byte(body), &invalid); err == nil && NormalizeAndValidate(&invalid) == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	projects := &countingProjectStub{}
	service := NewService(projects, &repoStub{}, clockStub{}, &idsStub{})
	if ErrorCode(func() error { _, err := service.TagsSearch(context.Background(), "  ", 10); return err }()) != CodeValidationError {
		t.Fatal("empty tag query accepted")
	}
	for _, limit := range []int{0, -1, 101} {
		if ErrorCode(func() error { _, err := service.TagsSearch(context.Background(), "tag", limit); return err }()) != CodeValidationError {
			t.Fatalf("invalid tag search limit accepted: %d", limit)
		}
	}
	if projects.calls != 0 {
		t.Fatalf("project resolved during validation: %d calls", projects.calls)
	}
	for _, limit := range []int{1, 100} {
		if _, err := service.TagsSearch(context.Background(), "tag", limit); err != nil {
			t.Fatalf("valid tag search limit rejected: %d: %v", limit, err)
		}
	}
	if projects.calls != 2 {
		t.Fatalf("valid searches resolved project %d times", projects.calls)
	}
}

func TestUpdateInputPresenceAndNullRules(t *testing.T) {
	var patch UpdateInput
	if err := json.Unmarshal([]byte(`{"attributes":null,"tags":[]}`), &patch); err != nil {
		t.Fatal(err)
	}
	if !patch.Attributes.Set || string(patch.Attributes.Value) != "null" || !patch.Tags.Set || patch.Tags.Value == nil {
		t.Fatalf("patch=%#v", patch)
	}
	for _, body := range []string{`{"tags":null}`, `{"unknown":true}`, `[]`} {
		if json.Unmarshal([]byte(body), &patch) == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}

func TestBatchPreservesOrderContinuesAndResolvesProjectOnce(t *testing.T) {
	id := "28fef1e4-42c5-43ca-a0c8-0c731797c06f"
	repo := &lifecycleRepo{}
	projects := &countingProjectStub{value: ProjectContext{ID: "project", WorktreeRoot: "/repo"}}
	service := NewService(projects, repo, clockStub{time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)}, &idsStub{values: []string{id}})
	invalid := CreateInput{Type: "invalid"}
	valid := CreateInput{Type: "fact", Title: "title", Content: "content", Importance: 3, Confidence: 4, Tags: []TagInput{}}
	response, err := service.Batch(context.Background(), []BatchOperation{
		{Operation: "create", CreateInput: &invalid},
		{Operation: "create", CreateInput: &valid},
		{Operation: "delete", MemoryID: id, ExpectedVersion: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if projects.calls != 1 || response.Total != 3 || response.Succeeded != 2 || response.Failed != 1 {
		t.Fatalf("projects=%d response=%#v", projects.calls, response)
	}
	if response.Results[0].Index != 0 || ErrorCode(response.Results[0].Err) != CodeValidationError || response.Results[1].Memory.ID != id || response.Results[2].Memory.DeletedAt == nil {
		t.Fatalf("results=%#v", response.Results)
	}
}

func TestBatchRejectsGlobalShapeBeforeResolutionOrMutation(t *testing.T) {
	id := "28fef1e4-42c5-43ca-a0c8-0c731797c06f"
	for name, operations := range map[string][]BatchOperation{
		"empty":     {},
		"duplicate": {{Operation: "update", MemoryID: id, ExpectedVersion: 1, UpdateInput: &UpdateInput{}}, {Operation: "delete", MemoryID: id, ExpectedVersion: 1}},
		"unknown":   {{Operation: "restore"}},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &lifecycleRepo{}
			projects := &countingProjectStub{}
			service := NewService(projects, repo, clockStub{}, &idsStub{})
			if _, err := service.Batch(context.Background(), operations); ErrorCode(err) != CodeValidationError {
				t.Fatalf("error=%v", err)
			}
			if projects.calls != 0 || repo.mutations != 0 || len(repo.revisions) != 0 {
				t.Fatalf("calls=%d repo=%#v", projects.calls, repo)
			}
		})
	}
	tooMany := make([]BatchOperation, 101)
	for index := range tooMany {
		input := CreateInput{}
		tooMany[index] = BatchOperation{Operation: "create", CreateInput: &input}
	}
	service := NewService(&countingProjectStub{}, &lifecycleRepo{}, clockStub{}, &idsStub{})
	if _, err := service.Batch(context.Background(), tooMany); ErrorCode(err) != CodeValidationError {
		t.Fatalf("101 operations error=%v", err)
	}
	for _, count := range []int{1, 100} {
		operations := make([]BatchOperation, count)
		for index := range operations {
			input := CreateInput{}
			operations[index] = BatchOperation{Operation: "create", CreateInput: &input}
		}
		projects := &countingProjectStub{value: ProjectContext{ID: "project"}}
		service := NewService(projects, &lifecycleRepo{}, clockStub{}, &idsStub{})
		response, err := service.Batch(context.Background(), operations)
		if err != nil || response.Total != count || response.Failed != count || projects.calls != 1 {
			t.Fatalf("count=%d response=%#v calls=%d err=%v", count, response, projects.calls, err)
		}
	}
}
