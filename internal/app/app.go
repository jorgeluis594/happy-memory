// Package app composes and runs the command-line application.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"time"

	"github.com/google/uuid"
	commandadapter "github.com/jorgeluis594/happy-memory/internal/adapters/cobra"
	gitadapter "github.com/jorgeluis594/happy-memory/internal/adapters/git"
	"github.com/jorgeluis594/happy-memory/internal/adapters/sqlite"
	"github.com/jorgeluis594/happy-memory/internal/project"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type uuidGenerator struct{}

func (uuidGenerator) New() string { return uuid.NewString() }

// Run composes and executes one CLI invocation.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	database, err := sqlite.Open(ctx)
	if err != nil {
		return writeFailure(stderr, project.NewError(project.CodeStoreError, errors.New("storage operation failed")))
	}
	migrations, subErr := fs.Sub(sqlite.EmbeddedMigrations, "migrations")
	if subErr != nil {
		_ = database.Close()
		return writeFailure(stderr, project.NewError(project.CodeStoreError, errors.New("storage operation failed")))
	}
	if err := sqlite.Migrate(ctx, database.SQL(), migrations); err != nil {
		_ = database.Close()
		return writeFailure(stderr, project.NewError(project.CodeStoreError, errors.New("storage operation failed")))
	}
	service := project.NewService(gitadapter.New(), sqlite.NewProjectRepository(database.SQL()), systemClock{}, uuidGenerator{})
	response, executeErr := commandadapter.Execute(ctx, args, service)
	closeErr := database.Close()
	if executeErr != nil {
		return writeFailure(stderr, executeErr)
	}
	if closeErr != nil {
		return writeFailure(stderr, project.NewError(project.CodeStoreError, errors.New("storage operation failed")))
	}
	if _, err := stdout.Write(response); err != nil {
		return 1
	}
	return 0
}

func writeFailure(output io.Writer, err error) int {
	code := project.Code(err)
	message := map[string]string{
		project.CodeGitRepositoryNotFound: "git repository not found",
		project.CodeProjectNotInitialized: "project is not initialized",
		project.CodeValidationError:       "invalid input",
		project.CodeStoreError:            "storage operation failed",
	}[code]
	if message == "" {
		code, message = project.CodeStoreError, "storage operation failed"
	}
	type errorBody struct {
		Code    string   `json:"code"`
		Message string   `json:"message"`
		Details struct{} `json:"details"`
	}
	type failure struct {
		OK    bool      `json:"ok"`
		Error errorBody `json:"error"`
	}
	payload := failure{OK: false, Error: errorBody{Code: code, Message: message}}
	_ = json.NewEncoder(output).Encode(payload)
	return 1
}
