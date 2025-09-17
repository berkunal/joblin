package unit

import (
	"testing"
	"time"

	"github.com/berkunal/joblin/src/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJob_ValidInputs(t *testing.T) {
	tests := []struct {
		name         string
		jobName      string
		scriptPath   string
		scriptContent []byte
		dependencies []string
	}{
		{
			name:          "simple_job",
			jobName:       "test-job",
			scriptPath:    "/path/to/script.py",
			scriptContent: []byte("print('hello')"),
			dependencies:  []string{"requests"},
		},
		{
			name:          "job_with_hyphen",
			jobName:       "data-processing-job",
			scriptPath:    "scripts/process.py",
			scriptContent: []byte("#!/usr/bin/env python3\nprint('processing')"),
			dependencies:  []string{"pandas", "numpy"},
		},
		{
			name:          "job_with_numbers",
			jobName:       "job123",
			scriptPath:    "./job.py",
			scriptContent: []byte("import sys\nprint('done')"),
			dependencies:  nil,
		},
		{
			name:          "single_char_name",
			jobName:       "a",
			scriptPath:    "a.py",
			scriptContent: []byte("1"),
			dependencies:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := models.NewJob(tt.jobName, tt.scriptPath, tt.scriptContent, tt.dependencies)

			require.NoError(t, err, "NewJob should succeed with valid inputs")
			assert.NotNil(t, job, "Job should not be nil")

			// Verify basic properties
			assert.Equal(t, tt.jobName, job.Name)
			assert.Equal(t, tt.scriptPath, job.ScriptPath)
			assert.Equal(t, tt.scriptContent, job.ScriptContent)
			assert.Equal(t, tt.dependencies, job.Dependencies)

			// Verify auto-generated properties
			assert.NotEmpty(t, job.ID, "ID should be generated")
			_, err = uuid.Parse(job.ID)
			assert.NoError(t, err, "ID should be valid UUID")

			assert.Equal(t, models.StatusPending, job.Status)
			assert.NotZero(t, job.CreatedAt)
			assert.Nil(t, job.StartedAt)
			assert.Nil(t, job.CompletedAt)
			assert.Nil(t, job.ExitCode)

			// Verify default resource limits
			assert.Equal(t, "100m", job.ResourceLimits.CPU)
			assert.Equal(t, "128Mi", job.ResourceLimits.Memory)
			assert.Equal(t, "1Gi", job.ResourceLimits.EphemeralStorage)

			// Verify default TTL
			assert.Equal(t, 24*time.Hour, job.TTL)

			// Verify Kubernetes job name generation
			assert.NotEmpty(t, job.KubernetesJobName)
			assert.Contains(t, job.KubernetesJobName, "joblin-")
			assert.Contains(t, job.KubernetesJobName, tt.jobName)

			// Verify labels map is initialized
			assert.NotNil(t, job.Labels)
			assert.Empty(t, job.Labels)

			// Validate the created job
			err = job.Validate()
			assert.NoError(t, err, "Created job should be valid")
		})
	}
}

