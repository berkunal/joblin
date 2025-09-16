package lib

import (
	"context"
	"fmt"
	"os"

	"github.com/berkunal/joblin/src/models"
)

// CLIErrorHandler provides consistent error handling for CLI commands
type CLIErrorHandler struct {
	logger     *Logger
	jsonOutput bool
}

// NewCLIErrorHandler creates a new CLI error handler
func NewCLIErrorHandler(logger *Logger, jsonOutput bool) *CLIErrorHandler {
	return &CLIErrorHandler{
		logger:     logger,
		jsonOutput: jsonOutput,
	}
}

// HandleError processes and displays errors consistently
func (h *CLIErrorHandler) HandleError(ctx context.Context, err error, operation string) {
	if err == nil {
		return
	}

	// Add operation context if not already present
	ctx = WithOperationContext(ctx, operation)

	// Log the error with full context
	h.logger.WithContext(ctx).WithError(err).Error("Command failed")

	if h.jsonOutput {
		h.handleJSONError(err)
	} else {
		h.handleTextError(err)
	}
}

// HandleJobError handles job-specific errors with context
func (h *CLIErrorHandler) HandleJobError(ctx context.Context, err error, jobID, operation string) {
	if err == nil {
		return
	}

	// Add job and operation context
	ctx = WithJobContext(ctx, jobID, "", "")
	ctx = WithOperationContext(ctx, operation)

	h.HandleError(ctx, err, operation)
}

// HandleSuccess displays success messages consistently
func (h *CLIErrorHandler) HandleSuccess(ctx context.Context, message string, data interface{}) {
	if h.jsonOutput {
		h.handleJSONSuccess(message, data)
	} else {
		fmt.Println(message)
		if data != nil {
			// Could add verbose output here if needed
		}
	}
}

// WrapValidationError creates a validation error with context
func (h *CLIErrorHandler) WrapValidationError(err error, field string, value interface{}) error {
	return NewValidationError("FIELD_VALIDATION_FAILED",
		fmt.Sprintf("Validation failed for field '%s'", field)).
		WithCause(err).
		WithContext("field", field).
		WithContext("value", value).
		WithUserMessage(fmt.Sprintf("Invalid value for %s: %v", field, value))
}

// WrapJobError creates a job-specific error with context
func (h *CLIErrorHandler) WrapJobError(err error, job *models.Job, operation string) error {
	joblinErr := NewInternalError("JOB_OPERATION_FAILED",
		fmt.Sprintf("Job operation '%s' failed", operation)).
		WithCause(err).
		WithContext("job_id", job.ID).
		WithContext("job_name", job.Name).
		WithContext("operation", operation)

	if job.Namespace != "" {
		joblinErr.WithContext("namespace", job.Namespace)
	}

	return joblinErr
}

// RequireJobID validates that a job ID is provided
func (h *CLIErrorHandler) RequireJobID(jobID string) error {
	if jobID == "" {
		return NewValidationError("JOB_ID_REQUIRED",
			"Job ID is required but was not provided").
			WithUserMessage("Please provide a job ID")
	}
	return nil
}

// RequireScriptFile validates that a script file exists and is readable
func (h *CLIErrorHandler) RequireScriptFile(scriptPath string) error {
	if scriptPath == "" {
		return NewValidationError("SCRIPT_PATH_REQUIRED",
			"Script path is required but was not provided").
			WithUserMessage("Please provide a Python script file path")
	}

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return NewValidationError("SCRIPT_FILE_NOT_FOUND",
			"Script file does not exist").
			WithContext("script_path", scriptPath).
			WithUserMessage(fmt.Sprintf("Script file not found: %s", scriptPath))
	}

	return nil
}

// handleJSONError outputs structured error information as JSON
func (h *CLIErrorHandler) handleJSONError(err error) {
	errorResponse := map[string]interface{}{
		"success": false,
		"error":   err.Error(),
	}

	// Add structured error information if it's a JoblinError
	if joblinErr, ok := AsJoblinError(err); ok {
		errorResponse["error_details"] = map[string]interface{}{
			"type":         joblinErr.Type,
			"code":         joblinErr.Code,
			"message":      joblinErr.Message,
			"details":      joblinErr.Details,
			"context":      joblinErr.Context,
			"retryable":    joblinErr.Retryable,
			"user_message": joblinErr.GetUserMessage(),
		}
	}

	PrintJSON(errorResponse)
}

// handleTextError outputs human-readable error information
func (h *CLIErrorHandler) handleTextError(err error) {
	if joblinErr, ok := AsJoblinError(err); ok {
		// Use user-friendly message if available
		if userMsg := joblinErr.GetUserMessage(); userMsg != "" {
			fmt.Fprintf(os.Stderr, "Error: %s\n", userMsg)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", joblinErr.Message)
		}

		// Show additional details if available
		if joblinErr.Details != "" {
			fmt.Fprintf(os.Stderr, "Details: %s\n", joblinErr.Details)
		}

		// Show retry information
		if joblinErr.IsRetryable() {
			fmt.Fprintf(os.Stderr, "This operation can be retried.\n")
		}

		// Show context information in verbose mode
		if len(joblinErr.Context) > 0 {
			fmt.Fprintf(os.Stderr, "Context: %v\n", joblinErr.Context)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

// handleJSONSuccess outputs structured success information as JSON
func (h *CLIErrorHandler) handleJSONSuccess(message string, data interface{}) {
	response := map[string]interface{}{
		"success": true,
		"message": message,
	}

	if data != nil {
		response["data"] = data
	}

	PrintJSON(response)
}

// PrintJSON outputs JSON with proper formatting
func PrintJSON(data interface{}) {
	// This would normally use the same JSON encoder as the CLI
	// For now, using a simple implementation
	fmt.Printf("%+v\n", data)
}

// Context helpers for CLI operations

// WithCLIContext adds CLI-specific context
func WithCLIContext(ctx context.Context, command string, flags map[string]interface{}) context.Context {
	ctx = WithOperationContext(ctx, command)
	for key, value := range flags {
		ctx = context.WithValue(ctx, LogContext("flag_"+key), value)
	}
	return ctx
}

// WithJobContextCLI adds job context for CLI operations (using existing WithJobContext from logger.go)

// GetJobContext extracts job context from context
func GetJobContext(ctx context.Context) (jobID, jobName, namespace string) {
	jobID = GetContextValue(ctx, LogContextJobID)
	jobName = GetContextValue(ctx, LogContextJobName)
	namespace = GetContextValue(ctx, LogContextNamespace)
	return
}
