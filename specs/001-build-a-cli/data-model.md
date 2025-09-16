# Data Model: Joblin CLI

## Core Entities

### Job
**Purpose**: Represents a Python script deployment on Kubernetes with full lifecycle tracking.

**Fields**:
- `ID` (string): Unique identifier for the job (UUID v4)
- `Name` (string): Human-readable job name (user-provided or auto-generated)
- `ScriptPath` (string): Path to the Python script file
- `ScriptContent` ([]byte): Content of the Python script (stored for history)
- `Dependencies` ([]string): Python package dependencies (from requirements.txt or inline)
- `Status` (JobStatus): Current state of the job
- `ResourceLimits` (ResourceSpec): CPU and memory allocation
- `CreatedAt` (time.Time): Job creation timestamp
- `StartedAt` (*time.Time): Job execution start time (nil if not started)
- `CompletedAt` (*time.Time): Job completion timestamp (nil if running/pending)
- `KubernetesJobName` (string): Generated Kubernetes Job resource name
- `Namespace` (string): Kubernetes namespace for job execution
- `ClusterContext` (string): Kubernetes context from kubeconfig
- `ExitCode` (*int): Job exit code (nil if not completed)
- `TTL` (time.Duration): Time-to-live for automatic cleanup
- `WebhookURL` (string): Teams webhook URL for notifications
- `Labels` (map[string]string): User-defined labels for job organization

**Validation Rules**:
- `ID` must be valid UUID v4 format
- `Name` must be 1-63 characters, DNS-1123 compliant
- `ScriptPath` must exist and be readable
- `ResourceLimits.CPU` must be valid Kubernetes quantity (e.g., "100m", "1")
- `ResourceLimits.Memory` must be valid Kubernetes quantity (e.g., "128Mi", "1Gi")
- `TTL` must be between 1 minute and 7 days

**State Transitions**:
```
Pending → Running → Completed (success)
Pending → Running → Failed (error)
Pending → Running → Terminated (user cancellation)
Pending → Failed (creation error)
```

### JobStatus
**Purpose**: Enumeration of possible job states with detailed information.

**Values**:
- `Pending`: Job created but not yet scheduled on Kubernetes
- `Running`: Job is executing on the cluster
- `Completed`: Job finished successfully (exit code 0)
- `Failed`: Job finished with error (non-zero exit code)
- `Terminated`: Job was manually stopped by user
- `Unknown`: Unable to determine job status (cluster unreachable)

### ResourceSpec
**Purpose**: Defines compute resource allocation for job execution.

**Fields**:
- `CPU` (string): CPU limit in Kubernetes quantity format (default: "100m")
- `Memory` (string): Memory limit in Kubernetes quantity format (default: "128Mi")
- `EphemeralStorage` (string): Temporary storage limit (default: "1Gi")

**Validation Rules**:
- All fields must be valid Kubernetes resource quantities
- CPU minimum: "10m", maximum: "16" (cores)
- Memory minimum: "64Mi", maximum: "32Gi"
- EphemeralStorage minimum: "100Mi", maximum: "10Gi"

### JobLog
**Purpose**: Captures output and error messages from job execution.

**Fields**:
- `JobID` (string): Reference to parent Job ID
- `Timestamp` (time.Time): Log entry timestamp
- `Source` (LogSource): Whether from stdout or stderr
- `Content` (string): Log message content
- `PodName` (string): Kubernetes pod that generated the log

**Validation Rules**:
- `JobID` must reference existing Job
- `Content` maximum length: 10KB per entry
- Logs older than Job TTL are automatically purged

### LogSource
**Purpose**: Enumeration for log entry sources.

**Values**:
- `Stdout`: Standard output from Python script
- `Stderr`: Standard error from Python script
- `System`: Kubernetes system messages (container events)

### Notification
**Purpose**: Teams webhook message for job completion events.

**Fields**:
- `JobID` (string): Reference to completed Job
- `Type` (NotificationType): Success or failure notification
- `SentAt` (time.Time): Notification delivery timestamp
- `WebhookURL` (string): Teams webhook endpoint
- `MessageID` (string): Teams message identifier for tracking
- `RetryCount` (int): Number of delivery attempts
- `LastError` (string): Last delivery error message (if failed)

**Validation Rules**:
- `WebhookURL` must be valid HTTPS URL
- `RetryCount` maximum: 5 attempts
- Notifications older than 24 hours are archived

### NotificationType
**Purpose**: Enumeration for notification event types.

**Values**:
- `Success`: Job completed successfully
- `Failure`: Job failed with error
- `Terminated`: Job was manually terminated

## Relationships

### Job → JobLog (1:N)
- One Job can have multiple log entries
- Logs are ordered by timestamp
- Cascading delete: logs removed when job is cleaned up

### Job → Notification (1:N)
- One Job can trigger multiple notifications (retry attempts)
- Only one successful notification per job completion
- Independent lifecycle: notifications persist after job cleanup for audit

## Storage Schema (BBolt)

### Buckets
- `jobs`: Primary job storage (key: Job.ID, value: JSON-serialized Job)
- `logs`: Job logs (key: JobID:timestamp, value: JSON-serialized JobLog)
- `notifications`: Notification history (key: JobID:timestamp, value: JSON-serialized Notification)
- `metadata`: CLI metadata (version, config, last cleanup time)

### Indexing Strategy
- Jobs by status: secondary index for listing active/completed jobs
- Jobs by creation time: for TTL-based cleanup operations
- Logs by job ID: for efficient log retrieval
- Notifications by timestamp: for audit and retry operations

## Configuration Model

### CLIConfig
**Purpose**: User configuration and preferences.

**Fields**:
- `DefaultCluster` (string): Default Kubernetes context
- `DefaultNamespace` (string): Default namespace for job creation
- `DefaultResources` (ResourceSpec): Default resource limits
- `DefaultTTL` (time.Duration): Default job cleanup time
- `TeamsWebhookURL` (string): Default Teams webhook URL
- `LogLevel` (string): Logging verbosity (debug, info, warn, error)
- `DataDir` (string): Directory for local storage files

**Storage**: YAML file in user config directory (~/.joblin/config.yaml)

This data model provides a complete foundation for job lifecycle management, resource tracking, and notification delivery while maintaining data integrity and supporting the constitutional requirements for simple, testable design.