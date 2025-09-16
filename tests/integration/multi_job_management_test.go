package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T014: Multi-Job Management Scenario Test
// Tests concurrent job management and filtering capabilities
func TestMultiJobManagement(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	testHome := filepath.Join(tempDir, "home")
	require.NoError(t, os.MkdirAll(testHome, 0755))

	os.Setenv("HOME", testHome)
	defer os.Setenv("HOME", originalHome)

	t.Run("concurrent_job_deployment", func(t *testing.T) {
		// Create multiple test scripts with different characteristics
		scripts := []struct {
			name     string
			content  string
			labels   map[string]string
			duration int // seconds
		}{
			{
				"quick_job.py",
				"import time\nprint('Quick job starting')\ntime.sleep(1)\nprint('Quick job completed')",
				map[string]string{"env": "test", "priority": "high", "team": "backend"},
				1,
			},
			{
				"medium_job.py",
				"import time\nprint('Medium job starting')\ntime.sleep(5)\nprint('Medium job completed')",
				map[string]string{"env": "staging", "priority": "medium", "team": "frontend"},
				5,
			},
			{
				"long_job.py",
				"import time\nprint('Long job starting')\nfor i in range(3):\n    print(f'Long job progress: {i+1}/3')\n    time.sleep(3)\nprint('Long job completed')",
				map[string]string{"env": "prod", "priority": "low", "team": "data"},
				9,
			},
			{
				"batch_job.py",
				"import time\nprint('Batch job starting')\nfor i in range(2):\n    print(f'Processing batch {i+1}')\n    time.sleep(2)\nprint('Batch job completed')",
				map[string]string{"env": "test", "priority": "medium", "team": "backend"},
				4,
			},
		}

		// Create all script files
		for _, script := range scripts {
			scriptPath := filepath.Join(tempDir, script.name)
			require.NoError(t, os.WriteFile(scriptPath, []byte(script.content), 0755))
		}

		// Deploy all jobs concurrently
		t.Log("Deploying multiple jobs concurrently...")
		jobIDs := make([]string, 0, len(scripts))

		for _, script := range scripts {
			// Build labels argument
			var labelArgs []string
			for k, v := range script.labels {
				labelArgs = append(labelArgs, fmt.Sprintf("%s=%s", k, v))
			}
			labelsStr := strings.Join(labelArgs, ",")

			deployArgs := []string{
				"deploy", script.name,
				"--labels", labelsStr,
				"--json",
			}

			deployCmd := exec.Command(getJoblinBinary(), deployArgs...)
			deployCmd.Dir = tempDir
			deployOutput, err := deployCmd.CombinedOutput()

			// Should fail until implementation
			assert.NotEqual(t, 0, getExitCode(err), "Deploy should fail until implementation")

			// If implemented, extract job ID
			if getExitCode(err) == 0 {
				var deployResult DeployResponse
				require.NoError(t, json.Unmarshal(deployOutput, &deployResult))
				jobIDs = append(jobIDs, deployResult.JobID)
				t.Logf("Deployed %s with job ID: %s", script.name, deployResult.JobID)
			}
		}

		// Only continue if jobs were actually deployed
		if len(jobIDs) == 0 {
			t.Log("Skipping remaining tests - no jobs deployed (expected until implementation)")
			return
		}

		// Test job listing and filtering
		t.Log("Testing job listing and filtering...")

		// Test 1: List all jobs
		listCmd := exec.Command(getJoblinBinary(), "list", "--json")
		listCmd.Dir = tempDir
		listOutput, err := listCmd.CombinedOutput()
		require.NoError(t, err, "List all jobs should succeed")

		var allJobsResult ListResponse
		require.NoError(t, json.Unmarshal(listOutput, &allJobsResult))
		assert.GreaterOrEqual(t, len(allJobsResult.Jobs), len(jobIDs), "Should list all deployed jobs")

		// Test 2: Filter by environment
		listEnvCmd := exec.Command(getJoblinBinary(), "list", "--labels", "env=test", "--json")
		listEnvCmd.Dir = tempDir
		listEnvOutput, err := listEnvCmd.CombinedOutput()
		require.NoError(t, err, "List with env filter should succeed")

		var envJobsResult ListResponse
		require.NoError(t, json.Unmarshal(listEnvOutput, &envJobsResult))
		assert.Equal(t, 2, len(envJobsResult.Jobs), "Should find 2 jobs with env=test")

		// Test 3: Filter by team
		listTeamCmd := exec.Command(getJoblinBinary(), "list", "--labels", "team=backend", "--json")
		listTeamCmd.Dir = tempDir
		listTeamOutput, err := listTeamCmd.CombinedOutput()
		require.NoError(t, err, "List with team filter should succeed")

		var teamJobsResult ListResponse
		require.NoError(t, json.Unmarshal(listTeamOutput, &teamJobsResult))
		assert.Equal(t, 2, len(teamJobsResult.Jobs), "Should find 2 jobs with team=backend")

		// Test 4: Filter by status
		time.Sleep(2 * time.Second) // Wait for quick job to complete

		listRunningCmd := exec.Command(getJoblinBinary(), "list", "--status", "running", "--json")
		listRunningCmd.Dir = tempDir
		listRunningOutput, err := listRunningCmd.CombinedOutput()
		require.NoError(t, err, "List running jobs should succeed")

		var runningJobsResult ListResponse
		require.NoError(t, json.Unmarshal(listRunningOutput, &runningJobsResult))
		assert.GreaterOrEqual(t, len(runningJobsResult.Jobs), 2, "Should have at least 2 running jobs")

		// Test 5: Filter by time
		listRecentCmd := exec.Command(getJoblinBinary(), "list", "--since", "1m", "--json")
		listRecentCmd.Dir = tempDir
		listRecentOutput, err := listRecentCmd.CombinedOutput()
		require.NoError(t, err, "List recent jobs should succeed")

		var recentJobsResult ListResponse
		require.NoError(t, json.Unmarshal(listRecentOutput, &recentJobsResult))
		assert.GreaterOrEqual(t, len(recentJobsResult.Jobs), len(jobIDs), "Should find all recently created jobs")

		// Test job termination
		t.Log("Testing job termination...")

		// Terminate the long job (should still be running)
		longJobID := ""
		for i, script := range scripts {
			if script.name == "long_job.py" && i < len(jobIDs) {
				longJobID = jobIDs[i]
				break
			}
		}

		if longJobID != "" {
			terminateCmd := exec.Command(getJoblinBinary(), "terminate", longJobID, "--json")
			terminateCmd.Dir = tempDir
			terminateOutput, err := terminateCmd.CombinedOutput()
			require.NoError(t, err, "Terminate should succeed")

			var terminateResult TerminateResponse
			require.NoError(t, json.Unmarshal(terminateOutput, &terminateResult))
			assert.Equal(t, "terminated", terminateResult.Status, "Job should be terminated")

			// Verify termination took effect
			time.Sleep(2 * time.Second)
			statusCmd := exec.Command(getJoblinBinary(), "status", longJobID, "--json")
			statusCmd.Dir = tempDir
			statusOutput, err := statusCmd.CombinedOutput()
			require.NoError(t, err, "Status check should succeed")

			var statusResult StatusResponse
			require.NoError(t, json.Unmarshal(statusOutput, &statusResult))
			assert.Contains(t, []string{"terminated", "failed"}, statusResult.Status, "Job should be terminated or failed")
		}

		// Test log retrieval for multiple jobs
		t.Log("Testing log retrieval for multiple jobs...")

		for i, jobID := range jobIDs {
			logsCmd := exec.Command(getJoblinBinary(), "logs", jobID)
			logsCmd.Dir = tempDir
			logsOutput, err := logsCmd.CombinedOutput()
			require.NoError(t, err, "Logs command should succeed for job %s", jobID)

			logsStr := string(logsOutput)
			scriptName := scripts[i].name
			expectedPrefix := strings.TrimSuffix(scriptName, ".py")
			assert.Contains(t, logsStr, fmt.Sprintf("%s job", strings.Title(expectedPrefix)),
				"Logs should contain expected job-specific content for %s", scriptName)
		}

		// Test cleanup with filtering
		t.Log("Testing cleanup with filtering...")

		// Wait for some jobs to complete
		time.Sleep(8 * time.Second)

		// Dry run cleanup of completed jobs
		cleanupCmd := exec.Command(getJoblinBinary(), "cleanup", "--status", "completed", "--dry-run", "--json")
		cleanupCmd.Dir = tempDir
		cleanupOutput, err := cleanupCmd.CombinedOutput()
		require.NoError(t, err, "Cleanup dry-run should succeed")

		var cleanupResult CleanupResponse
		require.NoError(t, json.Unmarshal(cleanupOutput, &cleanupResult))
		assert.True(t, cleanupResult.DryRun, "Should indicate dry-run mode")
		assert.GreaterOrEqual(t, len(cleanupResult.JobsToCleanup), 1, "Should identify at least one job for cleanup")

		// Actual cleanup of completed jobs older than 30 seconds
		cleanupActualCmd := exec.Command(getJoblinBinary(), "cleanup", "--older-than", "30s", "--status", "completed", "--json")
		cleanupActualCmd.Dir = tempDir
		cleanupActualOutput, err := cleanupActualCmd.CombinedOutput()
		require.NoError(t, err, "Actual cleanup should succeed")

		var cleanupActualResult CleanupResponse
		require.NoError(t, json.Unmarshal(cleanupActualOutput, &cleanupActualResult))
		assert.False(t, cleanupActualResult.DryRun, "Should not be dry-run mode")
		assert.GreaterOrEqual(t, cleanupActualResult.CleanedCount, 0, "Should report number of cleaned jobs")
	})

	t.Run("job_state_tracking", func(t *testing.T) {
		// Test BBolt storage consistency across operations
		scriptContent := `import time
print("State tracking test job")
time.sleep(2)
print("Job completed")
`
		scriptPath := filepath.Join(tempDir, "state_test.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0755))

		// Deploy job
		deployCmd := exec.Command(getJoblinBinary(), "deploy", "state_test.py", "--labels", "test=state-tracking", "--json")
		deployCmd.Dir = tempDir
		deployOutput, err := deployCmd.CombinedOutput()

		// Should fail until implementation
		assert.NotEqual(t, 0, getExitCode(err), "Deploy should fail until implementation")

		if getExitCode(err) == 0 {
			var deployResult DeployResponse
			require.NoError(t, json.Unmarshal(deployOutput, &deployResult))
			jobID := deployResult.JobID

			// Check initial state
			statusCmd := exec.Command(getJoblinBinary(), "status", jobID, "--json")
			statusCmd.Dir = tempDir
			statusOutput, err := statusCmd.CombinedOutput()
			require.NoError(t, err, "Initial status check should succeed")

			var statusResult StatusResponse
			require.NoError(t, json.Unmarshal(statusOutput, &statusResult))
			assert.Equal(t, "pending", statusResult.Status, "Initial status should be pending")

			// Wait and check running state
			time.Sleep(2 * time.Second)
			statusCmd = exec.Command(getJoblinBinary(), "status", jobID, "--json")
			statusCmd.Dir = tempDir
			statusOutput, err = statusCmd.CombinedOutput()
			require.NoError(t, err, "Running status check should succeed")

			require.NoError(t, json.Unmarshal(statusOutput, &statusResult))
			assert.Contains(t, []string{"running", "completed"}, statusResult.Status, "Should be running or completed")

			// Verify job appears in list
			listCmd := exec.Command(getJoblinBinary(), "list", "--labels", "test=state-tracking", "--json")
			listCmd.Dir = tempDir
			listOutput, err := listCmd.CombinedOutput()
			require.NoError(t, err, "List with label filter should succeed")

			var listResult ListResponse
			require.NoError(t, json.Unmarshal(listOutput, &listResult))
			assert.Len(t, listResult.Jobs, 1, "Should find exactly one job with test=state-tracking label")
			assert.Equal(t, jobID, listResult.Jobs[0].JobID, "Should find the correct job")
		}
	})

	t.Run("concurrent_operations", func(t *testing.T) {
		// Test concurrent operations on the same job
		scriptContent := `import time
print("Concurrent operations test")
time.sleep(5)
print("Test completed")
`
		scriptPath := filepath.Join(tempDir, "concurrent_test.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0755))

		// Deploy job
		deployCmd := exec.Command(getJoblinBinary(), "deploy", "concurrent_test.py", "--json")
		deployCmd.Dir = tempDir
		deployOutput, err := deployCmd.CombinedOutput()

		// Should fail until implementation
		assert.NotEqual(t, 0, getExitCode(err), "Deploy should fail until implementation")

		if getExitCode(err) == 0 {
			var deployResult DeployResponse
			require.NoError(t, json.Unmarshal(deployOutput, &deployResult))
			jobID := deployResult.JobID

			// Perform concurrent operations
			operations := []struct {
				name string
				cmd  []string
			}{
				{"status", []string{"status", jobID, "--json"}},
				{"logs", []string{"logs", jobID}},
				{"status_again", []string{"status", jobID, "--json"}},
			}

			results := make(chan error, len(operations))

			// Execute operations concurrently
			for _, op := range operations {
				go func(operation []string) {
					cmd := exec.Command(getJoblinBinary(), operation...)
					cmd.Dir = tempDir
					_, err := cmd.CombinedOutput()
					results <- err
				}(op.cmd)
			}

			// Collect results
			for i := 0; i < len(operations); i++ {
				err := <-results
				assert.NoError(t, err, "Concurrent operation %d should succeed", i)
			}

			// Clean up
			terminateCmd := exec.Command(getJoblinBinary(), "terminate", jobID)
			terminateCmd.Dir = tempDir
			terminateCmd.CombinedOutput()
		}
	})
}

// TerminateResponse represents the JSON response from joblin terminate command
type TerminateResponse struct {
	JobID     string `json:"job_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}
