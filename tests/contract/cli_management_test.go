package contract

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJoblinListCommand tests the contract for 'joblin list' command
// This test MUST FAIL until the list command is implemented
func TestJoblinListCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectJSON     bool
		expectFields   []string
		expectError    string
	}{
		{
			name:           "list_all_jobs",
			args:           []string{"list", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobs", "total", "filtered"},
		},
		{
			name:           "list_with_status_filter",
			args:           []string{"list", "--status", "running", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobs", "total", "filtered"},
		},
		{
			name:           "list_with_name_filter",
			args:           []string{"list", "--name", "test-*", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobs", "total", "filtered"},
		},
		{
			name:           "list_with_since_filter",
			args:           []string{"list", "--since", "2025-01-01T00:00:00Z", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobs", "total", "filtered"},
		},
		{
			name:           "list_with_labels_filter",
			args:           []string{"list", "--labels", "team=data-science", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobs", "total", "filtered"},
		},
		{
			name:           "list_with_limit",
			args:           []string{"list", "--limit", "10", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobs", "total", "filtered"},
		},
		{
			name:           "list_multiple_filters",
			args:           []string{"list", "--status", "completed", "--labels", "team=data-science", "--limit", "5", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"jobs", "total", "filtered"},
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

				// Validate structure
				if jobs, ok := result["jobs"].([]interface{}); ok {
					for _, job := range jobs {
						jobMap := job.(map[string]interface{})
						requiredFields := []string{"jobId", "name", "status", "createdAt"}
						for _, field := range requiredFields {
							assert.Contains(t, jobMap, field, "Job missing required field: %s", field)
						}

						// Validate completedAt if present
						if completedAt, ok := jobMap["completedAt"].(string); ok && completedAt != "" {
							_, err := time.Parse(time.RFC3339, completedAt)
							assert.NoError(t, err, "Invalid completedAt timestamp format")
						}

						// Validate exitCode if present
						if exitCode, ok := jobMap["exitCode"]; ok && exitCode != nil {
							assert.IsType(t, float64(0), exitCode, "exitCode should be a number")
						}
					}
				}

				// Validate total and filtered counts
				if total, ok := result["total"].(float64); ok {
					assert.GreaterOrEqual(t, total, float64(0), "Total count should be non-negative")
				}

				if filtered, ok := result["filtered"].(float64); ok {
					assert.GreaterOrEqual(t, filtered, float64(0), "Filtered count should be non-negative")
				}
			}

			if tt.expectError != "" {
				assert.Contains(t, string(output), tt.expectError, "Expected error message not found")
			}
		})
	}
}

// TestJoblinCleanupCommand tests the contract for 'joblin cleanup' command
// This test MUST FAIL until the cleanup command is implemented
func TestJoblinCleanupCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectJSON     bool
		expectFields   []string
		expectError    string
	}{
		{
			name:           "cleanup_default",
			args:           []string{"cleanup", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"cleanedJobs", "freedSpace", "errors"},
		},
		{
			name:           "cleanup_older_than",
			args:           []string{"cleanup", "--older-than", "24h", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"cleanedJobs", "freedSpace", "errors"},
		},
		{
			name:           "cleanup_by_status",
			args:           []string{"cleanup", "--status", "completed", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"cleanedJobs", "freedSpace", "errors"},
		},
		{
			name:           "cleanup_dry_run",
			args:           []string{"cleanup", "--dry-run", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"cleanedJobs", "freedSpace", "errors"},
		},
		{
			name:           "cleanup_force",
			args:           []string{"cleanup", "--force", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"cleanedJobs", "freedSpace", "errors"},
		},
		{
			name:           "cleanup_combined_filters",
			args:           []string{"cleanup", "--older-than", "1h", "--status", "failed", "--force", "--json"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"cleanedJobs", "freedSpace", "errors"},
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

				// Validate field types
				if cleanedJobs, ok := result["cleanedJobs"].(float64); ok {
					assert.GreaterOrEqual(t, cleanedJobs, float64(0), "Cleaned jobs count should be non-negative")
				}

				if freedSpace, ok := result["freedSpace"].(string); ok {
					// Should be in format like "2.3MB"
					assert.Regexp(t, `^\d+(\.\d+)?(KB|MB|GB|TB)$`, freedSpace, "Invalid freed space format")
				}

				if errors, ok := result["errors"].([]interface{}); ok {
					// Errors array should be present (can be empty)
					for _, errorItem := range errors {
						assert.IsType(t, "", errorItem, "Error should be a string")
					}
				}
			}

			if tt.expectError != "" {
				assert.Contains(t, string(output), tt.expectError, "Expected error message not found")
			}
		})
	}
}

