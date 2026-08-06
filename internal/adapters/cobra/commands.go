// Package cobra adapts project use cases to the CLI contract.
package cobra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jorgeluis594/happy-memory/internal/diagnostic"
	"github.com/jorgeluis594/happy-memory/internal/memory"
	"github.com/jorgeluis594/happy-memory/internal/project"
	"github.com/jorgeluis594/happy-memory/internal/publicerror"
	"github.com/jorgeluis594/happy-memory/internal/search"
	spf13cobra "github.com/spf13/cobra"
)

type projectService interface {
	Initialize(context.Context, *string) (project.Project, error)
	ShowCurrent(context.Context) (project.Project, error)
	List(context.Context) ([]project.Project, error)
}

type memoryService interface {
	Batch(context.Context, []memory.BatchOperation) (memory.BatchResponse, error)
	Create(context.Context, memory.CreateInput) (memory.Memory, error)
	Update(context.Context, string, int, memory.UpdateInput) (memory.Memory, error)
	Delete(context.Context, string, int) (memory.Memory, error)
	Restore(context.Context, string, int, int) (memory.Memory, error)
	Get(context.Context, string, bool) (memory.Memory, error)
	List(context.Context, memory.ListFilter) ([]memory.Memory, error)
	History(context.Context, string) ([]memory.Revision, error)
	TagsList(context.Context) ([]memory.Tag, error)
	TagsSearch(context.Context, string, int) ([]memory.Tag, error)
}

type searchService interface {
	Search(context.Context, search.Input) (search.Response, error)
}

type diagnosticService interface {
	Doctor(context.Context) (diagnostic.Report, error)
}

// Execute parses args and returns a buffered successful response.
func Execute(ctx context.Context, args []string, service projectService) ([]byte, error) {
	return execute(ctx, args, nil, service, nil, nil, nil)
}

// ExecuteWithMemory parses all project and memory commands, including stdin input.
func ExecuteWithMemory(ctx context.Context, args []string, input io.Reader, projects projectService, memories memoryService) ([]byte, error) {
	return execute(ctx, args, input, projects, memories, nil, nil)
}

// ExecuteWithServices parses all commands with memory and search services.
func ExecuteWithServices(ctx context.Context, args []string, input io.Reader, projects projectService, memories memoryService, searches searchService) ([]byte, error) {
	return execute(ctx, args, input, projects, memories, searches, nil)
}

// ExecuteAll parses every command, including storage diagnostics.
func ExecuteAll(ctx context.Context, args []string, input io.Reader, projects projectService, memories memoryService, searches searchService, diagnostics diagnosticService) ([]byte, error) {
	return execute(ctx, args, input, projects, memories, searches, diagnostics)
}

func execute(ctx context.Context, args []string, input io.Reader, service projectService, memories memoryService, searches searchService, diagnostics diagnosticService) ([]byte, error) {
	var output bytes.Buffer
	root := &spf13cobra.Command{Use: "happy-memory", SilenceUsage: true, SilenceErrors: true, Args: invalidArgs, RunE: invalidCommand}
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs(args)
	root.SetHelpFunc(func(*spf13cobra.Command, []string) {})
	root.SetUsageFunc(func(*spf13cobra.Command) error { return errors.New("invalid input") })
	root.SetHelpCommand(&spf13cobra.Command{Use: "help", Args: invalidArgs, RunE: invalidCommand})

	if service != nil {
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
	}
	if memories != nil {
		addMemoryCommands(root, &output, input, memories)
	}
	if searches != nil {
		addSearchCommand(root, &output, searches)
	}
	if diagnostics != nil {
		root.AddCommand(&spf13cobra.Command{Use: "doctor", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
			report, err := diagnostics.Doctor(command.Context())
			if err != nil {
				return err
			}
			return writeJSON(&output, struct {
				OK   bool              `json:"ok"`
				Data diagnostic.Report `json:"data"`
			}{OK: true, Data: report})
		}})
	}

	if err := root.ExecuteContext(ctx); err != nil {
		var domainError *project.Error
		if errors.As(err, &domainError) {
			return nil, err
		}
		var memoryError *memory.Error
		if errors.As(err, &memoryError) {
			return nil, err
		}
		var diagnosticError *diagnostic.UnhealthyError
		if errors.As(err, &diagnosticError) {
			return nil, err
		}
		return nil, project.NewError(project.CodeValidationError, errors.New("invalid input"))
	}
	if output.Len() == 0 {
		return nil, project.NewError(project.CodeValidationError, errors.New("invalid input"))
	}
	return output.Bytes(), nil
}