func TestNewJob_InvalidInputs(t *testing.T) {
	validScriptContent := []byte("print('hello')")
	validDependencies := []string{"requests"}

	tests := []struct {
		name          string
		jobName       string
		scriptPath    string
		scriptContent []byte
		dependencies  []string
		expectedError string
	}{
		{
			name:          "empty_name",
			jobName:       "",
			scriptPath:    "script.py",
			scriptContent: validScriptContent,
			dependencies:  validDependencies,
			expectedError: "invalid job name",
		},
		{
			name:          "invalid_name_uppercase",
			jobName:       "TestJob",
			scriptPath:    "script.py",
			scriptContent: validScriptContent,
			dependencies:  validDependencies,
			expectedError: "invalid job name",
		},
		{
			name:          "invalid_name_underscore",
			jobName:       "test_job",
			scriptPath:    "script.py",
			scriptContent: validScriptContent,
			dependencies:  validDependencies,
			expectedError: "invalid job name",
		},
		{
			name:          "invalid_name_special_chars",
			jobName:       "test@job",
			scriptPath:    "script.py",
			scriptContent: validScriptContent,
			dependencies:  validDependencies,
			expectedError: "invalid job name",
		},
		{
			name:          "name_too_long",
			jobName:       "this-is-a-very-very-very-very-very-very-very-very-long-job-name-x",
			scriptPath:    "script.py",
			scriptContent: validScriptContent,
			dependencies:  validDependencies,
			expectedError: "invalid job name",
		},
		{
			name:          "name_starts_with_hyphen",
			jobName:       "-invalid",
			scriptPath:    "script.py",
			scriptContent: validScriptContent,
			dependencies:  validDependencies,
			expectedError: "invalid job name",
		},
		{
			name:          "name_ends_with_hyphen",
			jobName:       "invalid-",
			scriptPath:    "script.py",
			scriptContent: validScriptContent,
			dependencies:  validDependencies,
			expectedError: "invalid job name",
		},
		{
			name:          "empty_script_path",
			jobName:       "valid-job",
			scriptPath:    "",
			scriptContent: validScriptContent,
			dependencies:  validDependencies,
			expectedError: "script path cannot be empty",
		},
		{
			name:          "empty_script_content",
			jobName:       "valid-job",
			scriptPath:    "script.py",
			scriptContent: []byte{},
			dependencies:  validDependencies,
			expectedError: "script content cannot be empty",
		},
		{
			name:          "nil_script_content",
			jobName:       "valid-job",
			scriptPath:    "script.py",
			scriptContent: nil,
			dependencies:  validDependencies,
			expectedError: "script content cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := models.NewJob(tt.jobName, tt.scriptPath, tt.scriptContent, tt.dependencies)

			assert.Error(t, err, "NewJob should fail with invalid inputs")
			assert.Nil(t, job, "Job should be nil on error")
			if err != nil {
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}

func TestJob_Validate(t *testing.T) {
	t.Run("valid_job", func(t *testing.T) {
		job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), []string{"requests"})
		require.NoError(t, err)

		err = job.Validate()
		assert.NoError(t, err, "Valid job should pass validation")
	})

	t.Run("invalid_id_empty", func(t *testing.T) {
		job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
		require.NoError(t, err)

		job.ID = ""
		err = job.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "job ID cannot be empty")
	})

	t.Run("invalid_id_not_uuid", func(t *testing.T) {
		job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
		require.NoError(t, err)

		job.ID = "not-a-uuid"
		err = job.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "job ID must be valid UUID")
	})

	t.Run("invalid_name", func(t *testing.T) {
		job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
		require.NoError(t, err)

		job.Name = "Invalid_Name"
		err = job.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid job name")
	})

	t.Run("empty_script_path", func(t *testing.T) {
		job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
		require.NoError(t, err)

		job.ScriptPath = ""
		err = job.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "script path cannot be empty")
	})

	t.Run("empty_script_content", func(t *testing.T) {
		job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
		require.NoError(t, err)

		job.ScriptContent = []byte{}
		err = job.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "script content cannot be empty")
	})

	t.Run("invalid_resource_limits", func(t *testing.T) {
		job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
		require.NoError(t, err)

		job.ResourceLimits.CPU = "invalid"
		err = job.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid resource limits")
	})

	t.Run("ttl_too_short", func(t *testing.T) {
		job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
		require.NoError(t, err)

		job.TTL = 30 * time.Second
		err = job.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TTL must be at least 1 minute")
	})

	t.Run("ttl_too_long", func(t *testing.T) {
		job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
		require.NoError(t, err)

		job.TTL = 8 * 24 * time.Hour // 8 days
		err = job.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TTL cannot exceed 7 days")
	})
}

