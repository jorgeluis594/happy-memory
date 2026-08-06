package app

import (
	"errors"

	"github.com/jorgeluis594/happy-memory/internal/diagnostic"
	"github.com/jorgeluis594/happy-memory/internal/memory"
	"github.com/jorgeluis594/happy-memory/internal/project"
)

const storeBusyCode = "STORE_BUSY"

// PublicError is the only error shape exposed by the application boundary.
type PublicError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func publicError(err error) PublicError {
	var unhealthy *diagnostic.UnhealthyError
	if errors.As(err, &unhealthy) {
		return PublicError{Code: project.CodeStoreError, Message: "storage diagnostics failed", Details: unhealthy.Details()}
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
		storeBusyCode:                     "storage is busy",
	}
	message, ok := messages[code]
	if !ok {
		code, message = project.CodeStoreError, "storage operation failed"
	}
	return PublicError{Code: code, Message: message, Details: details}
}
