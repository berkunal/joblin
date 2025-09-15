package contract

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJoblinStatusCommand tests the contract for 'joblin status' command
// This test MUST FAIL until the status command is implemented
func TestJoblinStatusCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectJSON     bool
		expectFields   []string
		expectError    string
	}{
		{
			name:           "status_all_jobs",
			args:           []string{"status", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobs"},
		},
		{
			name:           "status_specific_job",
			args:           []string{"status", "12345678-1234-4234-8234-123456789012", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobs"},
		},
		{
			name:           "status_with_watch",
			args:           []string{"status", "--watch", "--refresh", "2s", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobs"},
		},
		{
			name:           "status_nonexistent_job",
			args:           []string{"status", "nonexistent-job-id", "--json"},
			expectExitCode: 1,
			expectError:    "Job ID not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(getJoblinBinary(), tt.args...)
			output, err := cmd.CombinedOutput()

			// Check exit code
			exitCode := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
			assert.Equal(t, tt.expectExitCode, exitCode, "Unexpected exit code. Output: %s", string(output))

			if tt.expectJSON && exitCode == 0 {
				// Parse JSON output
				var result map[string]interface{}
				err := json.Unmarshal(output, &result)
				require.NoError(t, err, "Failed to parse JSON output: %s", string(output))

				// Check required fields
				for _, field := range tt.expectFields {
					assert.Contains(t, result, field, "Missing required field: %s", field)
				}

				// Validate jobs array structure
				if jobs, ok := result["jobs"].([]interface{}); ok {
					for _, job := range jobs {
						jobMap := job.(map[string]interface{})
						requiredJobFields := []string{"jobId", "name", "status", "createdAt", "kubernetesJob", "namespace"}
						for _, field := range requiredJobFields {
							assert.Contains(t, jobMap, field, "Job missing required field: %s", field)
						}

						// Validate status enum
						if status, ok := jobMap["status"]; ok {
							validStatuses := []string{"pending", "running", "completed", "failed", "terminated"}
							assert.Contains(t, validStatuses, status, "Invalid job status: %s", status)
						}

						// Validate timestamps
						if createdAt, ok := jobMap["createdAt"].(string); ok {
							_, err := time.Parse(time.RFC3339, createdAt)
							assert.NoError(t, err, "Invalid createdAt timestamp format")
						}

						// Validate optional timestamps
						if startedAt, ok := jobMap["startedAt"].(string); ok && startedAt != "" {
							_, err := time.Parse(time.RFC3339, startedAt)
							assert.NoError(t, err, "Invalid startedAt timestamp format")
						}

						if completedAt, ok := jobMap["completedAt"].(string); ok && completedAt != "" {
							_, err := time.Parse(time.RFC3339, completedAt)
							assert.NoError(t, err, "Invalid completedAt timestamp format")
						}
					}
				}
			}

			if tt.expectError != "" {
				assert.Contains(t, string(output), tt.expectError, "Expected error message not found")
			}
		})
	}
}

// TestJoblinLogsCommand tests the contract for 'joblin logs' command
// This test MUST FAIL until the logs command is implemented
func TestJoblinLogsCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectJSON     bool
		expectFields   []string
		expectError    string
	}{
		{
			name:           "logs_basic",
			args:           []string{"logs", "12345678-1234-4234-8234-123456789012", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobId", "logs"},
		},
		{
			name:           "logs_with_tail",
			args:           []string{"logs", "12345678-1234-4234-8234-123456789012", "--tail", "50", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobId", "logs"},
		},
		{
			name:           "logs_with_since",
			args:           []string{"logs", "12345678-1234-4234-8234-123456789012", "--since", "2025-01-01T00:00:00Z", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobId", "logs"},
		},
		{
			name:           "logs_with_timestamps",
			args:           []string{"logs", "12345678-1234-4234-8234-123456789012", "--timestamps", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobId", "logs"},
		},
		{
			name:           "logs_with_follow",
			args:           []string{"logs", "12345678-1234-4234-8234-123456789012", "--follow", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobId", "logs"},
		},
		{
			name:           "logs_nonexistent_job",
			args:           []string{"logs", "nonexistent-job-id", "--json"},
			expectExitCode: 1,
			expectError:    "Job ID not found",
		},
		{
			name:           "logs_no_logs_available",
			args:           []string{"logs", "12345678-1234-4234-8234-123456789012", "--json"},
			expectExitCode: 2,
			expectError:    "No logs available yet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(getJoblinBinary(), tt.args...)
			output, err := cmd.CombinedOutput()

			// Check exit code
			exitCode := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
			assert.Equal(t, tt.expectExitCode, exitCode, "Unexpected exit code. Output: %s", string(output))

			if tt.expectJSON && exitCode == 0 {
				// Parse JSON output
				var result map[string]interface{}
				err := json.Unmarshal(output, &result)
				require.NoError(t, err, "Failed to parse JSON output: %s", string(output))

				// Check required fields
				for _, field := range tt.expectFields {
					assert.Contains(t, result, field, "Missing required field: %s", field)
				}

				// Validate logs array structure
				if logs, ok := result["logs"].([]interface{}); ok {
					for _, log := range logs {
						logMap := log.(map[string]interface{})
						requiredLogFields := []string{"timestamp", "source", "content", "podName"}
						for _, field := range requiredLogFields {
							assert.Contains(t, logMap, field, "Log entry missing required field: %s", field)
						}

						// Validate source enum
						if source, ok := logMap["source"]; ok {
							validSources := []string{"stdout", "stderr", "system"}
							assert.Contains(t, validSources, source, "Invalid log source: %s", source)
						}

						// Validate timestamp format
						if timestamp, ok := logMap["timestamp"].(string); ok {
							_, err := time.Parse(time.RFC3339, timestamp)
							assert.NoError(t, err, "Invalid timestamp format")
						}
					}
				}
			}

			if tt.expectError != "" {
				assert.Contains(t, string(output), tt.expectError, "Expected error message not found")
			}
		})
	}
}

// TestJoblinTerminateCommand tests the contract for 'joblin terminate' command
// This test MUST FAIL until the terminate command is implemented
func TestJoblinTerminateCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectJSON     bool
		expectFields   []string
		expectError    string
	}{
		{
			name:           "terminate_basic",
			args:           []string{"terminate", "12345678-1234-4234-8234-123456789012", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobId", "status", "terminatedAt"},
		},
		{
			name:           "terminate_with_force",
			args:           []string{"terminate", "12345678-1234-4234-8234-123456789012", "--force", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobId", "status", "terminatedAt"},
		},
		{
			name:           "terminate_with_wait",
			args:           []string{"terminate", "12345678-1234-4234-8234-123456789012", "--wait", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobId", "status", "terminatedAt"},
		},
		{
			name:           "terminate_nonexistent_job",
			args:           []string{"terminate", "nonexistent-job-id", "--json"},
			expectExitCode: 1,
			expectError:    "Job ID not found",
		},
		{
			name:           "terminate_already_completed",
			args:           []string{"terminate", "12345678-1234-4234-8234-123456789012", "--json"},
			expectExitCode: 2,
			expectError:    "Job already completed/terminated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(getJoblinBinary(), tt.args...)
			output, err := cmd.CombinedOutput()

			// Check exit code
			exitCode := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
			assert.Equal(t, tt.expectExitCode, exitCode, "Unexpected exit code. Output: %s", string(output))

			if tt.expectJSON && exitCode == 0 {
				// Parse JSON output
				var result map[string]interface{}
				err := json.Unmarshal(output, &result)
				require.NoError(t, err, "Failed to parse JSON output: %s", string(output))

				// Check required fields
				for _, field := range tt.expectFields {
					assert.Contains(t, result, field, "Missing required field: %s", field)
				}

				// Validate status
				if status, ok := result["status"]; ok {
					validStatuses := []string{"terminating", "terminated"}
					assert.Contains(t, validStatuses, status, "Invalid termination status: %s", status)
				}

				// Validate terminatedAt timestamp
				if terminatedAt, ok := result["terminatedAt"].(string); ok {
					_, err := time.Parse(time.RFC3339, terminatedAt)
					assert.NoError(t, err, "Invalid terminatedAt timestamp format")
				}
			}

			if tt.expectError != "" {
				assert.Contains(t, string(output), tt.expectError, "Expected error message not found")
			}
		})
	}
}

// TestJoblinOperationsHelp tests help output for operations commands
func TestJoblinOperationsHelp(t *testing.T) {
	commands := []struct {
		command         string
		expectedContent []string
	}{
		{
			command: "status",
			expectedContent: []string{
				"Show status of one or all jobs",
				"Usage:",
				"joblin status [job-id]",
				"--watch",
				"--refresh",
			},
		},
		{
			command: "logs",
			expectedContent: []string{
				"Retrieve logs from a running or completed job",
				"Usage:",
				"joblin logs <job-id>",
				"--follow",
				"--tail",
				"--since",
				"--timestamps",
			},
		},
		{
			command: "terminate",
			expectedContent: []string{
				"Stop a running job",
				"Usage:",
				"joblin terminate <job-id>",
				"--force",
				"--wait",
			},
		},
	}

	for _, cmd := range commands {
		t.Run("help_"+cmd.command, func(t *testing.T) {
			execCmd := exec.Command(getJoblinBinary(), cmd.command, "--help")
			output, err := execCmd.CombinedOutput()

			assert.NoError(t, err, "Help command should not fail")

			helpText := string(output)
			for _, content := range cmd.expectedContent {
				assert.Contains(t, helpText, content, "Help text missing expected content: %s", content)
			}
		})
	}
}

// TestJoblinOperationsTextOutput tests non-JSON output formats
func TestJoblinOperationsTextOutput(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectContent  []string
	}{
		{
			name:           "status_text_output",
			args:           []string{"status"},
			expectExitCode: 0,
			expectContent:  []string{"JOB ID", "NAME", "STATUS", "CREATED", "NAMESPACE"},
		},
		{
			name:           "logs_text_output",
			args:           []string{"logs", "12345678-1234-4234-8234-123456789012"},
			expectExitCode: 0,
			expectContent:  []string{}, // Text logs should be raw output
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(getJoblinBinary(), tt.args...)
			output, err := cmd.CombinedOutput()

			// Check exit code
			exitCode := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
			assert.Equal(t, tt.expectExitCode, exitCode, "Unexpected exit code. Output: %s", string(output))

			// Check expected content
			outputStr := string(output)
			for _, content := range tt.expectContent {
				assert.Contains(t, outputStr, content, "Expected content not found: %s", content)
			}
		})
	}
}

// TestJoblinOperationsErrorHandling tests error scenarios
func TestJoblinOperationsErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectError    string
	}{
		{
			name:           "status_kubernetes_error",
			args:           []string{"status", "--context", "invalid-context", "--json"},
			expectExitCode: 3,
			expectError:    "Kubernetes API error",
		},
		{
			name:           "logs_kubernetes_error",
			args:           []string{"logs", "12345678-1234-4234-8234-123456789012", "--context", "invalid-context", "--json"},
			expectExitCode: 3,
			expectError:    "Kubernetes API error",
		},
		{
			name:           "terminate_kubernetes_error",
			args:           []string{"terminate", "12345678-1234-4234-8234-123456789012", "--context", "invalid-context", "--json"},
			expectExitCode: 3,
			expectError:    "Kubernetes API error",
		},
		{
			name:           "logs_missing_job_id",
			args:           []string{"logs", "--json"},
			expectExitCode: 1,
			expectError:    "job ID is required",
		},
		{
			name:           "terminate_missing_job_id",
			args:           []string{"terminate", "--json"},
			expectExitCode: 1,
			expectError:    "job ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(getJoblinBinary(), tt.args...)
			output, err := cmd.CombinedOutput()

			// Check exit code
			exitCode := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
			assert.Equal(t, tt.expectExitCode, exitCode, "Unexpected exit code. Output: %s", string(output))

			// Check error message
			outputStr := strings.ToLower(string(output))
			expectedError := strings.ToLower(tt.expectError)
			assert.Contains(t, outputStr, expectedError, "Expected error message not found")
		})
	}
}