func addSearchCommand(root *spf13cobra.Command, output *bytes.Buffer, service searchService) {
	var kind string
	var tags []string
	var minImportance, minConfidence, limit int
	command := &spf13cobra.Command{Use: "search <query>", Args: oneArg, RunE: func(command *spf13cobra.Command, args []string) error {
		response, err := service.Search(command.Context(), search.Input{Query: args[0], Type: kind, Tags: tags, MinImportance: minImportance, MinConfidence: minConfidence, Limit: limit})
		if err != nil {
			return err
		}
		results := make([]searchResultJSON, 0, len(response.Results))
		for _, result := range response.Results {
			names := make([]string, 0, len(result.Memory.Tags))
			for _, tag := range result.Memory.Tags {
				names = append(names, tag.Name)
			}
			results = append(results, searchResultJSON{ID: result.Memory.ID, Type: result.Memory.Type, Title: result.Memory.Title, Content: result.Memory.Content, Importance: result.Memory.Importance, Confidence: result.Memory.Confidence, Tags: names, Score: searchScoreJSON{Final: result.Score.Final, Text: result.Score.Text, Importance: result.Score.Importance, Confidence: result.Score.Confidence}})
		}
		return writeJSON(output, searchSuccess{OK: true, Data: searchData{RankingVersion: response.RankingVersion, Results: results}})
	}}
	command.Flags().StringVar(&kind, "type", "", "memory type")
	command.Flags().StringArrayVar(&tags, "tag", nil, "required tag (repeatable)")
	command.Flags().IntVar(&minImportance, "min-importance", 0, "minimum importance")
	command.Flags().IntVar(&minConfidence, "min-confidence", 0, "minimum confidence")
	command.Flags().IntVar(&limit, "limit", 10, "maximum results")
	root.AddCommand(command)
}

type searchScoreJSON struct {
	Final      float64 `json:"final"`
	Text       float64 `json:"text"`
	Importance float64 `json:"importance"`
	Confidence float64 `json:"confidence"`
}

type searchResultJSON struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Title      string          `json:"title"`
	Content    string          `json:"content"`
	Importance int             `json:"importance"`
	Confidence int             `json:"confidence"`
	Tags       []string        `json:"tags"`
	Score      searchScoreJSON `json:"score"`
}

type searchData struct {
	RankingVersion int                `json:"ranking_version"`
	Results        []searchResultJSON `json:"results"`
}

type searchSuccess struct {
	OK   bool       `json:"ok"`
	Data searchData `json:"data"`
}

