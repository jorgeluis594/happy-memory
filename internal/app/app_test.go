package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorgeluis594/happy-memory/internal/project"
)

func TestDoctorMissingDatabaseWritesOneJSONErrorAndCreatesNothing(t *testing.T) {
	directory := t.TempDir()
	chdir(t, directory)
	var stdout, stderr bytes.Buffer
	if code := RunWithInput(context.Background(), []string{"doctor"}, bytes.NewReader(nil), &stdout, &stderr); code == 0 {
		t.Fatal("doctor exit code = 0")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout=%q", stdout.String())
	}
	var payload struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	decoder := json.NewDecoder(&stderr)
	if err := decoder.Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.OK || payload.Error.Code != "STORE_ERROR" || payload.Error.Message != "storage diagnostics failed" {
		t.Fatalf("payload=%#v", payload)
	}
	if payload.Error.Details["database_path"] != filepath.Join(".happy-memory", "memory.db") {
		t.Fatalf("database_path=%v", payload.Error.Details["database_path"])
	}
	if err := decoder.Decode(&map[string]any{}); err == nil {
		t.Fatal("more than one JSON document")
	}
	wantPath := filepath.Join(directory, ".happy-memory", "memory.db")
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Fatalf("database was created: %v", err)
	}
}

func TestPublicBusyErrorIsStable(t *testing.T) {
	err := publicError(project.NewError(storeBusyCode, structuredError{code: storeBusyCode}))
	if err.Code != storeBusyCode || err.Message != "storage is busy" || len(err.Details) != 0 {
		t.Fatalf("error=%#v", err)
	}
}