func TestJob_SetStarted(t *testing.T) {
	job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
	require.NoError(t, err)

	beforeStart := time.Now().UTC()
	job.SetStarted()
	afterStart := time.Now().UTC()

	assert.Equal(t, models.StatusRunning, job.Status)
	assert.NotNil(t, job.StartedAt)
	assert.True(t, job.StartedAt.After(beforeStart) || job.StartedAt.Equal(beforeStart))
	assert.True(t, job.StartedAt.Before(afterStart) || job.StartedAt.Equal(afterStart))
	assert.Nil(t, job.CompletedAt)
	assert.Nil(t, job.ExitCode)
}

func TestJob_SetCompleted(t *testing.T) {
	job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
	require.NoError(t, err)
	job.SetStarted()

	t.Run("success_exit_code_zero", func(t *testing.T) {
		jobCopy := *job
		beforeComplete := time.Now().UTC()
		jobCopy.SetCompleted(0)
		afterComplete := time.Now().UTC()

		assert.Equal(t, models.StatusCompleted, jobCopy.Status)
		assert.NotNil(t, jobCopy.CompletedAt)
		assert.True(t, jobCopy.CompletedAt.After(beforeComplete) || jobCopy.CompletedAt.Equal(beforeComplete))
		assert.True(t, jobCopy.CompletedAt.Before(afterComplete) || jobCopy.CompletedAt.Equal(afterComplete))
		assert.NotNil(t, jobCopy.ExitCode)
		assert.Equal(t, 0, *jobCopy.ExitCode)
	})

	t.Run("failure_exit_code_non_zero", func(t *testing.T) {
		jobCopy := *job
		jobCopy.SetCompleted(1)

		assert.Equal(t, models.StatusFailed, jobCopy.Status)
		assert.NotNil(t, jobCopy.CompletedAt)
		assert.NotNil(t, jobCopy.ExitCode)
		assert.Equal(t, 1, *jobCopy.ExitCode)
	})
}

func TestJob_SetTerminated(t *testing.T) {
	job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
	require.NoError(t, err)
	job.SetStarted()

	beforeTerminate := time.Now().UTC()
	job.SetTerminated()
	afterTerminate := time.Now().UTC()

	assert.Equal(t, models.StatusTerminated, job.Status)
	assert.NotNil(t, job.CompletedAt)
	assert.True(t, job.CompletedAt.After(beforeTerminate) || job.CompletedAt.Equal(beforeTerminate))
	assert.True(t, job.CompletedAt.Before(afterTerminate) || job.CompletedAt.Equal(afterTerminate))
	assert.Nil(t, job.ExitCode) // Terminated jobs don't have exit codes
}

func TestJob_SetFailed(t *testing.T) {
	job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
	require.NoError(t, err)
	job.SetStarted()

	beforeFail := time.Now().UTC()
	job.SetFailed(2)
	afterFail := time.Now().UTC()

	assert.Equal(t, models.StatusFailed, job.Status)
	assert.NotNil(t, job.CompletedAt)
	assert.True(t, job.CompletedAt.After(beforeFail) || job.CompletedAt.Equal(beforeFail))
	assert.True(t, job.CompletedAt.Before(afterFail) || job.CompletedAt.Equal(afterFail))
	assert.NotNil(t, job.ExitCode)
	assert.Equal(t, 2, *job.ExitCode)
}

func TestJob_IsFinished(t *testing.T) {
	job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
	require.NoError(t, err)

	// Pending job is not finished
	assert.False(t, job.IsFinished())

	// Running job is not finished
	job.SetStarted()
	assert.False(t, job.IsFinished())

	// Completed job is finished
	job.SetCompleted(0)
	assert.True(t, job.IsFinished())

	// Reset and test failed
	job.Status = models.StatusPending
	job.CompletedAt = nil
	job.ExitCode = nil
	job.SetStarted()
	job.SetFailed(1)
	assert.True(t, job.IsFinished())

	// Reset and test terminated
	job.Status = models.StatusPending
	job.CompletedAt = nil
	job.ExitCode = nil
	job.SetStarted()
	job.SetTerminated()
	assert.True(t, job.IsFinished())
}

