# CLI Interface Contract: Joblin

## Global Flags
All commands support these global flags:
- `--config`: Config file path (default: ~/.joblin/config.yaml)
- `--context`: Kubernetes context to use
- `--namespace`: Kubernetes namespace for operations
- `--verbose`: Enable verbose logging
- `--json`: Output in JSON format
- `--help`: Show help information
- `--version`: Show version information

## Commands

### `joblin deploy`
**Purpose**: Deploy a Python script as a Kubernetes job

**Syntax**: `joblin deploy [script-path] [flags]`

**Arguments**:
- `script-path` (required): Path to Python script file

**Flags**:
- `--name`: Job name (default: auto-generated from script filename)
- `--cpu`: CPU limit (default: "100m")
- `--memory`: Memory limit (default: "128Mi")
- `--ttl`: Time-to-live for cleanup (default: "24h")
- `--webhook`: Teams webhook URL for notifications
- `--requirements`: Path to requirements.txt file
- `--env`: Environment variables as KEY=VALUE pairs (repeatable)
- `--labels`: Labels as KEY=VALUE pairs (repeatable)
- `--wait`: Wait for job completion before returning
- `--timeout`: Maximum wait time when using --wait (default: "30m")

**Output** (JSON format with --json):
```json
{
  "jobId": "uuid-v4",
  "name": "job-name",
  "status": "pending",
  "kubernetesJob": "k8s-job-name",
  "namespace": "default",
  "createdAt": "2025-09-15T10:30:00Z"
}
```

**Exit Codes**:
- 0: Success
- 1: Script file not found or unreadable
- 2: Invalid resource specifications
- 3: Kubernetes API error
- 4: Configuration error

### `joblin status`
**Purpose**: Show status of one or all jobs

**Syntax**: `joblin status [job-id] [flags]`

**Arguments**:
- `job-id` (optional): Specific job ID to check (if omitted, shows all jobs)

**Flags**:
- `--watch`: Continuously watch for status changes
- `--refresh`: Refresh interval for watch mode (default: "5s")

**Output** (JSON format with --json):
```json
{
  "jobs": [
    {
      "jobId": "uuid-v4",
      "name": "job-name",
      "status": "running",
      "createdAt": "2025-09-15T10:30:00Z",
      "startedAt": "2025-09-15T10:31:00Z",
      "resourceLimits": {
        "cpu": "100m",
        "memory": "128Mi"
      },
      "kubernetesJob": "k8s-job-name",
      "namespace": "default"
    }
  ]
}
```

**Exit Codes**:
- 0: Success
- 1: Job ID not found
- 3: Kubernetes API error

### `joblin logs`
**Purpose**: Retrieve logs from a running or completed job

**Syntax**: `joblin logs <job-id> [flags]`

**Arguments**:
- `job-id` (required): Job ID to get logs from

**Flags**:
- `--follow`: Stream logs in real-time
- `--tail`: Number of lines to show from end (default: all)
- `--since`: Show logs since timestamp (RFC3339 format)
- `--timestamps`: Include timestamps in output

**Output** (text format by default, JSON with --json):
```json
{
  "jobId": "uuid-v4",
  "logs": [
    {
      "timestamp": "2025-09-15T10:31:30Z",
      "source": "stdout",
      "content": "Processing data...",
      "podName": "job-pod-xyz"
    }
  ]
}
```

**Exit Codes**:
- 0: Success
- 1: Job ID not found
- 2: No logs available yet
- 3: Kubernetes API error

### `joblin terminate`
**Purpose**: Stop a running job

**Syntax**: `joblin terminate <job-id> [flags]`

**Arguments**:
- `job-id` (required): Job ID to terminate

**Flags**:
- `--force`: Force termination without grace period
- `--wait`: Wait for termination to complete

**Output** (JSON format with --json):
```json
{
  "jobId": "uuid-v4",
  "status": "terminating",
  "terminatedAt": "2025-09-15T10:45:00Z"
}
```

**Exit Codes**:
- 0: Success
- 1: Job ID not found
- 2: Job already completed/terminated
- 3: Kubernetes API error

### `joblin list`
**Purpose**: List all jobs with filtering options

**Syntax**: `joblin list [flags]`

**Flags**:
- `--status`: Filter by job status (pending, running, completed, failed, terminated)
- `--name`: Filter by job name pattern (supports wildcards)
- `--since`: Show jobs created since timestamp
- `--labels`: Filter by labels (KEY=VALUE pairs)
- `--limit`: Maximum number of jobs to show (default: 50)

**Output** (JSON format with --json):
```json
{
  "jobs": [
    {
      "jobId": "uuid-v4",
      "name": "job-name",
      "status": "completed",
      "createdAt": "2025-09-15T10:30:00Z",
      "completedAt": "2025-09-15T10:35:00Z",
      "exitCode": 0
    }
  ],
  "total": 1,
  "filtered": 1
}
```

**Exit Codes**:
- 0: Success
- 3: Kubernetes API error

### `joblin cleanup`
**Purpose**: Clean up completed/failed jobs and their logs

**Syntax**: `joblin cleanup [flags]`

**Flags**:
- `--older-than`: Clean up jobs older than duration (default: "7d")
- `--status`: Only clean up jobs with specific status
- `--dry-run`: Show what would be cleaned up without doing it
- `--force`: Skip confirmation prompt

**Output** (JSON format with --json):
```json
{
  "cleanedJobs": 5,
  "freedSpace": "2.3MB",
  "errors": []
}
```

**Exit Codes**:
- 0: Success
- 3: Kubernetes API error

### `joblin config`
**Purpose**: Manage configuration settings

**Syntax**: `joblin config <subcommand> [args]`

**Subcommands**:
- `show`: Display current configuration
- `set <key> <value>`: Set a configuration value
- `unset <key>`: Remove a configuration value
- `init`: Initialize configuration with defaults

**Configuration Keys**:
- `default.cluster`: Default Kubernetes context
- `default.namespace`: Default namespace
- `default.cpu`: Default CPU limit
- `default.memory`: Default memory limit
- `default.ttl`: Default job TTL
- `webhook.url`: Default Teams webhook URL
- `log.level`: Logging level

**Exit Codes**:
- 0: Success
- 1: Invalid configuration key/value
- 2: Configuration file error

## Error Handling

### Standard Error Format (JSON)
```json
{
  "error": {
    "type": "ValidationError",
    "message": "CPU limit '10x' is not a valid Kubernetes quantity",
    "details": {
      "field": "cpu",
      "value": "10x",
      "expected": "Kubernetes quantity format (e.g., 100m, 1, 2.5)"
    }
  }
}
```

### Common Error Types
- `ValidationError`: Invalid input parameters
- `NotFoundError`: Job/resource not found
- `KubernetesError`: Kubernetes API errors
- `ConfigurationError`: Invalid configuration
- `NetworkError`: Connection failures
- `AuthenticationError`: Kubernetes auth failures

## Environment Variables
- `JOBLIN_CONFIG`: Override config file path
- `JOBLIN_CONTEXT`: Override Kubernetes context
- `JOBLIN_NAMESPACE`: Override namespace
- `JOBLIN_LOG_LEVEL`: Override log level
- `KUBECONFIG`: Kubernetes config file path (standard)

## Shell Completion
Support for bash, zsh, fish, and PowerShell completion via:
- `joblin completion bash`
- `joblin completion zsh`
- `joblin completion fish`
- `joblin completion powershell`

This CLI interface contract defines the complete user interaction surface for the Joblin tool, ensuring consistent behavior and clear expectations for all operations.