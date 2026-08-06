package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgeluis594/happy-memory/internal/project"
)

func TestDoctorMissingDatabaseWritesOneJSONErrorAndCreatesNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
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
	if err := decoder.Decode(&map[string]any{}); err == nil {
		t.Fatal("more than one JSON document")
	}
	wantPath := filepath.Join(home, "Library", "Application Support", "happy-memory", "happy-memory.db")
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
	t.Setenv("HOME", t.TempDir())
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

type structuredError struct{ code string }

func (err structuredError) Error() string { return err.code }