func TestBatchIntegrationAllowsPartialSuccessAndPersistsSuccessfulItems(t *testing.T) {
	repository := initRepository(t)
	chdir(t, repository)
	run := func(args []string, input string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := RunWithInput(context.Background(), args, bytes.NewBufferString(input), &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	if code, _, stderr := run([]string{"init", "--name", "batch-integration"}, ""); code != 0 {
		t.Fatalf("init code=%d stderr=%s", code, stderr)
	}
	create := `{"type":"fact","title":"original","content":"content","importance":3,"confidence":4,"tags":["sqlite"]}`
	code, stdout, stderr := run([]string{"create", "--input", "-"}, create)
	if code != 0 || stderr != "" {
		t.Fatalf("create code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var created struct {
		Data struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &created); err != nil {
		t.Fatal(err)
	}
	batch := `{"operations":[` +
		`{"operation":"update","memory_id":"` + created.Data.ID + `","expected_version":1,"input":{"title":"updated"}},` +
		`{"operation":"create","input":{"type":"fact","title":"updated","content":"content","importance":3,"confidence":4,"tags":[]}},` +
		`{"operation":"create","input":{"type":"decision","title":"second","content":"survives","importance":5,"confidence":5,"tags":["batch"]}}]}`
	code, stdout, stderr = run([]string{"batch", "--input", "-"}, batch)
	if code != 0 || stderr != "" {
		t.Fatalf("batch code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Summary struct {
				Total, Succeeded, Failed int
			} `json:"summary"`
			Results []struct {
				OK    bool `json:"ok"`
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK || response.Data.Summary.Total != 3 || response.Data.Summary.Succeeded != 2 || response.Data.Summary.Failed != 1 || response.Data.Results[1].Error.Code != "DUPLICATE_MEMORY" {
		t.Fatalf("response=%s", stdout)
	}
	code, stdout, stderr = run([]string{"get", created.Data.ID}, "")
	if code != 0 || stderr != "" {
		t.Fatalf("get code=%d stderr=%s", code, stderr)
	}
	var current struct {
		Data struct {
			Title   string `json:"title"`
			Version int    `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &current); err != nil {
		t.Fatal(err)
	}
	if current.Data.Title != "updated" || current.Data.Version != 2 {
		t.Fatalf("current=%s", stdout)
	}
}

func TestInitCreatesRepositoryDatabaseAndReusesIt(t *testing.T) {
	repository := initRepository(t)
	chdir(t, repository)
	run := func(args ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), args, &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	if code, _, stderr := run("init", "--name", "shared"); code != 0 {
		t.Fatalf("first init code=%d stderr=%s", code, stderr)
	}
	path := filepath.Join(repository, ".happy-memory", "memory.db")
	assertPermission(t, filepath.Dir(path), 0o700)
	assertPermission(t, path, 0o600)
	if code, _, stderr := run("init"); code != 0 {
		t.Fatalf("second init code=%d stderr=%s", code, stderr)
	}
	if code, stdout, stderr := run("project", "show"); code != 0 || !strings.Contains(stdout, `"name":"shared"`) {
		t.Fatalf("show code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestNormalCommandWithoutDatabaseDoesNotCreateIt(t *testing.T) {
	repository := initRepository(t)
	chdir(t, repository)
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"projects", "list"}, &stdout, &stderr); code == 0 {
		t.Fatal("projects list exit code = 0")
	}
	if !strings.Contains(stderr.String(), `"code":"PROJECT_NOT_INITIALIZED"`) {
		t.Fatalf("stderr=%s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(repository, ".happy-memory")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("storage directory was created: %v", err)
	}
}

func TestVersionWorksOutsideRepositoryAndCreatesNothing(t *testing.T) {
	directory := t.TempDir()
	chdir(t, directory)
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("version code=%d stderr=%s", code, stderr.String())
	}
	want := "{\"ok\":true,\"data\":{\"version\":\"dev\",\"commit\":\"unknown\",\"build_date\":\"unknown\"}}\n"
	if stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(directory, ".happy-memory")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("version created storage: %v", err)
	}
}

func TestInvalidInputDoesNotCreateDatabase(t *testing.T) {
	repository := initRepository(t)
	chdir(t, repository)
	for _, args := range [][]string{{"init", "extra"}, {"init", "--unknown"}, {"init", "--name", " "}} {
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), args, &stdout, &stderr); code == 0 {
			t.Fatalf("Run(%v) exit code = 0", args)
		}
		if _, err := os.Stat(filepath.Join(repository, ".happy-memory")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Run(%v) created storage: %v", args, err)
		}
	}
}

func TestLinkedWorktreeSharesDatabase(t *testing.T) {
	repository := initRepository(t)
	worktree := filepath.Join(t.TempDir(), "linked")
	runGit(t, repository, "worktree", "add", "-b", "linked-test", worktree)
	chdir(t, worktree)
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"init", "--name", "worktrees"}, &stdout, &stderr); code != 0 {
		t.Fatalf("init code=%d stderr=%s", code, stderr.String())
	}
	wantPath := filepath.Join(repository, ".happy-memory", "memory.db")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("shared database: %v", err)
	}
	if _, err := os.Stat(filepath.Join(worktree, ".happy-memory")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("worktree-local storage exists: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	input := bytes.NewBufferString(`{"type":"fact","title":"shared","content":"from worktree","importance":3,"confidence":4,"tags":[]}`)
	if code := RunWithInput(context.Background(), []string{"create", "--input", "-"}, input, &stdout, &stderr); code != 0 {
		t.Fatalf("create code=%d stderr=%s", code, stderr.String())
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	chdir(t, repository)
	stdout.Reset()
	stderr.Reset()
	if code := Run(context.Background(), []string{"get", created.Data.ID}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), `"content":"from worktree"`) {
		t.Fatalf("get code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestInitDoesNotTouchLegacyGlobalDatabase(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	legacy := filepath.Join(home, "Library", "Application Support", "happy-memory", "happy-memory.db")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("legacy-marker"), 0o600); err != nil {
		t.Fatal(err)
	}
	repository := initRepository(t)
	chdir(t, repository)
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"init"}, &stdout, &stderr); code != 0 {
		t.Fatalf("init code=%d stderr=%s", code, stderr.String())
	}
	contents, err := os.ReadFile(legacy)
	if err != nil || string(contents) != "legacy-marker" {
		t.Fatalf("legacy database changed: contents=%q err=%v", contents, err)
	}
}

func initRepository(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	runGit(t, directory, "init")
	runGit(t, directory, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "initial")
	return directory
}

func runGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

func chdir(t *testing.T, directory string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
}

func assertPermission(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode %q = %04o, want %04o", path, got, want)
	}
}

type structuredError struct{ code string }

func (err structuredError) Error() string { return err.code }
