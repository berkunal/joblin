package lib

import (
	"context"
	"os"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

// LogContext is a key type for context values
type LogContext string

const (
	// LogContextJobID is used to add job ID to log context
	LogContextJobID       LogContext = "job_id"
	// LogContextJobName is used to add job name to log context
	LogContextJobName     LogContext = "job_name"
	// LogContextNamespace is used to add namespace to log context
	LogContextNamespace   LogContext = "namespace"
	// LogContextCluster is used to add cluster name to log context
	LogContextCluster     LogContext = "cluster"
	// LogContextOperation is used to add operation name to log context
	LogContextOperation   LogContext = "operation"
	// LogContextComponent is used to add component name to log context
	LogContextComponent   LogContext = "component"
	// LogContextUserID is used to add user ID to log context
	LogContextUserID      LogContext = "user_id"
	// LogContextRequestID is used to add request ID to log context
	LogContextRequestID   LogContext = "request_id"
	// LogContextCorrelation is used to add correlation ID to log context
	LogContextCorrelation LogContext = "correlation_id"
)

// Logger wraps logrus with context support and structured logging
type Logger struct {
	*logrus.Logger
	component string
	fields    logrus.Fields
}

// NewLogger creates a new structured logger instance
func NewLogger(component string) *Logger {
	logger := logrus.New()

	// Set default formatter
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
			logrus.FieldKeyFunc:  "function",
			logrus.FieldKeyFile:  "file",
		},
	})

	// Set output to stderr
	logger.SetOutput(os.Stderr)

	// Set default level
	logger.SetLevel(logrus.InfoLevel)

	// Enable caller information
	logger.SetReportCaller(true)

	return &Logger{
		Logger:    logger,
		component: component,
		fields:    make(logrus.Fields),
	}
}

// NewConsoleLogger creates a logger with human-readable console output
func NewConsoleLogger(component string) *Logger {
	logger := NewLogger(component)

	// Use text formatter for console
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: false,
		FullTimestamp:    true,
		TimestampFormat:  "2006-01-02 15:04:05",
		DisableColors:    false,
		PadLevelText:     true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := strings.Split(f.File, "/")
			return f.Function, filename[len(filename)-1] + ":" + string(rune(f.Line))
		},
	})

	return logger
}

// WithContext creates a new logger entry with context values
func (l *Logger) WithContext(ctx context.Context) *logrus.Entry {
	entry := l.Logger.WithField("component", l.component)

	// Add context values if they exist
	if jobID := ctx.Value(LogContextJobID); jobID != nil {
		entry = entry.WithField(string(LogContextJobID), jobID)
	}
	if jobName := ctx.Value(LogContextJobName); jobName != nil {
		entry = entry.WithField(string(LogContextJobName), jobName)
	}
	if namespace := ctx.Value(LogContextNamespace); namespace != nil {
		entry = entry.WithField(string(LogContextNamespace), namespace)
	}
	if cluster := ctx.Value(LogContextCluster); cluster != nil {
		entry = entry.WithField(string(LogContextCluster), cluster)
	}
	if operation := ctx.Value(LogContextOperation); operation != nil {
		entry = entry.WithField(string(LogContextOperation), operation)
	}
	if requestID := ctx.Value(LogContextRequestID); requestID != nil {
		entry = entry.WithField(string(LogContextRequestID), requestID)
	}
	if correlationID := ctx.Value(LogContextCorrelation); correlationID != nil {
		entry = entry.WithField(string(LogContextCorrelation), correlationID)
	}

	// Add any persistent fields
	for key, value := range l.fields {
		entry = entry.WithField(key, value)
	}

	return entry
}

// WithField adds a field to all subsequent log entries
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := &Logger{
		Logger:    l.Logger,
		component: l.component,
		fields:    make(logrus.Fields),
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Add new field
	newLogger.fields[key] = value

	return newLogger
}

// WithFields adds multiple fields to all subsequent log entries
func (l *Logger) WithFields(fields logrus.Fields) *Logger {
	newLogger := &Logger{
		Logger:    l.Logger,
		component: l.component,
		fields:    make(logrus.Fields),
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Add new fields
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// WithError creates a logger entry with error information
func (l *Logger) WithError(err error) *logrus.Entry {
	entry := l.Logger.WithField("component", l.component).WithError(err)

	// Add structured error information if it's a JoblinError
	if joblinErr, ok := AsJoblinError(err); ok {
		entry = entry.WithFields(logrus.Fields{
			"error_type":    joblinErr.Type,
			"error_code":    joblinErr.Code,
			"error_context": joblinErr.Context,
			"retryable":     joblinErr.Retryable,
		})
	}

	return entry
}

// LogOperation logs the start and end of an operation
func (l *Logger) LogOperation(ctx context.Context, operation string, fn func() error) error {
	ctx = context.WithValue(ctx, LogContextOperation, operation)

	l.WithContext(ctx).Info("Operation started")

	err := fn()

	if err != nil {
		l.WithContext(ctx).WithError(err).Error("Operation failed")
	} else {
		l.WithContext(ctx).Info("Operation completed successfully")
	}

	return err
}

// LogJobOperation logs job-specific operations
func (l *Logger) LogJobOperation(ctx context.Context, jobID, operation string, fn func() error) error {
	ctx = context.WithValue(ctx, LogContextJobID, jobID)
	ctx = context.WithValue(ctx, LogContextOperation, operation)

	return l.LogOperation(ctx, operation, fn)
}

// Emergency logs system-critical errors
func (l *Logger) Emergency(ctx context.Context, message string, fields ...logrus.Fields) {
	entry := l.WithContext(ctx).WithField("severity", "emergency")
	for _, field := range fields {
		entry = entry.WithFields(field)
	}
	entry.Fatal(message)
}

// Alert logs errors requiring immediate attention
func (l *Logger) Alert(ctx context.Context, message string, fields ...logrus.Fields) {
	entry := l.WithContext(ctx).WithField("severity", "alert")
	for _, field := range fields {
		entry = entry.WithFields(field)
	}
	entry.Error(message)
}

// Critical logs critical errors
func (l *Logger) Critical(ctx context.Context, message string, fields ...logrus.Fields) {
	entry := l.WithContext(ctx).WithField("severity", "critical")
	for _, field := range fields {
		entry = entry.WithFields(field)
	}
	entry.Error(message)
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level string) error {
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	l.Logger.SetLevel(logLevel)
	return nil
}

// SetJSONOutput enables JSON formatted output
func (l *Logger) SetJSONOutput() {
	l.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	})
}

// SetConsoleOutput enables human-readable console output
func (l *Logger) SetConsoleOutput() {
	l.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: false,
		FullTimestamp:    true,
		TimestampFormat:  "2006-01-02 15:04:05",
		DisableColors:    false,
	})
}

// Context helper functions

// WithJobContext adds job-related context to the context
func WithJobContext(ctx context.Context, jobID, jobName, namespace string) context.Context {
	ctx = context.WithValue(ctx, LogContextJobID, jobID)
	ctx = context.WithValue(ctx, LogContextJobName, jobName)
	ctx = context.WithValue(ctx, LogContextNamespace, namespace)
	return ctx
}

// WithOperationContext adds operation context
func WithOperationContext(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, LogContextOperation, operation)
}

// WithClusterContext adds cluster context
func WithClusterContext(ctx context.Context, cluster string) context.Context {
	return context.WithValue(ctx, LogContextCluster, cluster)
}

// GetContextValue safely extracts a context value
func GetContextValue(ctx context.Context, key LogContext) string {
	if value := ctx.Value(key); value != nil {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}
