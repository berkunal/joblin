package contract

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test constants to reduce duplication
const (
	testScriptName    = "test_script.py"
	testScriptContent = "print('test')"
	testPyName        = "test.py"
	memoryFlag        = "--memory"
	labelsFlag        = "--labels"
)

// TestJoblinDeployCommand tests the contract for 'joblin deploy' command
// This test MUST FAIL until the deploy command is implemented
func TestJoblinDeployCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectExitCode int
		expectJSON     bool
		expectFields   []string
		expectError    string
		setupScript    func(t *testing.T) string // Returns script path
	}{
		{
			name:           "deploy_simple_script",
			args:           []string{"deploy", testScriptName},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"id", "name", "status", "kubernetes_job_name", "namespace", "created_at"},
			setupScript: func(t *testing.T) string {
				return createTestScript(t, testScriptName, "print('Hello, World!')")
			},
		},
		{
			name:           "deploy_with_custom_name",
			args:           []string{"deploy", testScriptName, "--name", "my-test-job"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"id", "name", "status", "kubernetes_job_name", "namespace", "created_at"},
			setupScript: func(t *testing.T) string {
				return createTestScript(t, testScriptName, "print('Custom named job')")
			},
		},
		{
			name:           "deploy_with_resources",
			args:           []string{"deploy", testScriptName, "--cpu", "200m", memoryFlag, "256Mi"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"id", "name", "status", "kubernetes_job_name", "namespace", "created_at"},
			setupScript: func(t *testing.T) string {
				return createTestScript(t, testScriptName, "print('Resource limited job')")
			},
		},
		{
			name:           "deploy_with_requirements",
			args:           []string{"deploy", testScriptName, "--requirements", "requirements.txt"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"id", "name", "status", "kubernetes_job_name", "namespace", "created_at"},
			setupScript: func(t *testing.T) string {
				scriptPath := createTestScript(t, testScriptName, "import requests\nprint('Dependencies work!')")
				createRequirementsFile(t, "requirements.txt", "requests==2.31.0")
				return scriptPath
			},
		},
		{
			name:           "deploy_with_environment_variables",
			args:           []string{"deploy", testScriptName, "--env", "ENV_VAR=test_value", "--env", "DEBUG=true"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"id", "name", "status", "kubernetes_job_name", "namespace", "created_at"},
			setupScript: func(t *testing.T) string {
				return createTestScript(t, testScriptName, "import os\nprint(f'ENV_VAR: {os.environ.get(\"ENV_VAR\")}')")
			},
		},
		{
			name:           "deploy_with_labels",
			args:           []string{"deploy", testScriptName, labelsFlag, "team=data-science", labelsFlag, "project=analysis"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"id", "name", "status", "kubernetes_job_name", "namespace", "created_at"},
			setupScript: func(t *testing.T) string {
				return createTestScript(t, testScriptName, "print('Labeled job')")
			},
		},
		{
			name:           "deploy_with_webhook",
			args:           []string{"deploy", testScriptName, "--webhook", "https://teams.microsoft.com/webhook/test"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"id", "name", "status", "kubernetes_job_name", "namespace", "created_at"},
			setupScript: func(t *testing.T) string {
				return createTestScript(t, testScriptName, "print('Job with webhook')")
			},
		},
		{
			name:           "deploy_with_ttl",
			args:           []string{"deploy", testScriptName, "--ttl", "2h"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"id", "name", "status", "kubernetes_job_name", "namespace", "created_at"},
			setupScript: func(t *testing.T) string {
				return createTestScript(t, testScriptName, "print('Job with TTL')")
			},
		},
		{
			name:           "deploy_with_wait",
			args:           []string{"deploy", testScriptName, "--wait", "--timeout", "30s"},
			expectExitCode: 0,
			expectJSON:     true,
			expectFields:   []string{"id", "name", "status", "kubernetes_job_name", "namespace", "created_at"},
			setupScript: func(t *testing.T) string {
				return createTestScript(t, testScriptName, "import time\ntime.sleep(1)\nprint('Short job')")
			},
		},
		// Error cases
		{
			name:           "deploy_nonexistent_script",
			args:           []string{"deploy", "nonexistent.py"},
			expectExitCode: 1,
			expectError:    "Script file not found or unreadable",
			setupScript:    func(t *testing.T) string { return "" }, // No setup needed
		},
		{
			name:           "deploy_invalid_cpu",
			args:           []string{"deploy", testScriptName, "--cpu", "invalid"},
			expectExitCode: 2,
			expectError:    "Invalid resource specifications",
			setupScript: func(t *testing.T) string {
				return createTestScript(t, testScriptName, testScriptContent)
			},
		},
		{
			name:           "deploy_invalid_memory",
			args:           []string{"deploy", testScriptName, memoryFlag, "999ZZ"},
			expectExitCode: 2,
			expectError:    "Invalid resource specifications",
			setupScript: func(t *testing.T) string {
				return createTestScript(t, testScriptName, testScriptContent)
			},
		},
		{
			name:           "deploy_invalid_ttl",
			args:           []string{"deploy", testScriptName, "--ttl", "invalid"},
			expectExitCode: 2,
			expectError:    "Invalid TTL format",
			setupScript: func(t *testing.T) string {
				return createTestScript(t, testScriptName, testScriptContent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			tempDir := t.TempDir()
			oldDir, _ := os.Getwd()
			defer os.Chdir(oldDir)
			os.Chdir(tempDir)

			// Create test script if needed
			var scriptPath string
			if tt.setupScript != nil {
				// Create script in the current working directory (tempDir)
				scriptPath = filepath.Join(tempDir, testScriptName)
				content := "print('Hello, World!')"
				if strings.Contains(tt.name, "custom_name") {
					content = "print('Custom named job')"
				}
				err := os.WriteFile(scriptPath, []byte(content), 0644)
				require.NoError(t, err, "Failed to create test script")
			}

			// Build command arguments with --json for parseable output
			args := append([]string{"--json"}, tt.args...)

			// Execute joblin command
			cmd := exec.Command(getJoblinBinary(), args...)
			cmd.Dir = tempDir

			// Capture stdout and stderr separately
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				require.NoError(t, err, "Failed to create stdout pipe")
			}
			stderr, err := cmd.StderrPipe()
			if err != nil {
				require.NoError(t, err, "Failed to create stderr pipe")
			}

			// Start the command
			err = cmd.Start()
			if err != nil {
				require.NoError(t, err, "Failed to start command")
			}

			// Read stdout and stderr
			stdoutData, err := io.ReadAll(stdout)
			if err != nil {
				require.NoError(t, err, "Failed to read stdout")
			}
			stderrData, err := io.ReadAll(stderr)
			if err != nil {
				require.NoError(t, err, "Failed to read stderr")
			}

			// Wait for command to complete
			err = cmd.Wait()

			// Combine outputs for error reporting (backwards compatibility)
			output := append(stdoutData, stderrData...)

			// Check exit code
			exitCode := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
			assert.Equal(t, tt.expectExitCode, exitCode, "Unexpected exit code. Output: %s", string(output))

			if tt.expectJSON && exitCode == 0 {
				// Parse JSON output from stdout only
				var result map[string]interface{}
				err := json.Unmarshal(stdoutData, &result)
				require.NoError(t, err, "Failed to parse JSON output: %s", string(stdoutData))

				// Check required fields
				for _, field := range tt.expectFields {
					assert.Contains(t, result, field, "Missing required field: %s", field)
				}

				// Validate field types and formats
				if jobID, ok := result["id"]; ok {
					jobIDStr := jobID.(string)
					assert.Regexp(t, `^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`, jobIDStr, "Invalid UUID format")
				}

				if status, ok := result["status"]; ok {
					assert.Contains(t, []string{"Pending", "Running", "Completed", "Failed"}, status, "Invalid status value")
				}

				if createdAt, ok := result["created_at"]; ok {
					createdAtStr := createdAt.(string)
					_, err := time.Parse(time.RFC3339, createdAtStr)
					assert.NoError(t, err, "Invalid RFC3339 timestamp format")
				}

				if kubernetesJob, ok := result["kubernetes_job_name"]; ok {
					kubernetesJobStr := kubernetesJob.(string)
					assert.Regexp(t, `^joblin-.*-[0-9]+$`, kubernetesJobStr, "Invalid Kubernetes job name format")
				}
			}

			if tt.expectError != "" {
				// Check error message in output
				outputStr := string(output)
				assert.Contains(t, outputStr, tt.expectError, "Expected error message not found in output")
			}

			// Cleanup created script
			if scriptPath != "" {
				os.Remove(scriptPath)
			}
		})
	}
}

