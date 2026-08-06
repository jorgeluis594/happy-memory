package publicerror

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jorgeluis594/happy-memory/internal/memory"
)

func TestStoreErrorIncludesRootCause(t *testing.T) {
	err := memory.NewError(memory.CodeStoreError, fmt.Errorf("storage operation failed: %w", errors.New("NOT NULL constraint failed: memories.title")))

	public := From(err)

	if public.Code != memory.CodeStoreError || public.Message != "storage operation failed" {
		t.Fatalf("error=%#v", public)
	}
	if public.Details["cause"] != "NOT NULL constraint failed: memories.title" {
		t.Fatalf("cause=%v", public.Details["cause"])
	}
}

func TestNonStoreErrorDoesNotIncludeCause(t *testing.T) {
	public := From(memory.NewError(memory.CodeNotFound, errors.New("internal lookup failed")))

	if public.Code != memory.CodeNotFound || public.Message != "memory not found" || len(public.Details) != 0 {
		t.Fatalf("error=%#v", public)
	}
}
