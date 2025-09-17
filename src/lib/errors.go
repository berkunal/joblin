package lib

import (
	"fmt"
	"net/http"
)

// ErrorType represents different categories of errors
type ErrorType string

const (
	// ErrorTypeValidation indicates validation-related errors
	ErrorTypeValidation    ErrorType = "validation"
	// ErrorTypeConfiguration indicates configuration-related errors
	ErrorTypeConfiguration ErrorType = "configuration"
	// ErrorTypeNetwork indicates network-related errors
	ErrorTypeNetwork       ErrorType = "network"
	// ErrorTypeKubernetes indicates Kubernetes-related errors
	ErrorTypeKubernetes    ErrorType = "kubernetes"
	// ErrorTypeStorage indicates storage-related errors
	ErrorTypeStorage       ErrorType = "storage"
	// ErrorTypeNotification indicates notification-related errors
	ErrorTypeNotification  ErrorType = "notification"
	// ErrorTypeInternal indicates internal system errors
	ErrorTypeInternal      ErrorType = "internal"
	// ErrorTypeNotFound indicates resource not found errors
	ErrorTypeNotFound      ErrorType = "not_found"
	// ErrorTypeConflict indicates resource conflict errors
	ErrorTypeConflict      ErrorType = "conflict"
	// ErrorTypeTimeout indicates timeout-related errors
	ErrorTypeTimeout       ErrorType = "timeout"
	// ErrorTypeUnauthorized indicates authentication/authorization errors
	ErrorTypeUnauthorized  ErrorType = "unauthorized"
)

// JoblinError represents a structured error with context
type JoblinError struct {
	Type        ErrorType              `json:"type"`
	Code        string                 `json:"code"`
	Message     string                 `json:"message"`
	Details     string                 `json:"details,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Cause       error                  `json:"cause,omitempty"`
	HTTPStatus  int                    `json:"http_status,omitempty"`
	Retryable   bool                   `json:"retryable"`
	UserMessage string                 `json:"user_message,omitempty"`
	ExitCode    int                    `json:"exit_code,omitempty"`
}

// Error implements the error interface
func (e *JoblinError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s:%s] %s: %s", e.Type, e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Type, e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *JoblinError) Unwrap() error {
	return e.Cause
}

// WithContext adds context information to the error
func (e *JoblinError) WithContext(key string, value interface{}) *JoblinError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithCause sets the underlying cause error
func (e *JoblinError) WithCause(cause error) *JoblinError {
	e.Cause = cause
	return e
}

// WithDetails adds additional details to the error
func (e *JoblinError) WithDetails(details string) *JoblinError {
	e.Details = details
	return e
}

// WithUserMessage sets a user-friendly message
func (e *JoblinError) WithUserMessage(message string) *JoblinError {
	e.UserMessage = message
	return e
}

// IsRetryable indicates if the operation can be retried
func (e *JoblinError) IsRetryable() bool {
	return e.Retryable
}

// GetUserMessage returns a user-friendly error message
func (e *JoblinError) GetUserMessage() string {
	if e.UserMessage != "" {
		return e.UserMessage
	}
	return e.Message
}

// Error constructors for common error types

// NewValidationError creates a new validation error
func NewValidationError(code, message string) *JoblinError {
	return &JoblinError{
		Type:       ErrorTypeValidation,
		Code:       code,
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
		Retryable:  false,
	}
}

// NewConfigurationError creates a new configuration error
func NewConfigurationError(code, message string) *JoblinError {
	return &JoblinError{
		Type:       ErrorTypeConfiguration,
		Code:       code,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
		Retryable:  false,
	}
}

// NewNetworkError creates a new network error
func NewNetworkError(code, message string) *JoblinError {
	return &JoblinError{
		Type:       ErrorTypeNetwork,
		Code:       code,
		Message:    message,
		HTTPStatus: http.StatusServiceUnavailable,
		Retryable:  true,
	}
}

// NewKubernetesError creates a new Kubernetes error
func NewKubernetesError(code, message string) *JoblinError {
	return &JoblinError{
		Type:       ErrorTypeKubernetes,
		Code:       code,
		Message:    message,
		HTTPStatus: http.StatusServiceUnavailable,
		Retryable:  true,
	}
}

// NewStorageError creates a new storage error
func NewStorageError(code, message string) *JoblinError {
	return &JoblinError{
		Type:       ErrorTypeStorage,
		Code:       code,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
		Retryable:  false,
	}
}

// NewNotificationError creates a new notification error
func NewNotificationError(code, message string) *JoblinError {
	return &JoblinError{
		Type:       ErrorTypeNotification,
		Code:       code,
		Message:    message,
		HTTPStatus: http.StatusBadGateway,
		Retryable:  true,
	}
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(code, message string) *JoblinError {
	return &JoblinError{
		Type:       ErrorTypeNotFound,
		Code:       code,
		Message:    message,
		HTTPStatus: http.StatusNotFound,
		Retryable:  false,
	}
}

// NewConflictError creates a new conflict error
func NewConflictError(code, message string) *JoblinError {
	return &JoblinError{
		Type:       ErrorTypeConflict,
		Code:       code,
		Message:    message,
		HTTPStatus: http.StatusConflict,
		Retryable:  false,
	}
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(code, message string) *JoblinError {
	return &JoblinError{
		Type:       ErrorTypeTimeout,
		Code:       code,
		Message:    message,
		HTTPStatus: http.StatusRequestTimeout,
		Retryable:  true,
	}
}

// NewInternalError creates a new internal error
func NewInternalError(code, message string) *JoblinError {
	return &JoblinError{
		Type:       ErrorTypeInternal,
		Code:       code,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
		Retryable:  false,
	}
}

// WrapError wraps a generic error into a JoblinError
func WrapError(err error, errorType ErrorType, code, message string) *JoblinError {
	return &JoblinError{
		Type:    errorType,
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

// IsJoblinError checks if an error is a JoblinError
func IsJoblinError(err error) bool {
	_, ok := err.(*JoblinError)
	return ok
}

// AsJoblinError converts an error to JoblinError if possible
func AsJoblinError(err error) (*JoblinError, bool) {
	joblinErr, ok := err.(*JoblinError)
	return joblinErr, ok
}
