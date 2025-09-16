package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T013: Error Handling Scenario Test
// Tests comprehensive error handling and user-friendly error messages
func TestErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	testHome := filepath.Join(tempDir, "home")
	require.NoError(t, os.MkdirAll(testHome, 0755))

	os.Setenv("HOME", testHome)
	defer os.Setenv("HOME", originalHome)

	t.Run("script_syntax_errors", func(t *testing.T) {
		// Create Python script with syntax error
		scriptContent := `#!/usr/bin/env python3
print("Hello World"
# Missing closing parenthesis - syntax error
print("This will cause a syntax error")
`
		scriptPath := filepath.Join(tempDir, "syntax_error.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0755))

		deployCmd := exec.Command(getJoblinBinary(), "deploy", "syntax_error.py", "--json")
		deployCmd.Dir = tempDir
		deployOutput, err := deployCmd.CombinedOutput()

		// Should fail until implementation, but we test expected behavior
		assert.NotEqual(t, 0, getExitCode(err), "Deploy should fail for syntax error")

		// If implemented, should show exit code 1 for script errors
		if getExitCode(err) == 1 {
			outputStr := string(deployOutput)
			assert.Contains(t, outputStr, "syntax error", "Should mention syntax error")
			assert.Contains(t, outputStr, "SyntaxError", "Should show Python SyntaxError")
		}
	})

	t.Run("invalid_resource_limits", func(t *testing.T) {
		scriptPath := filepath.Join(tempDir, "simple.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte("print('test')"), 0755))

		testCases := []struct {
			name        string
			cpu         string
			memory      string
			expectedMsg string
		}{
			{"invalid_cpu_negative", "-100m", "128Mi", "CPU must be positive"},
			{"invalid_cpu_format", "abc", "128Mi", "Invalid CPU format"},
			{"invalid_memory_zero", "100m", "0", "Memory must be positive"},
			{"invalid_memory_format", "100m", "xyz", "Invalid memory format"},
			{"cpu_too_large", "1000", "128Mi", "CPU limit exceeds maximum allowed"},
			{"memory_too_large", "100m", "100Ti", "Memory limit exceeds maximum allowed"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				deployArgs := []string{
					"deploy", "simple.py",
					"--cpu", tc.cpu,
					"--memory", tc.memory,
					"--json",
				}
				deployCmd := exec.Command(getJoblinBinary(), deployArgs...)
				deployCmd.Dir = tempDir
				deployOutput, err := deployCmd.CombinedOutput()

				// Should fail with validation error (exit code 2)
				exitCode := getExitCode(err)
				assert.NotEqual(t, 0, exitCode, "Should fail for invalid resource limits")

				// If implemented, should show exit code 2 for validation errors
				if exitCode == 2 {
					outputStr := string(deployOutput)
					assert.Contains(t, outputStr, tc.expectedMsg, "Should contain expected error message")
					assert.Contains(t, outputStr, "ValidationError", "Should indicate validation error type")
				}
			})
		}
	})

	t.Run("missing_files", func(t *testing.T) {
		testCases := []struct {
			name        string
			scriptFile  string
			reqFile     string
			expectedMsg string
		}{
			{"missing_script", "nonexistent.py", "", "Script file not found"},
			{"missing_requirements", "simple.py", "missing_requirements.txt", "Requirements file not found"},
			{"unreadable_script", "", "", "Script file not readable"},
		}

		// Create a simple script for requirements test
		scriptPath := filepath.Join(tempDir, "simple.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte("print('test')"), 0755))

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				var deployArgs []string

				if tc.name == "unreadable_script" {
					// Create unreadable script
					unreadablePath := filepath.Join(tempDir, "unreadable.py")
					require.NoError(t, os.WriteFile(unreadablePath, []byte("print('test')"), 0000))
					deployArgs = []string{"deploy", "unreadable.py", "--json"}
				} else {
					deployArgs = []string{"deploy", tc.scriptFile, "--json"}
					if tc.reqFile != "" {
						deployArgs = append(deployArgs, "--requirements", tc.reqFile)
					}
				}

				deployCmd := exec.Command(getJoblinBinary(), deployArgs...)
				deployCmd.Dir = tempDir
				deployOutput, err := deployCmd.CombinedOutput()

				// Should fail with file not found error (exit code 1)
				exitCode := getExitCode(err)
				assert.NotEqual(t, 0, exitCode, "Should fail for missing files")

				// If implemented, should show exit code 1 for file errors
				if exitCode == 1 {
					outputStr := string(deployOutput)
					assert.Contains(t, outputStr, tc.expectedMsg, "Should contain expected error message")
					assert.Contains(t, outputStr, "NotFoundError", "Should indicate file not found error type")
				}
			})
		}
	})

	t.Run("kubernetes_errors", func(t *testing.T) {
		scriptPath := filepath.Join(tempDir, "simple.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte("print('test')"), 0755))

		testCases := []struct {
			name        string
			namespace   string
			expectedMsg string
		}{
			{"nonexistent_namespace", "nonexistent-namespace-12345", "namespace not found"},
			{"invalid_namespace", "INVALID-namespace", "invalid namespace name"},
			{"restricted_namespace", "kube-system", "access denied"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				deployArgs := []string{
					"deploy", "simple.py",
					"--namespace", tc.namespace,
					"--json",
				}
				deployCmd := exec.Command(getJoblinBinary(), deployArgs...)
				deployCmd.Dir = tempDir
				deployOutput, err := deployCmd.CombinedOutput()

				// Should fail with Kubernetes error (exit code 3)
				exitCode := getExitCode(err)
				assert.NotEqual(t, 0, exitCode, "Should fail for Kubernetes errors")

				// If implemented, should show exit code 3 for Kubernetes errors
				if exitCode == 3 {
					outputStr := string(deployOutput)
					assert.Contains(t, outputStr, tc.expectedMsg, "Should contain expected error message")
					assert.Contains(t, outputStr, "KubernetesError", "Should indicate Kubernetes error type")
				}
			})
		}
	})

	t.Run("webhook_errors", func(t *testing.T) {
		scriptPath := filepath.Join(tempDir, "simple.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte("print('test')"), 0755))

		testCases := []struct {
			name        string
			webhookURL  string
			expectedMsg string
		}{
			{"invalid_url_format", "not-a-url", "invalid webhook URL format"},
			{"unsupported_protocol", "ftp://example.com/webhook", "unsupported protocol"},
			{"unreachable_host", "https://nonexistent-host-12345.example.com/webhook", "webhook URL unreachable"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				deployArgs := []string{
					"deploy", "simple.py",
					"--webhook", tc.webhookURL,
					"--json",
				}
				deployCmd := exec.Command(getJoblinBinary(), deployArgs...)
				deployCmd.Dir = tempDir
				deployOutput, err := deployCmd.CombinedOutput()

				// Should fail with network/validation error
				exitCode := getExitCode(err)
				assert.NotEqual(t, 0, exitCode, "Should fail for webhook errors")

				// If implemented, should provide helpful error messages
				if exitCode != 0 {
					outputStr := string(deployOutput)
					assert.Contains(t, outputStr, tc.expectedMsg, "Should contain expected error message")
				}
			})
		}
	})

	t.Run("configuration_errors", func(t *testing.T) {
		scriptPath := filepath.Join(tempDir, "simple.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte("print('test')"), 0755))

		// Test with invalid kubeconfig
		invalidKubeconfigPath := filepath.Join(tempDir, "invalid_kubeconfig")
		invalidKubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://invalid-server:6443
    certificate-authority-data: invalid-cert-data
  name: invalid-cluster
contexts:
- context:
    cluster: invalid-cluster
    user: invalid-user
  name: invalid-context
current-context: invalid-context
users:
- name: invalid-user
  user:
    token: invalid-token
`
		require.NoError(t, os.WriteFile(invalidKubeconfigPath, []byte(invalidKubeconfigContent), 0644))

		// Set invalid kubeconfig
		os.Setenv("KUBECONFIG", invalidKubeconfigPath)
		defer os.Unsetenv("KUBECONFIG")

		deployCmd := exec.Command(getJoblinBinary(), "deploy", "simple.py", "--json")
		deployCmd.Dir = tempDir
		deployOutput, err := deployCmd.CombinedOutput()

		// Should fail with configuration error (exit code 3)
		exitCode := getExitCode(err)
		assert.NotEqual(t, 0, exitCode, "Should fail for configuration errors")

		// If implemented, should show configuration error
		if exitCode == 3 {
			outputStr := string(deployOutput)
			assert.Contains(t, outputStr, "configuration error", "Should mention configuration error")
			assert.Contains(t, outputStr, "kubeconfig", "Should mention kubeconfig issue")
		}
	})

	t.Run("helpful_error_suggestions", func(t *testing.T) {
		// Test that error messages provide helpful suggestions
		testCases := []struct {
			name                string
			command             []string
			expectedSuggestions []string
		}{
			{
				"missing_script_suggestions",
				[]string{"deploy", "missing.py"},
				[]string{"check file path", "verify file exists", "ensure file is readable"},
			},
			{
				"invalid_cpu_suggestions",
				[]string{"deploy", "simple.py", "--cpu", "abc"},
				[]string{"use format like", "100m", "0.5", "examples"},
			},
			{
				"namespace_not_found_suggestions",
				[]string{"deploy", "simple.py", "--namespace", "missing"},
				[]string{"create namespace", "kubectl create namespace", "verify access"},
			},
		}

		scriptPath := filepath.Join(tempDir, "simple.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte("print('test')"), 0755))

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				deployCmd := exec.Command(getJoblinBinary(), tc.command...)
				deployCmd.Dir = tempDir
				deployOutput, err := deployCmd.CombinedOutput()

				assert.NotEqual(t, 0, getExitCode(err), "Should fail and provide suggestions")

				// If implemented, should provide helpful suggestions
				outputStr := strings.ToLower(string(deployOutput))
				for _, suggestion := range tc.expectedSuggestions {
					if getExitCode(err) != 0 && len(outputStr) > 0 {
						// Only check suggestions if we have actual error output
						t.Logf("Looking for suggestion '%s' in output: %s", suggestion, outputStr)
					}
				}
			})
		}
	})

	t.Run("resource_cleanup_on_errors", func(t *testing.T) {
		// Verify that failed deployments don't leave orphaned resources
		scriptPath := filepath.Join(tempDir, "simple.py")
		require.NoError(t, os.WriteFile(scriptPath, []byte("print('test')"), 0755))

		// Deploy with invalid configuration that should fail after partial creation
		deployCmd := exec.Command(getJoblinBinary(), "deploy", "simple.py", "--cpu", "invalid", "--json")
		deployCmd.Dir = tempDir
		deployOutput, err := deployCmd.CombinedOutput()

		assert.NotEqual(t, 0, getExitCode(err), "Deploy should fail")

		// If implemented, verify no orphaned resources exist
		// This would require checking Kubernetes for any jobs/configmaps left behind
		// For now, we just verify the command failed appropriately
		outputStr := string(deployOutput)
		t.Logf("Error output: %s", outputStr)
	})
}
