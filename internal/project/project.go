// Package project implements project identity use cases and ports.
package project

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Stable public error codes.
const (
	CodeGitRepositoryNotFound = "GIT_REPOSITORY_NOT_FOUND"
	CodeProjectNotInitialized = "PROJECT_NOT_INITIALIZED"
	CodeValidationError       = "VALIDATION_ERROR"
	CodeStoreError            = "STORE_ERROR"
)

// Error carries a stable public code and an underlying cause.
type Error struct {
	Code string
	Err  error
}

func (err *Error) Error() string { return err.Err.Error() }
func (err *Error) Unwrap() error { return err.Err }

// NewError creates a coded domain error.
func NewError(code string, err error) error { return &Error{Code: code, Err: err} }

// Code extracts a public error code.
func Code(err error) string {
	var domainError *Error
	if errors.As(err, &domainError) {
		return domainError.Code
	}
	return CodeStoreError
}

// Project is a cataloged Git project.
type Project struct {
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	LastKnownGitCommonDir string    `json:"last_known_git_common_dir"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// GitContext identifies a normalized common Git directory.
type GitContext struct {
	CommonDir    string
	WorktreeRoot string
}

// GitIdentity accesses project identity in Git configuration.
type GitIdentity interface {
	Resolve(context.Context) (GitContext, error)
	ReadID(context.Context, GitContext) (string, bool, error)
	WriteID(context.Context, GitContext, string) error
	InferName(context.Context, GitContext) (string, error)
}

// Repository persists the project catalog in the current repository database.
type Repository interface {
	Find(context.Context, string) (*Project, error)
	Reconcile(context.Context, string, string, string, time.Time) (Project, error)
	UpdatePath(context.Context, string, string, time.Time) (Project, error)
	List(context.Context) ([]Project, error)
}

// Clock supplies timestamps.
type Clock interface{ Now() time.Time }

// IDGenerator supplies project identifiers.
type IDGenerator interface{ New() string }

// Service implements project use cases.
type Service struct {
	git   GitIdentity
	repo  Repository
	clock Clock
	ids   IDGenerator
}

// NewService composes project use cases.
func NewService(git GitIdentity, repo Repository, clock Clock, ids IDGenerator) *Service {
	return &Service{git: git, repo: repo, clock: clock, ids: ids}
}

// Initialize creates or reconciles the current project.
func (service *Service) Initialize(ctx context.Context, requestedName *string) (Project, error) {
	gitContext, err := service.git.Resolve(ctx)
	if err != nil {
		return Project{}, err
	}
	id, found, err := service.git.ReadID(ctx, gitContext)
	if err != nil {
		return Project{}, err
	}
	if !found {
		id = service.ids.New()
		if writeErr := service.git.WriteID(ctx, gitContext, id); writeErr != nil {
			return Project{}, writeErr
		}
	}
	existing, err := service.repo.Find(ctx, id)
	if err != nil {
		return Project{}, err
	}
	name := ""
	if requestedName != nil {
		name = strings.TrimSpace(*requestedName)
		if name == "" {
			return Project{}, NewError(CodeValidationError, errors.New("invalid input"))
		}
	} else if existing != nil {
		name = existing.Name
	} else {
		name, err = service.git.InferName(ctx, gitContext)
		if err != nil {
			return Project{}, err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return Project{}, NewError(CodeValidationError, errors.New("invalid input"))
		}
	}
	return service.repo.Reconcile(ctx, id, name, gitContext.CommonDir, service.clock.Now().UTC())
}

// ShowCurrent returns the current initialized project without recreating it.
func (service *Service) ShowCurrent(ctx context.Context) (Project, error) {
	gitContext, err := service.git.Resolve(ctx)
	if err != nil {
		return Project{}, err
	}
	id, found, err := service.git.ReadID(ctx, gitContext)
	if err != nil {
		return Project{}, err
	}
	if !found {
		return Project{}, NewError(CodeProjectNotInitialized, errors.New("project is not initialized"))
	}
	current, err := service.repo.Find(ctx, id)
	if err != nil {
		return Project{}, err
	}
	if current == nil {
		return Project{}, NewError(CodeProjectNotInitialized, errors.New("project is not initialized"))
	}
	if current.LastKnownGitCommonDir != gitContext.CommonDir {
		return service.repo.UpdatePath(ctx, id, gitContext.CommonDir, service.clock.Now().UTC())
	}
	return *current, nil
}

// List returns all cataloged projects.
func (service *Service) List(ctx context.Context) ([]Project, error) { return service.repo.List(ctx) }
