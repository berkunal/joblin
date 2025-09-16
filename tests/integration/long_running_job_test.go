package integration

import (
	"bufio"
	"context"
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

// T015: Long-running Job with Monitoring Test
// Tests long-running job scenarios with real-time monitoring
func TestLongRunningJobMonitoring(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	testHome := filepath.Join(tempDir, "home")
	require.NoError(t, os.MkdirAll(testHome, 0755))

	os.Setenv("HOME", testHome)
	defer os.Setenv("HOME", originalHome)

	t.Run("long_running_job_with_follow", func(t *testing.T) {
		// Create long-running script (matches quickstart.md example)
		scriptContent := `#!/usr/bin/env python3
import time
import sys

print("Long-running job started")
print("This job will run for several minutes with periodic output")

for i in range(10):
    print(f"Progress update {i+1}/10: Processing batch {i+1}")
    print(f"Current time: {time.strftime('%Y-%m-%d %H:%M:%S')}")

    # Flush output to ensure real-time visibility
    sys.stdout.flush()

    # Sleep for 30 seconds between iterations
    time.sleep(30)

    if i == 4:
        print("Halfway point reached - continuing processing...")

print("Long-running job completed successfully")
print("All batches processed")
`
		scriptPath := filepath.Join(tempDir, "long_runner.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0755))

		// Deploy the long-running job
		t.Log("Deploying long-running job...")
		deployCmd := exec.Command(getJoblinBinary(), "deploy", "long_runner.py", "--labels", "type=long-running", "--json")
		deployCmd.Dir = tempDir
		deployOutput, err := deployCmd.CombinedOutput()

		// Should fail until implementation
		assert.NotEqual(t, 0, getExitCode(err), "Deploy should fail until implementation")

		if getExitCode(err) != 0 {
			t.Log("Skipping long-running job tests - deployment failed (expected until implementation)")
			return
		}

		var deployResult DeployResponse
		require.NoError(t, json.Unmarshal(deployOutput, &deployResult))
		jobID := deployResult.JobID
		t.Logf("Deployed long-running job with ID: %s", jobID)

		// Test real-time log following
		t.Log("Testing real-time log following...")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		followCmd := exec.CommandContext(ctx, getJoblinBinary(), "logs", jobID, "--follow")
		followCmd.Dir = tempDir

		stdout, err := followCmd.StdoutPipe()
		require.NoError(t, err, "Should be able to create stdout pipe")

		err = followCmd.Start()
		require.NoError(t, err, "Follow command should start successfully")

		// Read log output in real-time
		logLines := make([]string, 0)
		reader := bufio.NewReader(stdout)

		// Read logs for 90 seconds
		logTimeout := time.After(90 * time.Second)
		done := make(chan bool)

		go func() {
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						t.Logf("Error reading logs: %v", err)
					}
					break
				}
				logLines = append(logLines, strings.TrimSpace(line))
				t.Logf("Log line: %s", strings.TrimSpace(line))
			}
			done <- true
		}()

		select {
		case <-logTimeout:
			t.Log("Log reading timeout reached")
		case <-done:
			t.Log("Log reading completed")
		}

		followCmd.Process.Kill()
		followCmd.Wait()

		// Verify we received real-time log output
		assert.Greater(t, len(logLines), 0, "Should receive log lines in real-time")

		foundStartMessage := false
		foundProgressUpdate := false
		for _, line := range logLines {
			if strings.Contains(line, "Long-running job started") {
				foundStartMessage = true
			}
			if strings.Contains(line, "Progress update") {
				foundProgressUpdate = true
			}
		}
		assert.True(t, foundStartMessage, "Should see job start message")
		assert.True(t, foundProgressUpdate, "Should see progress updates")

		// Test status monitoring during execution
		t.Log("Testing status monitoring during execution...")

		// Check status multiple times while job is running
		for i := 0; i < 3; i++ {
			statusCmd := exec.Command(getJoblinBinary(), "status", jobID, "--json")
			statusCmd.Dir = tempDir
			statusOutput, err := statusCmd.CombinedOutput()
			require.NoError(t, err, "Status check should succeed")

			var statusResult StatusResponse
			require.NoError(t, json.Unmarshal(statusOutput, &statusResult))

			t.Logf("Status check %d: %s", i+1, statusResult.Status)
			assert.Contains(t, []string{"pending", "running", "completed"}, statusResult.Status,
				"Status should be valid during execution")

			if statusResult.Status == "running" {
				assert.NotNil(t, statusResult.StartedAt, "Running job should have started_at timestamp")
			}

			time.Sleep(10 * time.Second)
		}

		// Test job termination during execution
		t.Log("Testing job termination during execution...")

		terminateCmd := exec.Command(getJoblinBinary(), "terminate", jobID, "--json")
		terminateCmd.Dir = tempDir
		terminateOutput, err := terminateCmd.CombinedOutput()
		require.NoError(t, err, "Terminate command should succeed")

		var terminateResult TerminateResponse
		require.NoError(t, json.Unmarshal(terminateOutput, &terminateResult))
		assert.Equal(t, "terminated", terminateResult.Status, "Job should be marked as terminated")

		// Verify termination took effect
		time.Sleep(5 * time.Second)

		statusCmd := exec.Command(getJoblinBinary(), "status", jobID, "--json")
		statusCmd.Dir = tempDir
		statusOutput, err := statusCmd.CombinedOutput()
		require.NoError(t, err, "Status check after termination should succeed")

		var finalStatusResult StatusResponse
		require.NoError(t, json.Unmarshal(statusOutput, &finalStatusResult))
		assert.Contains(t, []string{"terminated", "failed"}, finalStatusResult.Status,
			"Job should be terminated or failed after termination")

		if finalStatusResult.CompletedAt != nil {
			assert.NotEmpty(t, *finalStatusResult.CompletedAt, "Terminated job should have completion timestamp")
		}
	})

	t.Run("watch_status_changes", func(t *testing.T) {
		// Create medium-duration job for status watching
		scriptContent := `#!/usr/bin/env python3
import time
import sys

print("Status watch test job starting")
sys.stdout.flush()

for i in range(5):
    print(f"Status watch test - iteration {i+1}/5")
    sys.stdout.flush()
    time.sleep(10)

print("Status watch test job completed")
`
		scriptPath := filepath.Join(tempDir, "status_watch.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0755))

		// Deploy the job
		deployCmd := exec.Command(getJoblinBinary(), "deploy", "status_watch.py", "--json")
		deployCmd.Dir = tempDir
		deployOutput, err := deployCmd.CombinedOutput()

		// Should fail until implementation
		assert.NotEqual(t, 0, getExitCode(err), "Deploy should fail until implementation")

		if getExitCode(err) != 0 {
			t.Log("Skipping status watch test - deployment failed (expected until implementation)")
			return
		}

		var deployResult DeployResponse
		require.NoError(t, json.Unmarshal(deployOutput, &deployResult))
		jobID := deployResult.JobID

		// Test --watch flag for continuous status monitoring
		t.Log("Testing continuous status monitoring with --watch...")

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		watchCmd := exec.CommandContext(ctx, getJoblinBinary(), "status", jobID, "--watch", "--json")
		watchCmd.Dir = tempDir

		stdout, err := watchCmd.StdoutPipe()
		require.NoError(t, err, "Should be able to create stdout pipe for watch")

		err = watchCmd.Start()
		require.NoError(t, err, "Watch command should start successfully")

		// Monitor status changes
		statusUpdates := make([]StatusResponse, 0)
		reader := bufio.NewReader(stdout)

		watchTimeout := time.After(50 * time.Second)
		done := make(chan bool)

		go func() {
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break
				}

				var statusResult StatusResponse
				if json.Unmarshal([]byte(line), &statusResult) == nil {
					statusUpdates = append(statusUpdates, statusResult)
					t.Logf("Status update: %s", statusResult.Status)
				}
			}
			done <- true
		}()

		select {
		case <-watchTimeout:
			t.Log("Status watch timeout reached")
		case <-done:
			t.Log("Status watching completed")
		}

		watchCmd.Process.Kill()
		watchCmd.Wait()

		// Verify we received status updates
		assert.Greater(t, len(statusUpdates), 0, "Should receive status updates")

		// Verify status progression
		statusProgression := make([]string, 0)
		for _, update := range statusUpdates {
			statusProgression = append(statusProgression, update.Status)
		}

		t.Logf("Status progression: %v", statusProgression)

		// Should see at least pending->running or pending->completed progression
		if len(statusProgression) > 1 {
			firstStatus := statusProgression[0]
			lastStatus := statusProgression[len(statusProgression)-1]

			assert.Contains(t, []string{"pending", "running"}, firstStatus, "First status should be pending or running")
			assert.Contains(t, []string{"running", "completed", "terminated"}, lastStatus,
				"Last status should be running, completed, or terminated")
		}
	})

	t.Run("ttl_based_cleanup", func(t *testing.T) {
		// Test TTL-based automatic cleanup
		scriptContent := `import time
print("TTL cleanup test job")
time.sleep(1)
print("Job completed for TTL testing")
`
		scriptPath := filepath.Join(tempDir, "ttl_test.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0755))

		// Deploy job with short TTL
		deployCmd := exec.Command(getJoblinBinary(), "deploy", "ttl_test.py", "--ttl", "30s", "--json")
		deployCmd.Dir = tempDir
		deployOutput, err := deployCmd.CombinedOutput()

		// Should fail until implementation
		assert.NotEqual(t, 0, getExitCode(err), "Deploy should fail until implementation")

		if getExitCode(err) != 0 {
			t.Log("Skipping TTL cleanup test - deployment failed (expected until implementation)")
			return
		}

		var deployResult DeployResponse
		require.NoError(t, json.Unmarshal(deployOutput, &deployResult))
		jobID := deployResult.JobID

		// Wait for job to complete
		timeout := time.After(30 * time.Second)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				t.Fatal("Job did not complete within timeout")
			case <-ticker.C:
				statusCmd := exec.Command(getJoblinBinary(), "status", jobID, "--json")
				statusCmd.Dir = tempDir
				statusOutput, err := statusCmd.CombinedOutput()
				if err == nil {
					var statusResult StatusResponse
					if json.Unmarshal(statusOutput, &statusResult) == nil {
						if statusResult.Status == "completed" {
							goto jobCompleted
						}
					}
				}
			}
		}
	jobCompleted:

		// Wait for TTL cleanup to take effect (30s TTL + buffer)
		t.Log("Waiting for TTL cleanup to take effect...")
		time.Sleep(40 * time.Second)

		// Verify job was cleaned up
		statusCmd := exec.Command(getJoblinBinary(), "status", jobID, "--json")
		statusCmd.Dir = tempDir
		statusOutput, err := statusCmd.CombinedOutput()

		// Job should either not be found or marked for cleanup
		if err != nil {
			// Job not found is expected after TTL cleanup
			assert.Contains(t, string(statusOutput), "not found", "Job should be cleaned up after TTL")
		} else {
			// If job still exists, it should be marked for cleanup
			var statusResult StatusResponse
			if json.Unmarshal(statusOutput, &statusResult) == nil {
				t.Logf("Job still exists with status: %s", statusResult.Status)
			}
		}

		// Verify job doesn't appear in active job list
		listCmd := exec.Command(getJoblinBinary(), "list", "--json")
		listCmd.Dir = tempDir
		listOutput, err := listCmd.CombinedOutput()
		require.NoError(t, err, "List command should succeed")

		var listResult ListResponse
		require.NoError(t, json.Unmarshal(listOutput, &listResult))

		jobFound := false
		for _, job := range listResult.Jobs {
			if job.JobID == jobID {
				jobFound = true
				break
			}
		}
		assert.False(t, jobFound, "TTL-expired job should not appear in active job list")
	})

	t.Run("resource_usage_monitoring", func(t *testing.T) {
		// Test resource usage monitoring for long-running jobs
		scriptContent := `#!/usr/bin/env python3
import time
import sys

print("Resource monitoring test starting")
sys.stdout.flush()

# Simulate some CPU and memory usage
data = []
for i in range(10):
    print(f"Resource test iteration {i+1}/10")

    # Allocate some memory
    chunk = [0] * 1000000  # ~8MB
    data.append(chunk)

    # Some CPU work
    result = sum(range(100000))

    print(f"Allocated memory chunk {i+1}, sum result: {result}")
    sys.stdout.flush()

    time.sleep(5)

print("Resource monitoring test completed")
`
		scriptPath := filepath.Join(tempDir, "resource_test.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0755))

		// Deploy with specific resource limits
		deployCmd := exec.Command(getJoblinBinary(), "deploy", "resource_test.py",
			"--cpu", "500m", "--memory", "256Mi", "--json")
		deployCmd.Dir = tempDir
		deployOutput, err := deployCmd.CombinedOutput()

		// Should fail until implementation
		assert.NotEqual(t, 0, getExitCode(err), "Deploy should fail until implementation")

		if getExitCode(err) != 0 {
			t.Log("Skipping resource monitoring test - deployment failed (expected until implementation)")
			return
		}

		var deployResult DeployResponse
		require.NoError(t, json.Unmarshal(deployOutput, &deployResult))
		jobID := deployResult.JobID

		// Monitor resource usage during execution
		t.Log("Monitoring resource usage...")

		for i := 0; i < 5; i++ {
			// Check if status includes resource usage information
			statusCmd := exec.Command(getJoblinBinary(), "status", jobID, "--verbose", "--json")
			statusCmd.Dir = tempDir
			statusOutput, err := statusCmd.CombinedOutput()
			require.NoError(t, err, "Status with verbose should succeed")

			var statusResult StatusResponse
			require.NoError(t, json.Unmarshal(statusOutput, &statusResult))

			t.Logf("Status check %d - Job: %s", i+1, statusResult.Status)

			// If implemented, status might include resource usage metrics
			// This would be implementation-specific

			time.Sleep(10 * time.Second)
		}

		// Clean up
		terminateCmd := exec.Command(getJoblinBinary(), "terminate", jobID)
		terminateCmd.Dir = tempDir
		terminateCmd.CombinedOutput()
	})
}
