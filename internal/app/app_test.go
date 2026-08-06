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

type structuredError struct{ code string }

func (err structuredError) Error() string { return err.code }
