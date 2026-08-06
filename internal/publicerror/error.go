// Package publicerror maps internal errors to the stable application contract.
package publicerror

import (
	"errors"
	"strings"

	"github.com/jorgeluis594/happy-memory/internal/diagnostic"
	"github.com/jorgeluis594/happy-memory/internal/memory"
	"github.com/jorgeluis594/happy-memory/internal/project"
)

// StoreBusyCode identifies exhausted bounded SQLite retries.
const StoreBusyCode = "STORE_BUSY"

// Error is the only error shape exposed by the application boundary.
type Error struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

// From converts any internal error to a safe, stable public error.
func From(err error) Error {
	var unhealthy *diagnostic.UnhealthyError
	if errors.As(err, &unhealthy) {
		return Error{Code: project.CodeStoreError, Message: "storage diagnostics failed", Details: unhealthy.Details()}
	}
	code, details := project.Code(err), map[string]any{}
	var memoryError *memory.Error
	if errors.As(err, &memoryError) {
		code, details = memoryError.Code, memory.ErrorDetails(err)
	}
	messages := map[string]string{
		project.CodeGitRepositoryNotFound: "git repository not found",
		project.CodeProjectNotInitialized: "project is not initialized",
		project.CodeValidationError:       "invalid input",
		project.CodeStoreError:            "storage operation failed",
		memory.CodeNotFound:               "memory not found",
		memory.CodeDuplicate:              "duplicate memory",
		memory.CodeVersionConflict:        "version conflict",
		StoreBusyCode:                     "storage is busy",
	}
	message, ok := messages[code]
	if !ok {
		code, message = project.CodeStoreError, "storage operation failed"
	}
	if code == project.CodeStoreError {
		if cause := rootCause(err); cause != "" {
			details["cause"] = cause
		}
	}
	return Error{Code: code, Message: message, Details: details}
}

func rootCause(err error) string {
	var cause error
	for next := errors.Unwrap(err); next != nil; next = errors.Unwrap(next) {
		cause = next
	}
	if cause == nil {
		return ""
	}
	return strings.TrimSpace(cause.Error())
}