func addMemoryCommands(root *spf13cobra.Command, output *bytes.Buffer, input io.Reader, service memoryService) {
	var batchInputSource string
	batchCommand := &spf13cobra.Command{Use: "batch", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
		if !command.Flags().Changed("input") || batchInputSource != "-" || input == nil {
			return errors.New("invalid input")
		}
		var wire batchInput
		if err := decodeStrict(input, &wire); err != nil {
			return errors.New("invalid input")
		}
		operations := make([]memory.BatchOperation, 0, len(wire.Operations))
		for _, raw := range wire.Operations {
			operation, err := decodeBatchOperation(raw)
			if err != nil {
				return errors.New("invalid input")
			}
			operations = append(operations, operation)
		}
		response, err := service.Batch(command.Context(), operations)
		if err != nil {
			return err
		}
		results := make([]batchResultJSON, 0, len(response.Results))
		for _, result := range response.Results {
			item := batchResultJSON{Index: result.Index, Operation: result.Operation, OK: result.Err == nil}
			if result.Err == nil {
				value := toMemoryJSON(result.Memory)
				item.Data = &value
			} else {
				value := publicerror.From(result.Err)
				item.Error = &value
			}
			results = append(results, item)
		}
		return writeJSON(output, batchSuccess{OK: true, Data: batchData{Summary: batchSummary{Total: response.Total, Succeeded: response.Succeeded, Failed: response.Failed}, Results: results}})
	}}
	batchCommand.Flags().StringVar(&batchInputSource, "input", "", "read one JSON object from stdin")

	var inputSource string
	createCommand := &spf13cobra.Command{Use: "create", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
		if !command.Flags().Changed("input") || inputSource != "-" || input == nil {
			return errors.New("invalid input")
		}
		var value memory.CreateInput
		if err := decodeStrict(input, &value); err != nil {
			return errors.New("invalid input")
		}
		created, err := service.Create(command.Context(), value)
		if err != nil {
			return err
		}
		return writeJSON(output, memorySuccess{OK: true, Data: toMemoryJSON(created)})
	}}
	createCommand.Flags().StringVar(&inputSource, "input", "", "read one JSON object from stdin")
	var includeDeletedGet bool
	getCommand := &spf13cobra.Command{Use: "get <memory-id>", Args: func(_ *spf13cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("invalid input")
		}
		return nil
	}, RunE: func(command *spf13cobra.Command, args []string) error {
		value, err := service.Get(command.Context(), args[0], includeDeletedGet)
		if err != nil {
			return err
		}
		return writeJSON(output, memorySuccess{OK: true, Data: toMemoryJSON(value)})
	}}
	getCommand.Flags().BoolVar(&includeDeletedGet, "include-deleted", false, "include a deleted memory")
	var kind string
	var tags []string
	var minImportance, minConfidence int
	var includeDeletedList bool
	listCommand := &spf13cobra.Command{Use: "list", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
		values, err := service.List(command.Context(), memory.ListFilter{Type: kind, Tags: tags, MinImportance: minImportance, MinConfidence: minConfidence, IncludeDeleted: includeDeletedList})
		if err != nil {
			return err
		}
		items := make([]memoryJSON, 0, len(values))
		for _, value := range values {
			items = append(items, toMemoryJSON(value))
		}
		return writeJSON(output, memoryListSuccess{OK: true, Data: memoryListData{Memories: items}})
	}}
	listCommand.Flags().StringVar(&kind, "type", "", "memory type")
	listCommand.Flags().StringArrayVar(&tags, "tag", nil, "required tag (repeatable)")
	listCommand.Flags().IntVar(&minImportance, "min-importance", 0, "minimum importance")
	listCommand.Flags().IntVar(&minConfidence, "min-confidence", 0, "minimum confidence")
	listCommand.Flags().BoolVar(&includeDeletedList, "include-deleted", false, "include deleted memories")

	var updateInput string
	var updateExpected int
	updateCommand := &spf13cobra.Command{Use: "update <memory-id>", Args: oneArg, RunE: func(command *spf13cobra.Command, args []string) error {
		if !command.Flags().Changed("expected-version") || updateExpected < 1 || !command.Flags().Changed("input") || updateInput != "-" || input == nil {
			return errors.New("invalid input")
		}
		var patch memory.UpdateInput
		if err := decodeStrict(input, &patch); err != nil {
			return errors.New("invalid input")
		}
		value, err := service.Update(command.Context(), args[0], updateExpected, patch)
		if err != nil {
			return err
		}
		return writeJSON(output, memorySuccess{OK: true, Data: toMemoryJSON(value)})
	}}
	updateCommand.Flags().IntVar(&updateExpected, "expected-version", 0, "current memory version")
	updateCommand.Flags().StringVar(&updateInput, "input", "", "read one JSON object from stdin")

	var deleteExpected int
	deleteCommand := &spf13cobra.Command{Use: "delete <memory-id>", Args: oneArg, RunE: func(command *spf13cobra.Command, args []string) error {
		if !command.Flags().Changed("expected-version") || deleteExpected < 1 {
			return errors.New("invalid input")
		}
		value, err := service.Delete(command.Context(), args[0], deleteExpected)
		if err != nil {
			return err
		}
		return writeJSON(output, memorySuccess{OK: true, Data: toMemoryJSON(value)})
	}}
	deleteCommand.Flags().IntVar(&deleteExpected, "expected-version", 0, "current memory version")

	var restoreVersion, restoreExpected int
	restoreCommand := &spf13cobra.Command{Use: "restore <memory-id>", Args: oneArg, RunE: func(command *spf13cobra.Command, args []string) error {
		if !command.Flags().Changed("version") || restoreVersion < 1 || !command.Flags().Changed("expected-version") || restoreExpected < 1 {
			return errors.New("invalid input")
		}
		value, err := service.Restore(command.Context(), args[0], restoreVersion, restoreExpected)
		if err != nil {
			return err
		}
		return writeJSON(output, memorySuccess{OK: true, Data: toMemoryJSON(value)})
	}}
	restoreCommand.Flags().IntVar(&restoreVersion, "version", 0, "revision version to restore")
	restoreCommand.Flags().IntVar(&restoreExpected, "expected-version", 0, "current memory version")

	historyCommand := &spf13cobra.Command{Use: "history <memory-id>", Args: oneArg, RunE: func(command *spf13cobra.Command, args []string) error {
		values, err := service.History(command.Context(), args[0])
		if err != nil {
			return err
		}
		items := make([]revisionJSON, 0, len(values))
		for _, value := range values {
			item, convertErr := toRevisionJSON(value)
			if convertErr != nil {
				return convertErr
			}
			items = append(items, item)
		}
		return writeJSON(output, historySuccess{OK: true, Data: historyData{Revisions: items}})
	}}
	tagsCommand := &spf13cobra.Command{Use: "tags", Args: invalidArgs, RunE: invalidCommand}
	var tagsSearchLimit int
	tagsSearchCommand := &spf13cobra.Command{Use: "search <query>", Args: oneArg, RunE: func(command *spf13cobra.Command, args []string) error {
		values, err := service.TagsSearch(command.Context(), args[0], tagsSearchLimit)
		if err != nil {
			return err
		}
		return writeJSON(output, tagListSuccess{OK: true, Data: tagListData{Tags: toVocabularyJSON(values)}})
	}}
	tagsSearchCommand.Flags().IntVar(&tagsSearchLimit, "limit", 10, "maximum results")
	tagsCommand.AddCommand(
		&spf13cobra.Command{Use: "list", Args: invalidArgs, RunE: func(command *spf13cobra.Command, _ []string) error {
			values, err := service.TagsList(command.Context())
			if err != nil {
				return err
			}
			return writeJSON(output, tagListSuccess{OK: true, Data: tagListData{Tags: toVocabularyJSON(values)}})
		}},
		tagsSearchCommand,
	)
	root.AddCommand(batchCommand, createCommand, updateCommand, deleteCommand, restoreCommand, getCommand, listCommand, historyCommand, tagsCommand)
}

