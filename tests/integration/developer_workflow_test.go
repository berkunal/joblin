package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T012: Developer Workflow Scenario Test
// Tests realistic developer workflow with dependencies and notifications
func TestDeveloperWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	testHome := filepath.Join(tempDir, "home")
	require.NoError(t, os.MkdirAll(testHome, 0755))

	os.Setenv("HOME", testHome)
	defer os.Setenv("HOME", originalHome)

	t.Run("complete_developer_workflow", func(t *testing.T) {
		// Setup: Create Python script with dependencies
		t.Log("Setting up Python script with dependencies")
		scriptContent := `#!/usr/bin/env python3
import requests
import datetime
import time

print("Starting data processing job...")
print(f"Current time: {datetime.datetime.now()}")

# Simulate API call with requests library
try:
    # Mock API call - this would normally hit a real endpoint
    print("Processing data with external dependencies...")
    time.sleep(2)  # Simulate processing time
    print("Data processing completed successfully!")
    print(f"Job completed at: {datetime.datetime.now()}")
except Exception as e:
    print(f"Error during processing: {e}")
    exit(1)
`
		scriptPath := filepath.Join(tempDir, "data_processor.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0755))

		// Create requirements.txt
		requirementsContent := `requests==2.31.0
python-dateutil==2.8.2
`
		requirementsPath := filepath.Join(tempDir, "requirements.txt")
		require.NoError(t, os.WriteFile(requirementsPath, []byte(requirementsContent), 0644))

		// Setup mock Teams webhook server
		webhookReceived := make(chan bool, 1)
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method, "Webhook should use POST method")
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "Webhook should send JSON")

			// Validate webhook payload structure
			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err, "Webhook payload should be valid JSON")

			// Validate Teams MessageCard format
			assert.Equal(t, "MessageCard", payload["@type"], "Should be Teams MessageCard format")
			assert.Contains(t, payload, "summary", "Should contain summary field")
			assert.Contains(t, payload, "sections", "Should contain sections field")

			w.WriteHeader(http.StatusOK)
			select {
			case webhookReceived <- true:
			default:
			}
		}))
		defer mockServer.Close()

		// Step 1: Deploy with custom resources and webhook
		t.Log("Step 1: Deploying with custom resources and webhook")
		deployArgs := []string{
			"deploy", "data_processor.py",
			"--requirements", "requirements.txt",
			"--cpu", "200m",
			"--memory", "256Mi",
			"--webhook", mockServer.URL,
			"--labels", "env=test,team=backend",
			"--output", "json",
		}
		deployCmd := exec.Command(getJoblinBinary(), deployArgs...)
		deployCmd.Dir = tempDir
		deployOutput, err := deployCmd.CombinedOutput()

		// Should fail until implementation is complete
		assert.NotEqual(t, 0, getExitCode(err), "Deploy should fail until implementation. Output: %s", string(deployOutput))

		// If this were working, validate the workflow:
		if getExitCode(err) == 0 {
			var deployResult DeployResponse
			require.NoError(t, json.Unmarshal(deployOutput, &deployResult))
			jobID := deployResult.JobID

			// Validate deployment response
			assert.NotEmpty(t, jobID, "Should return job ID")
			assert.Equal(t, "deployed", deployResult.Status, "Should be in deployed status")

			// Step 2: Monitor job progress
			t.Log("Step 2: Monitoring job progress")
			timeout := time.After(5 * time.Minute) // Longer timeout for dependency installation
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()

			var jobCompleted bool
			for !jobCompleted {
				select {
				case <-timeout:
					t.Fatal("Job did not complete within 5 minutes")
				case <-ticker.C:
					statusCmd := exec.Command(getJoblinBinary(), "status", jobID, "--output", "json")
					statusCmd.Dir = tempDir
					statusOutput, err := statusCmd.CombinedOutput()
					if err == nil {
						var statusResult StatusResponse
						if json.Unmarshal(statusOutput, &statusResult) == nil {
							t.Logf("Job status: %s", statusResult.Status)
							if statusResult.Status == "completed" || statusResult.Status == "failed" {
								jobCompleted = true
								assert.Equal(t, "completed", statusResult.Status, "Job should complete successfully")
							}
						}
					}
				}
			}

			// Step 3: Verify dependency installation and execution
			t.Log("Step 3: Verifying job execution and dependencies")
			logsCmd := exec.Command(getJoblinBinary(), "logs", jobID)
			logsCmd.Dir = tempDir
			logsOutput, err := logsCmd.CombinedOutput()
			require.NoError(t, err, "Logs command should succeed")

			logsStr := string(logsOutput)
			assert.Contains(t, logsStr, "Starting data processing job", "Should show job started")
			assert.Contains(t, logsStr, "Processing data with external dependencies", "Should show dependency usage")
			assert.Contains(t, logsStr, "Data processing completed successfully", "Should complete successfully")

			// Verify pip install occurred (should show in logs)
			assert.Contains(t, logsStr, "requests", "Should show requests library installation or usage")

			// Step 4: Verify Teams webhook notification
			t.Log("Step 4: Verifying Teams webhook notification")
			select {
			case <-webhookReceived:
				t.Log("Teams webhook notification received successfully")
			case <-time.After(30 * time.Second):
				t.Error("Teams webhook notification not received within 30 seconds")
			}

			// Step 5: List jobs with filtering
			t.Log("Step 5: Testing job listing with filters")
			listCmd := exec.Command(getJoblinBinary(), "list", "--labels", "env=test", "--output", "json")
			listCmd.Dir = tempDir
			listOutput, err := listCmd.CombinedOutput()
			require.NoError(t, err, "List command should succeed")

			var listResult ListResponse
			require.NoError(t, json.Unmarshal(listOutput, &listResult))
			assert.Len(t, listResult.Jobs, 1, "Should find exactly one job with env=test label")
			assert.Equal(t, jobID, listResult.Jobs[0].JobID, "Should find the correct job")

			// Step 6: Test cleanup functionality
			t.Log("Step 6: Testing cleanup functionality")
			cleanupCmd := exec.Command(getJoblinBinary(), "cleanup", "--status", "completed", "--dry-run", "--output", "json")
			cleanupCmd.Dir = tempDir
			cleanupOutput, err := cleanupCmd.CombinedOutput()
			require.NoError(t, err, "Cleanup dry-run should succeed")

			var cleanupResult CleanupResponse
			require.NoError(t, json.Unmarshal(cleanupOutput, &cleanupResult))
			assert.Greater(t, len(cleanupResult.JobsToCleanup), 0, "Should identify jobs for cleanup")
		}
	})

	t.Run("resource_limits_validation", func(t *testing.T) {
		// Test various resource limit formats
		testCases := []struct {
			name        string
			cpu         string
			memory      string
			shouldFail  bool
			expectedMsg string
		}{
			{"valid_millicores", "100m", "128Mi", false, ""},
			{"valid_cores", "0.5", "1Gi", false, ""},
			{"invalid_cpu_format", "abc", "128Mi", true, "Invalid CPU format"},
			{"invalid_memory_format", "100m", "xyz", true, "Invalid memory format"},
			{"negative_cpu", "-100m", "128Mi", true, "CPU must be positive"},
			{"zero_memory", "100m", "0", true, "Memory must be positive"},
		}

		scriptPath := filepath.Join(tempDir, "simple.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte("print('test')"), 0755))

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				deployArgs := []string{
					"deploy", "simple.py",
					"--cpu", tc.cpu,
					"--memory", tc.memory,
					"--output", "json",
				}
				deployCmd := exec.Command(getJoblinBinary(), deployArgs...)
				deployCmd.Dir = tempDir
				deployOutput, err := deployCmd.CombinedOutput()

				if tc.shouldFail {
					assert.NotEqual(t, 0, getExitCode(err), "Should fail for invalid resource: %s", tc.name)
					if tc.expectedMsg != "" {
						assert.Contains(t, string(deployOutput), tc.expectedMsg, "Should contain expected error message")
					}
				} else {
					// All should fail until implementation, but for different reasons
					assert.NotEqual(t, 0, getExitCode(err), "Should fail until implementation")
				}
			})
		}
	})
}

// ListResponse represents the JSON response from joblin list command
type ListResponse struct {
	Jobs []JobSummary `json:"jobs"`
}

// JobSummary represents a job in the list response
type JobSummary struct {
	JobID       string            `json:"job_id"`
	Status      string            `json:"status"`
	Namespace   string            `json:"namespace"`
	CreatedAt   string            `json:"created_at"`
	CompletedAt *string           `json:"completed_at,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// CleanupResponse represents the JSON response from joblin cleanup command
type CleanupResponse struct {
	JobsToCleanup []string `json:"jobs_to_cleanup"`
	DryRun        bool     `json:"dry_run"`
	CleanedCount  int      `json:"cleaned_count"`
}