// TestJoblinDeployGlobalFlags tests global flags with deploy command
func TestJoblinDeployGlobalFlags(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectExitCode int
	}{
		{
			name:           "deploy_with_custom_config",
			args:           []string{"--config", "/tmp/joblin-test.yaml", "deploy", testPyName},
			expectExitCode: 0,
		},
		{
			name:           "deploy_with_custom_context",
			args:           []string{"--context", "test-cluster", "deploy", testPyName},
			expectExitCode: 0,
		},
		{
			name:           "deploy_with_custom_namespace",
			args:           []string{"--namespace", "test-namespace", "deploy", testPyName},
			expectExitCode: 0,
		},
		{
			name:           "deploy_with_verbose",
			args:           []string{"--verbose", "deploy", testPyName},
			expectExitCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			oldDir, _ := os.Getwd()
			defer os.Chdir(oldDir)
			os.Chdir(tempDir)

			// Create test script
			createTestScript(t, testPyName, testScriptContent)

			// Execute command
			cmd := exec.Command(getJoblinBinary(), tt.args...)
			cmd.Dir = tempDir
			_, err := cmd.CombinedOutput()

			// Check exit code
			exitCode := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
			assert.Equal(t, tt.expectExitCode, exitCode)
		})
	}
}

// TestJoblinDeployHelp tests help output for deploy command
func TestJoblinDeployHelp(t *testing.T) {
	cmd := exec.Command(getJoblinBinary(), "deploy", "--help")
	output, err := cmd.CombinedOutput()

	assert.NoError(t, err, "Help command should not fail")

	helpText := string(output)
	expectedSections := []string{
		"Deploy a Python script as a Kubernetes job",
		"Usage:",
		"joblin deploy [script-path]",
		"Flags:",
		"--name",
		"--cpu",
		memoryFlag,
		"--ttl",
		"--webhook",
		"--requirements",
		"--env",
		labelsFlag,
		"--wait",
		"--timeout",
	}

	for _, section := range expectedSections {
		assert.Contains(t, helpText, section, "Help text missing expected section: %s", section)
	}
}