type batchInput struct {
	Operations []json.RawMessage `json:"operations"`
}

type batchSummary struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

type batchResultJSON struct {
	Index     int                `json:"index"`
	Operation string             `json:"operation"`
	OK        bool               `json:"ok"`
	Data      *memoryJSON        `json:"data,omitempty"`
	Error     *publicerror.Error `json:"error,omitempty"`
}

type batchData struct {
	Summary batchSummary      `json:"summary"`
	Results []batchResultJSON `json:"results"`
}

type batchSuccess struct {
	OK   bool      `json:"ok"`
	Data batchData `json:"data"`
}

func decodeBatchOperation(raw json.RawMessage) (memory.BatchOperation, error) {
	var header map[string]json.RawMessage
	if err := json.Unmarshal(raw, &header); err != nil || header == nil {
		return memory.BatchOperation{}, errors.New("invalid input")
	}
	var operation string
	if err := json.Unmarshal(header["operation"], &operation); err != nil {
		return memory.BatchOperation{}, errors.New("invalid input")
	}
	switch operation {
	case "create":
		if !exactKeys(header, "operation", "input") {
			return memory.BatchOperation{}, errors.New("invalid input")
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(header["input"], &object) != nil || object == nil {
			return memory.BatchOperation{}, errors.New("invalid input")
		}
		var input memory.CreateInput
		if err := decodeStrict(bytes.NewReader(header["input"]), &input); err != nil {
			return memory.BatchOperation{}, err
		}
		return memory.BatchOperation{Operation: operation, CreateInput: &input}, nil
	case "update":
		if !exactKeys(header, "operation", "memory_id", "expected_version", "input") {
			return memory.BatchOperation{}, errors.New("invalid input")
		}
		var value struct {
			MemoryID        string             `json:"memory_id"`
			ExpectedVersion int                `json:"expected_version"`
			Input           memory.UpdateInput `json:"input"`
		}
		if json.Unmarshal(header["memory_id"], &value.MemoryID) != nil || json.Unmarshal(header["expected_version"], &value.ExpectedVersion) != nil || json.Unmarshal(header["input"], &value.Input) != nil {
			return memory.BatchOperation{}, errors.New("invalid input")
		}
		return memory.BatchOperation{Operation: operation, MemoryID: value.MemoryID, ExpectedVersion: value.ExpectedVersion, UpdateInput: &value.Input}, nil
	case "delete":
		if !exactKeys(header, "operation", "memory_id", "expected_version") {
			return memory.BatchOperation{}, errors.New("invalid input")
		}
		var memoryID string
		var expected int
		if json.Unmarshal(header["memory_id"], &memoryID) != nil || json.Unmarshal(header["expected_version"], &expected) != nil {
			return memory.BatchOperation{}, errors.New("invalid input")
		}
		return memory.BatchOperation{Operation: operation, MemoryID: memoryID, ExpectedVersion: expected}, nil
	default:
		return memory.BatchOperation{}, errors.New("invalid input")
	}
}

func exactKeys(values map[string]json.RawMessage, keys ...string) bool {
	if len(values) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := values[key]; !ok {
			return false
		}
	}
	return true
}

