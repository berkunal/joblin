package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T011: First-time User Setup Scenario Test
// Tests the complete first-time user experience from installation to job completion
func TestFirstTimeUserSetup(t *testing.T) {
	// Create isolated test environment
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	testHome := filepath.Join(tempDir, "home")
	require.NoError(t, os.MkdirAll(testHome, 0755))

	// Set temporary HOME for config isolation
	os.Setenv("HOME", testHome)
	defer os.Setenv("HOME", originalHome)

	t.Run("complete_first_user_workflow", func(t *testing.T) {
		// Step 1: Initialize configuration
		t.Log("Step 1: Running 'joblin config init'")
		configCmd := exec.Command(getJoblinBinary(), "config", "init")
		configCmd.Dir = tempDir
		configOutput, err := configCmd.CombinedOutput()

		// Should fail until T030 is implemented
		assert.NotEqual(t, 0, getExitCode(err), "Config init should fail until implementation. Output: %s", string(configOutput))

		// Step 2: Create hello.py script
		t.Log("Step 2: Creating hello.py script")
		scriptContent := `#!/usr/bin/env python3
import time
print("Hello from Kubernetes!")
print(f"Job started at: {time.strftime('%Y-%m-%d %H:%M:%S')}")
print("First-time user setup test completed successfully!")
`
		scriptPath := filepath.Join(tempDir, "hello.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0755))

		// Step 3: Deploy the script
		t.Log("Step 3: Deploying hello.py with 'joblin deploy'")
		deployCmd := exec.Command(getJoblinBinary(), "deploy", "hello.py", "--output", "json")
		deployCmd.Dir = tempDir
		deployOutput, err := deployCmd.CombinedOutput()

		// Should fail until T016 is implemented
		assert.NotEqual(t, 0, getExitCode(err), "Deploy should fail until implementation. Output: %s", string(deployOutput))

		// If this were working, we would validate:
		// - JSON output contains job_id field
		// - Configuration file exists at ~/.joblin/config.yaml
		// - Job deploys to default namespace
		if getExitCode(err) == 0 {
			var deployResult DeployResponse
			require.NoError(t, json.Unmarshal(deployOutput, &deployResult), "Deploy output should be valid JSON")
			assert.NotEmpty(t, deployResult.JobID, "Deploy should return job ID")
			assert.Equal(t, "deployed", deployResult.Status, "Job should be in deployed status")

			// Verify config file creation
			configPath := filepath.Join(testHome, ".joblin", "config.yaml")
			assert.FileExists(t, configPath, "Configuration file should be created")

			jobID := deployResult.JobID

			// Step 4: Check job status
			t.Log("Step 4: Checking job status with 'joblin status'")
			statusCmd := exec.Command(getJoblinBinary(), "status", jobID, "--output", "json")
			statusCmd.Dir = tempDir
			statusOutput, err := statusCmd.CombinedOutput()
			require.NoError(t, err, "Status command should succeed")

			var statusResult StatusResponse
			require.NoError(t, json.Unmarshal(statusOutput, &statusResult))
			assert.Contains(t, []string{"pending", "running", "completed"}, statusResult.Status)

			// Wait for job completion (max 2 minutes)
			t.Log("Waiting for job completion (max 2 minutes)")
			timeout := time.After(2 * time.Minute)
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			var finalStatus string
			for {
				select {
				case <-timeout:
					t.Fatal("Job did not complete within 2 minutes")
				case <-ticker.C:
					statusCmd := exec.Command(getJoblinBinary(), "status", jobID, "--output", "json")
					statusCmd.Dir = tempDir
					statusOutput, err := statusCmd.CombinedOutput()
					if err == nil {
						var statusResult StatusResponse
						if json.Unmarshal(statusOutput, &statusResult) == nil {
							finalStatus = statusResult.Status
							if finalStatus == "completed" || finalStatus == "failed" {
								goto statusComplete
							}
						}
					}
				}
			}
		statusComplete:

			assert.Equal(t, "completed", finalStatus, "Job should complete successfully")

			// Step 5: View job logs
			t.Log("Step 5: Viewing job logs with 'joblin logs'")
			logsCmd := exec.Command(getJoblinBinary(), "logs", jobID)
			logsCmd.Dir = tempDir
			logsOutput, err := logsCmd.CombinedOutput()
			require.NoError(t, err, "Logs command should succeed")

			logsStr := string(logsOutput)
			assert.Contains(t, logsStr, "Hello from Kubernetes!", "Logs should contain expected message")
			assert.Contains(t, logsStr, "First-time user setup test completed successfully!", "Logs should contain test completion message")
		}
	})
}

// DeployResponse represents the JSON response from joblin deploy command
type DeployResponse struct {
	JobID     string `json:"job_id"`
	Status    string `json:"status"`
	Namespace string `json:"namespace"`
	CreatedAt string `json:"created_at"`
}

// StatusResponse represents the JSON response from joblin status command
type StatusResponse struct {
	JobID       string            `json:"job_id"`
	Status      string            `json:"status"`
	Namespace   string            `json:"namespace"`
	CreatedAt   string            `json:"created_at"`
	StartedAt   *string           `json:"started_at,omitempty"`
	CompletedAt *string           `json:"completed_at,omitempty"`
	ExitCode    *int              `json:"exit_code,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// getJoblinBinary returns the path to the joblin binary for testing
func getJoblinBinary() string {
	// Look for binary in common locations
	candidates := []string{
		"./joblin",
		"../joblin",
		"../../joblin",
		"./bin/joblin",
		"../bin/joblin",
		"../../bin/joblin",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Fallback to assuming it's in PATH
	return "joblin"
}

// getExitCode extracts the exit code from a command error
func getExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		return exitError.ExitCode()
	}
	return -1
}