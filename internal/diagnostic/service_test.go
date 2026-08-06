package diagnostic

import (
	"context"
	"errors"
	"testing"
)

type checkerStub struct{ checks []Check }

func (stub checkerStub) Check(context.Context, string) ([]Check, error) { return stub.checks, nil }

func TestDoctorReturnsHealthyAndUnhealthyReports(t *testing.T) {
	names := []string{"database_access", "schema_compatibility", "sqlite_integrity", "foreign_keys", "fts5", "fts_index_consistency"}
	checks := make([]Check, len(names))
	for i, name := range names {
		checks[i] = Check{Name: name, OK: true, Message: "ok", Details: map[string]any{}}
	}
	report, err := NewService("/database", checkerStub{checks: checks}).Doctor(context.Background())
	if err != nil || !report.Healthy || len(report.Checks) != 6 {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	checks[2].OK = false
	report, err = NewService("/database", checkerStub{checks: checks}).Doctor(context.Background())
	var unhealthy *UnhealthyError
	if !errors.As(err, &unhealthy) || report.Healthy || unhealthy.Report.DatabasePath != "/database" {
		t.Fatalf("report=%#v err=%v", report, err)
	}
}