// TestJoblinConfigCommand tests the contract for 'joblin config' command
// This test MUST FAIL until the config command is implemented
func TestJoblinConfigCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectContent  []string
		expectError    string
	}{
		{
			name:           "config_show",
			args:           []string{"config", "show"},
			expectExitCode: 0,
			expectContent:  []string{"default.cluster", "default.namespace", "webhook.url"},
		},
		{
			name:           "config_init",
			args:           []string{"config", "init"},
			expectExitCode: 0,
			expectContent:  []string{"Configuration initialized"},
		},
		{
			name:           "config_set_valid",
			args:           []string{"config", "set", "default.cluster", "test-cluster"},
			expectExitCode: 0,
			expectContent:  []string{"Configuration updated"},
		},
		{
			name:           "config_set_webhook",
			args:           []string{"config", "set", "webhook.url", "https://teams.microsoft.com/webhook/test"},
			expectExitCode: 0,
			expectContent:  []string{"Configuration updated"},
		},
		{
			name:           "config_unset",
			args:           []string{"config", "unset", "default.cluster"},
			expectExitCode: 0,
			expectContent:  []string{"Configuration updated"},
		},
		{
			name:           "config_set_invalid_key",
			args:           []string{"config", "set", "invalid.key", "value"},
			expectExitCode: 1,
			expectError:    "Invalid configuration key",
		},
		{
			name:           "config_set_invalid_value",
			args:           []string{"config", "set", "default.cpu", "invalid-quantity"},
			expectExitCode: 1,
			expectError:    "Invalid configuration value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temporary config file for testing
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "test-config.yaml")

			args := append([]string{"--config", configFile}, tt.args...)
			cmd := exec.Command(getJoblinBinary(), args...)
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

			if tt.expectError != "" {
				assert.Contains(t, outputStr, tt.expectError, "Expected error message not found")
			}
		})
	}
}

// TestJoblinConfigKeys tests valid configuration keys
func TestJoblinConfigKeys(t *testing.T) {
	validKeys := []string{
		"default.cluster",
		"default.namespace",
		"default.cpu",
		"default.memory",
		"default.ttl",
		"webhook.url",
		"log.level",
	}

	for _, key := range validKeys {
		t.Run("valid_key_"+strings.ReplaceAll(key, ".", "_"), func(t *testing.T) {
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "test-config.yaml")

			// Initialize config first
			initCmd := exec.Command(getJoblinBinary(), "--config", configFile, "config", "init")
			_, err := initCmd.CombinedOutput()
			require.NoError(t, err, "Config init should succeed")

			// Set the key
			setCmd := exec.Command(getJoblinBinary(), "--config", configFile, "config", "set", key, "test-value")
			output, err := setCmd.CombinedOutput()

			exitCode := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}

			// Some keys may have value validation, but the key itself should be valid
			assert.True(t, exitCode == 0 || strings.Contains(string(output), "Invalid configuration value"),
				"Key should be valid, got exit code %d with output: %s", exitCode, string(output))
		})
	}
}

// TestJoblinManagementHelp tests help output for management commands
func TestJoblinManagementHelp(t *testing.T) {
	commands := []struct {
		command         string
		expectedContent []string
	}{
		{
			command: "list",
			expectedContent: []string{
				"List all jobs with filtering options",
				"Usage:",
				"joblin list",
				"--status",
				"--name",
				"--since",
				"--labels",
				"--limit",
			},
		},
		{
			command: "cleanup",
			expectedContent: []string{
				"Clean up completed/failed jobs and their logs",
				"Usage:",
				"joblin cleanup",
				"--older-than",
				"--status",
				"--dry-run",
				"--force",
			},
		},
		{
			command: "config",
			expectedContent: []string{
				"Manage configuration settings",
				"Usage:",
				"joblin config <subcommand>",
				"show",
				"set",
				"unset",
				"init",
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

// TestJoblinManagementTextOutput tests non-JSON output formats
func TestJoblinManagementTextOutput(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectContent  []string
	}{
		{
			name:           "list_text_output",
			args:           []string{"list"},
			expectExitCode: 0,
			expectContent:  []string{"JOB ID", "NAME", "STATUS", "CREATED", "COMPLETED", "EXIT CODE"},
		},
		{
			name:           "cleanup_text_output",
			args:           []string{"cleanup", "--dry-run"},
			expectExitCode: 0,
			expectContent:  []string{"Would clean up", "jobs", "Free up approximately"},
		},
		{
			name:           "config_show_text_output",
			args:           []string{"config", "show"},
			expectExitCode: 0,
			expectContent:  []string{"Current configuration:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "test-config.yaml")

			args := tt.args
			if strings.Contains(tt.name, "config") {
				args = append([]string{"--config", configFile}, args...)
			}

			cmd := exec.Command(getJoblinBinary(), args...)
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

// TestJoblinManagementErrorHandling tests error scenarios
func TestJoblinManagementErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectError    string
	}{
		{
			name:           "list_kubernetes_error",
			args:           []string{"list", "--context", "invalid-context", "--json"},
			expectExitCode: 3,
			expectError:    "Kubernetes API error",
		},
		{
			name:           "cleanup_kubernetes_error",
			args:           []string{"cleanup", "--context", "invalid-context", "--json"},
			expectExitCode: 3,
			expectError:    "Kubernetes API error",
		},
		{
			name:           "config_file_error",
			args:           []string{"--config", "/invalid/path/config.yaml", "config", "show"},
			expectExitCode: 2,
			expectError:    "Configuration file error",
		},
		{
			name:           "config_missing_subcommand",
			args:           []string{"config"},
			expectExitCode: 1,
			expectError:    "subcommand is required",
		},
		{
			name:           "config_set_missing_value",
			args:           []string{"config", "set", "default.cluster"},
			expectExitCode: 1,
			expectError:    "value is required",
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
