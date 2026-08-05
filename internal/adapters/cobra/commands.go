// Package cobra adapts project use cases to the CLI contract.
package cobra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jorgeluis594/happy-memory/internal/project"
	spf13cobra "github.com/spf13/cobra"
)

type projectService interface {
	Initialize(context.Context, *string) (project.Project, error)
	ShowCurrent(context.Context) (project.Project, error)
	List(context.Context) ([]project.Project, error)
}

// Execute parses args and returns a buffered successful response.
func Execute(ctx context.Context, args []string, service projectService) ([]byte, error) {
	var output bytes.Buffer
	root := &spf13cobra.Command{Use: "happy-memory", SilenceUsage: true, SilenceErrors: true, Args: invalidArgs, RunE: invalidCommand}
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs(args)

	var name string
	initCommand := &spf13cobra.Command{Use: "init", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
		var requested *string
		if command.Flags().Changed("name") {
			requested = &name
		}
		value, err := service.Initialize(command.Context(), requested)
		if err != nil {
			return err
		}
		return writeJSON(&output, successProject(value))
	}}
	initCommand.Flags().StringVar(&name, "name", "", "project name")

	projectCommand := &spf13cobra.Command{Use: "project", Args: invalidArgs, RunE: invalidCommand}
	projectCommand.AddCommand(&spf13cobra.Command{Use: "show", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
		value, err := service.ShowCurrent(command.Context())
		if err != nil {
			return err
		}
		return writeJSON(&output, successProject(value))
	}})
	projectsCommand := &spf13cobra.Command{Use: "projects", Args: invalidArgs, RunE: invalidCommand}
	projectsCommand.AddCommand(&spf13cobra.Command{Use: "list", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
		values, err := service.List(command.Context())
		if err != nil {
			return err
		}
		items := make([]projectJSON, 0, len(values))
		for _, value := range values {
			items = append(items, toJSON(value))
		}
		return writeJSON(&output, listSuccess{OK: true, Data: projectList{Projects: items}})
	}})
	root.AddCommand(initCommand, projectCommand, projectsCommand)

	if err := root.ExecuteContext(ctx); err != nil {
		var domainError *project.Error
		if errors.As(err, &domainError) {
			return nil, err
		}
		return nil, project.NewError(project.CodeValidationError, errors.New("invalid input"))
	}
	return output.Bytes(), nil
}

func invalidArgs(_ *spf13cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("invalid input")
	}
	return nil
}
func invalidCommand(_ *spf13cobra.Command, _ []string) error { return errors.New("invalid input") }

type projectJSON struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	LastKnownGitCommonDir string `json:"last_known_git_common_dir"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
}

func toJSON(value project.Project) projectJSON {
	return projectJSON{ID: value.ID, Name: value.Name, LastKnownGitCommonDir: value.LastKnownGitCommonDir, CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: value.UpdatedAt.UTC().Format(time.RFC3339)}
}

type projectSuccess struct {
	OK   bool        `json:"ok"`
	Data projectJSON `json:"data"`
}

type projectList struct {
	Projects []projectJSON `json:"projects"`
}

type listSuccess struct {
	OK   bool        `json:"ok"`
	Data projectList `json:"data"`
}

func successProject(value project.Project) any { return projectSuccess{OK: true, Data: toJSON(value)} }
func writeJSON(output *bytes.Buffer, value any) error {
	if err := json.NewEncoder(output).Encode(value); err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	return nil
}
