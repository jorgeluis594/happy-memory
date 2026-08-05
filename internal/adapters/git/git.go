// Package git adapts the Git executable to project identity ports.
package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jorgeluis594/happy-memory/internal/project"
)

const identityKey = "happy-memory.project-id"

// Adapter stores identity in common Git configuration.
type Adapter struct{}

// New creates an Adapter.
func New() *Adapter { return &Adapter{} }

// Resolve finds the normalized common Git directory.
func (adapter *Adapter) Resolve(ctx context.Context) (project.GitContext, error) {
	output, err := exec.CommandContext(ctx, "git", "rev-parse", "--git-common-dir").Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return project.GitContext{}, project.NewError(project.CodeGitRepositoryNotFound, errors.New("git repository not found"))
		}
		return project.GitContext{}, storeError(err)
	}
	commonDir := strings.TrimSpace(string(output))
	abs, err := filepath.Abs(commonDir)
	if err != nil {
		return project.GitContext{}, storeError(err)
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return project.GitContext{}, storeError(err)
	}
	return project.GitContext{CommonDir: filepath.Clean(abs)}, nil
}

// ReadID reads and validates the configured UUID.
func (adapter *Adapter) ReadID(ctx context.Context, gitContext project.GitContext) (string, bool, error) {
	output, err := exec.CommandContext(ctx, "git", "config", "--file", filepath.Join(gitContext.CommonDir, "config"), "--get-all", identityKey).Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return "", false, nil
		}
		return "", false, storeError(err)
	}
	values := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(values) != 1 {
		return "", false, storeError(errors.New("multiple project identities"))
	}
	parsed, err := uuid.Parse(values[0])
	if err != nil || parsed.String() != values[0] {
		return "", false, storeError(errors.New("invalid project identity"))
	}
	return values[0], true, nil
}

// WriteID stores a UUID in common Git configuration.
func (adapter *Adapter) WriteID(ctx context.Context, gitContext project.GitContext, id string) error {
	if output, err := exec.CommandContext(ctx, "git", "config", "--file", filepath.Join(gitContext.CommonDir, "config"), identityKey, id).CombinedOutput(); err != nil {
		return storeError(fmt.Errorf("write git identity: %w: %s", err, strings.TrimSpace(string(output))))
	}
	return nil
}

// InferName derives a name from the main or bare repository directory.
func (adapter *Adapter) InferName(ctx context.Context, gitContext project.GitContext) (string, error) {
	output, err := exec.CommandContext(ctx, "git", "--git-dir", gitContext.CommonDir, "rev-parse", "--is-bare-repository").Output()
	if err != nil {
		return "", storeError(err)
	}
	if strings.TrimSpace(string(output)) == "true" {
		return strings.TrimSuffix(filepath.Base(gitContext.CommonDir), ".git"), nil
	}
	return filepath.Base(filepath.Dir(gitContext.CommonDir)), nil
}

func storeError(err error) error {
	return project.NewError(project.CodeStoreError, fmt.Errorf("storage operation failed: %w", err))
}