// Helper functions

func createTestScript(t *testing.T, filename, content string) string {
	scriptPath := filepath.Join(t.TempDir(), filename)
	err := os.WriteFile(scriptPath, []byte(content), 0644)
	require.NoError(t, err, "Failed to create test script")
	return scriptPath
}

func createRequirementsFile(t *testing.T, filename, content string) string {
	reqPath := filepath.Join(t.TempDir(), filename)
	err := os.WriteFile(reqPath, []byte(content), 0644)
	require.NoError(t, err, "Failed to create requirements file")
	return reqPath
}

func getJoblinBinary() string {
	// Try to find the joblin binary
	if binary, ok := os.LookupEnv("JOBLIN_BINARY"); ok {
		if _, err := os.Stat(binary); err == nil {
			return binary
		}
	}

	// Check if we already have a built binary in /tmp
	if _, err := os.Stat("/tmp/joblin"); err == nil {
		return "/tmp/joblin"
	}

	// Try common build locations
	candidates := []string{
		"../../cmd/joblin/joblin",
		"../../build/joblin",
		"joblin",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Fallback to building it
	buildCmd := exec.Command("go", "build", "-o", "/tmp/joblin", "../../cmd/joblin")
	buildCmd.Dir = "."
	if err := buildCmd.Run(); err == nil {
		if _, err := os.Stat("/tmp/joblin"); err == nil {
			return "/tmp/joblin"
		}
	}

	// Alternative build location
	buildCmd = exec.Command("go", "build", "-o", "/tmp/joblin", "./cmd/joblin")
	buildCmd.Dir = "../.."
	if err := buildCmd.Run(); err == nil {
		if _, err := os.Stat("/tmp/joblin"); err == nil {
			return "/tmp/joblin"
		}
	}

	// This will cause the test to fail, which is expected since we haven't implemented it yet
	return "joblin"
}
