package project

import (
	"context"
	"testing"
	"time"
)

func TestInitializeUsesResolvedGitContext(t *testing.T) {
	git := &gitStub{inferredName: " inferred "}
	repo := &repositoryStub{}
	stamp := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	service := NewService(git, repo, clockStub{stamp}, idStub{"project-id"})
	gitContext := GitContext{CommonDir: "/repo/.git", WorktreeRoot: "/repo/worktree"}

	got, err := service.Initialize(context.Background(), gitContext, nil)
	if err != nil {
		t.Fatal(err)
	}
	if git.readContext != gitContext || git.writeContext != gitContext || git.inferContext != gitContext {
		t.Fatalf("Git contexts: read=%#v write=%#v infer=%#v", git.readContext, git.writeContext, git.inferContext)
	}
	if got.ID != "project-id" || got.Name != "inferred" || got.LastKnownGitCommonDir != gitContext.CommonDir {
		t.Fatalf("project=%#v", got)
	}
}

func TestShowCurrentUsesResolvedGitContext(t *testing.T) {
	gitContext := GitContext{CommonDir: "/repo/.git", WorktreeRoot: "/repo/worktree"}
	git := &gitStub{id: "project-id", found: true}
	repo := &repositoryStub{project: &Project{ID: "project-id", Name: "name", LastKnownGitCommonDir: gitContext.CommonDir}}
	service := NewService(git, repo, clockStub{}, idStub{})

	got, err := service.ShowCurrent(context.Background(), gitContext)
	if err != nil {
		t.Fatal(err)
	}
	if git.readContext != gitContext || got.ID != "project-id" || repo.updateCalls != 0 {
		t.Fatalf("context=%#v project=%#v updates=%d", git.readContext, got, repo.updateCalls)
	}
}

func TestShowCurrentReconcilesResolvedCommonDir(t *testing.T) {
	stamp := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	gitContext := GitContext{CommonDir: "/moved/.git", WorktreeRoot: "/moved"}
	git := &gitStub{id: "project-id", found: true}
	repo := &repositoryStub{project: &Project{ID: "project-id", Name: "name", LastKnownGitCommonDir: "/old/.git"}}
	service := NewService(git, repo, clockStub{stamp}, idStub{})

	got, err := service.ShowCurrent(context.Background(), gitContext)
	if err != nil {
		t.Fatal(err)
	}
	if repo.updateCalls != 1 || repo.updatedPath != gitContext.CommonDir || !repo.updatedAt.Equal(stamp) {
		t.Fatalf("updates=%d path=%q at=%v", repo.updateCalls, repo.updatedPath, repo.updatedAt)
	}
	if got.LastKnownGitCommonDir != gitContext.CommonDir {
		t.Fatalf("project=%#v", got)
	}
}

type gitStub struct {
	id, inferredName                        string
	found                                   bool
	readContext, writeContext, inferContext GitContext
}

func (stub *gitStub) ReadID(_ context.Context, gitContext GitContext) (string, bool, error) {
	stub.readContext = gitContext
	return stub.id, stub.found, nil
}
func (stub *gitStub) WriteID(_ context.Context, gitContext GitContext, id string) error {
	stub.writeContext, stub.id, stub.found = gitContext, id, true
	return nil
}
func (stub *gitStub) InferName(_ context.Context, gitContext GitContext) (string, error) {
	stub.inferContext = gitContext
	return stub.inferredName, nil
}

type repositoryStub struct {
	project     *Project
	updateCalls int
	updatedPath string
	updatedAt   time.Time
}

func (stub *repositoryStub) Find(context.Context, string) (*Project, error) { return stub.project, nil }
func (stub *repositoryStub) Reconcile(_ context.Context, id, name, path string, at time.Time) (Project, error) {
	value := Project{ID: id, Name: name, LastKnownGitCommonDir: path, CreatedAt: at, UpdatedAt: at}
	stub.project = &value
	return value, nil
}
func (stub *repositoryStub) UpdatePath(_ context.Context, _ string, path string, at time.Time) (Project, error) {
	stub.updateCalls++
	stub.updatedPath, stub.updatedAt = path, at
	stub.project.LastKnownGitCommonDir, stub.project.UpdatedAt = path, at
	return *stub.project, nil
}
func (stub *repositoryStub) List(context.Context) ([]Project, error) { return nil, nil }

type clockStub struct{ value time.Time }

func (stub clockStub) Now() time.Time { return stub.value }

type idStub struct{ value string }

func (stub idStub) New() string { return stub.value }