type tagJSON struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	NormalizedName string  `json:"normalized_name"`
	Description    *string `json:"description"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}
type vocabularyTagJSON struct {
	tagJSON
	ActiveMemoryCount int `json:"active_memory_count"`
}
type tagListData struct {
	Tags []vocabularyTagJSON `json:"tags"`
}
type tagListSuccess struct {
	OK   bool        `json:"ok"`
	Data tagListData `json:"data"`
}
type memoryJSON struct {
	ID          string          `json:"id"`
	Version     int             `json:"version"`
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Content     string          `json:"content"`
	Importance  int             `json:"importance"`
	Confidence  int             `json:"confidence"`
	Attributes  json.RawMessage `json:"attributes"`
	Tags        []tagJSON       `json:"tags"`
	ContentHash string          `json:"content_hash"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	DeletedAt   *string         `json:"deleted_at"`
}
type memorySuccess struct {
	OK   bool       `json:"ok"`
	Data memoryJSON `json:"data"`
}
type memoryListData struct {
	Memories []memoryJSON `json:"memories"`
}
type memoryListSuccess struct {
	OK   bool           `json:"ok"`
	Data memoryListData `json:"data"`
}
type revisionJSON struct {
	Version      int        `json:"version"`
	Operation    string     `json:"operation"`
	AgentName    *string    `json:"agent_name"`
	AgentRole    string     `json:"agent_role"`
	WorktreeRoot string     `json:"worktree_root"`
	CreatedAt    string     `json:"created_at"`
	Snapshot     memoryJSON `json:"snapshot"`
}
type historyData struct {
	Revisions []revisionJSON `json:"revisions"`
}
type historySuccess struct {
	OK   bool        `json:"ok"`
	Data historyData `json:"data"`
}

func toMemoryJSON(value memory.Memory) memoryJSON {
	tags := make([]tagJSON, 0, len(value.Tags))
	for _, tag := range value.Tags {
		tags = append(tags, toTagJSON(tag))
	}
	attributes := value.Attributes
	if len(attributes) == 0 {
		attributes = json.RawMessage("null")
	}
	var deleted *string
	if value.DeletedAt != nil {
		formatted := value.DeletedAt.UTC().Format(time.RFC3339)
		deleted = &formatted
	}
	return memoryJSON{ID: value.ID, Version: value.Version, Type: value.Type, Title: value.Title, Content: value.Content, Importance: value.Importance, Confidence: value.Confidence, Attributes: attributes, Tags: tags, ContentHash: value.ContentHash, CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: value.UpdatedAt.UTC().Format(time.RFC3339), DeletedAt: deleted}
}
func toTagJSON(tag memory.Tag) tagJSON {
	return tagJSON{ID: tag.ID, Name: tag.Name, NormalizedName: tag.NormalizedName, Description: tag.Description, CreatedAt: tag.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: tag.UpdatedAt.UTC().Format(time.RFC3339)}
}
func toVocabularyJSON(values []memory.Tag) []vocabularyTagJSON {
	tags := make([]vocabularyTagJSON, 0, len(values))
	for _, tag := range values {
		tags = append(tags, vocabularyTagJSON{tagJSON: toTagJSON(tag), ActiveMemoryCount: tag.ActiveMemoryCount})
	}
	return tags
}
func toRevisionJSON(value memory.Revision) (revisionJSON, error) {
	snapshot, err := memory.FromSnapshot(value.Snapshot)
	if err != nil {
		return revisionJSON{}, memory.NewError(memory.CodeStoreError, err)
	}
	return revisionJSON{Version: value.Version, Operation: value.Operation, AgentName: value.AgentName, AgentRole: value.AgentRole, WorktreeRoot: value.WorktreeRoot, CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339), Snapshot: toMemoryJSON(snapshot)}, nil
}

func decodeStrict(input io.Reader, value any) error {
	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("invalid input")
	}
	return nil
}
func oneArg(_ *spf13cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid input")
	}
	return nil
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