func TestJob_Duration(t *testing.T) {
	job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
	require.NoError(t, err)

	// Not started - zero duration
	assert.Equal(t, time.Duration(0), job.Duration())

	// Started but not completed - should return time since started
	job.SetStarted()
	time.Sleep(10 * time.Millisecond) // Small delay to ensure measurable duration
	duration := job.Duration()
	assert.Greater(t, duration, time.Duration(0))
	assert.Less(t, duration, time.Second) // Should be very small

	// Completed - should return exact duration
	time.Sleep(10 * time.Millisecond)
	job.SetCompleted(0)
	completedDuration := job.Duration()
	assert.Greater(t, completedDuration, duration) // Should be longer than before
	assert.Equal(t, job.CompletedAt.Sub(*job.StartedAt), completedDuration)

	// Duration should be stable after completion
	time.Sleep(10 * time.Millisecond)
	stableDuration := job.Duration()
	assert.Equal(t, completedDuration, stableDuration)
}

func TestJob_IsExpired(t *testing.T) {
	job, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
	require.NoError(t, err)

	// Fresh job should not be expired
	assert.False(t, job.IsExpired())

	// Set TTL to very short duration and backdate creation time
	job.TTL = time.Millisecond
	job.CreatedAt = time.Now().UTC().Add(-time.Second)
	assert.True(t, job.IsExpired())

	// Set TTL to very long duration
	job.TTL = 24 * time.Hour
	job.CreatedAt = time.Now().UTC()
	assert.False(t, job.IsExpired())
}

func TestValidateJobName(t *testing.T) {
	// This tests the internal validateJobName function through NewJob
	validNames := []string{
		"a",
		"test",
		"test-job",
		"job123",
		"123job",
		"a-b-c-d-e-f-g-h-i-j-k-l-m-n-o-p-q-r-s-t-u-v-w-x-y-z-0-1-2-3-4",
	}

	for _, name := range validNames {
		t.Run("valid_"+name, func(t *testing.T) {
			_, err := models.NewJob(name, "script.py", []byte("print('test')"), nil)
			assert.NoError(t, err, "Name %s should be valid", name)
		})
	}

	invalidNames := []string{
		"",                           // empty
		"Test",                       // uppercase
		"test_job",                   // underscore
		"test@job",                   // special character
		"-test",                      // starts with hyphen
		"test-",                      // ends with hyphen
		"test job",                   // space
		"test.job",                   // dot
		"a-b-c-d-e-f-g-h-i-j-k-l-m-n-o-p-q-r-s-t-u-v-w-x-y-z-0-1-2-3-4-5-6-7", // too long (64 chars)
	}

	for _, name := range invalidNames {
		t.Run("invalid_"+name, func(t *testing.T) {
			_, err := models.NewJob(name, "script.py", []byte("print('test')"), nil)
			assert.Error(t, err, "Name %s should be invalid", name)
		})
	}
}

func TestGenerateKubernetesJobName(t *testing.T) {
	// Test through NewJob to ensure Kubernetes job name generation
	job1, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
	require.NoError(t, err)

	// Small delay to ensure different timestamps
	time.Sleep(time.Second + 10*time.Millisecond)

	job2, err := models.NewJob("test-job", "script.py", []byte("print('test')"), nil)
	require.NoError(t, err)

	// Should generate unique names even for same job name
	assert.NotEqual(t, job1.KubernetesJobName, job2.KubernetesJobName)

	// Should contain the original job name
	assert.Contains(t, job1.KubernetesJobName, "test-job")
	assert.Contains(t, job2.KubernetesJobName, "test-job")

	// Should start with joblin prefix
	assert.Contains(t, job1.KubernetesJobName, "joblin-")
	assert.Contains(t, job2.KubernetesJobName, "joblin-")
}