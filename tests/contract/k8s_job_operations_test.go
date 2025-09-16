package contract

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

// TestKubernetesJobStatusMonitoring tests the contract for monitoring Job status
// This test MUST FAIL until the Kubernetes monitoring is implemented
func TestKubernetesJobStatusMonitoring(t *testing.T) {
	tests := []struct {
		name           string
		jobID          string
		kubernetesJob  *batchv1.Job
		kubernetesPods []*corev1.Pod
		expectedStatus string
		expectedError  string
	}{
		{
			name:  "job_pending",
			jobID: "12345678-1234-4234-8234-123456789012",
			kubernetesJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "joblin-12345678",
					Namespace: "default",
					Labels: map[string]string{
						"joblin.io/job-id": "12345678-1234-4234-8234-123456789012",
					},
				},
				Status: batchv1.JobStatus{
					Active:    0,
					Succeeded: 0,
					Failed:    0,
				},
			},
			expectedStatus: "pending",
		},
		{
			name:  "job_running",
			jobID: "12345678-1234-4234-8234-123456789013",
			kubernetesJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "joblin-12345678",
					Namespace: "default",
					Labels: map[string]string{
						"joblin.io/job-id": "12345678-1234-4234-8234-123456789013",
					},
				},
				Status: batchv1.JobStatus{
					Active:    1,
					Succeeded: 0,
					Failed:    0,
				},
			},
			kubernetesPods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "joblin-12345678-pod1",
						Namespace: "default",
						Labels: map[string]string{
							"job-name": "joblin-12345678",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			expectedStatus: "running",
		},
		{
			name:  "job_completed",
			jobID: "12345678-1234-4234-8234-123456789014",
			kubernetesJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "joblin-12345678",
					Namespace: "default",
					Labels: map[string]string{
						"joblin.io/job-id": "12345678-1234-4234-8234-123456789014",
					},
				},
				Status: batchv1.JobStatus{
					Active:    0,
					Succeeded: 1,
					Failed:    0,
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			kubernetesPods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "joblin-12345678-pod1",
						Namespace: "default",
						Labels: map[string]string{
							"job-name": "joblin-12345678",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode: 0,
									},
								},
							},
						},
					},
				},
			},
			expectedStatus: "completed",
		},
		{
			name:  "job_failed",
			jobID: "12345678-1234-4234-8234-123456789015",
			kubernetesJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "joblin-12345678",
					Namespace: "default",
					Labels: map[string]string{
						"joblin.io/job-id": "12345678-1234-4234-8234-123456789015",
					},
				},
				Status: batchv1.JobStatus{
					Active:    0,
					Succeeded: 0,
					Failed:    1,
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobFailed,
							Status: corev1.ConditionTrue,
							Reason: "BackoffLimitExceeded",
						},
					},
				},
			},
			kubernetesPods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "joblin-12345678-pod1",
						Namespace: "default",
						Labels: map[string]string{
							"job-name": "joblin-12345678",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodFailed,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode: 1,
										Reason:   "Error",
									},
								},
							},
						},
					},
				},
			},
			expectedStatus: "failed",
		},
		{
			name:          "job_not_found",
			jobID:         "nonexistent-job-id",
			kubernetesJob: nil,
			expectedError: "job not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake Kubernetes client
			client := fake.NewSimpleClientset()

			// Add the Job to the fake client if it exists
			if tt.kubernetesJob != nil {
				_, err := client.BatchV1().Jobs("default").Create(context.Background(), tt.kubernetesJob, metav1.CreateOptions{})
				require.NoError(t, err, "Failed to create test Job")
			}

			// Add Pods to the fake client
			for _, pod := range tt.kubernetesPods {
				_, err := client.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})
				require.NoError(t, err, "Failed to create test Pod")
			}

			// Create the Kubernetes service (this will fail until implemented)
			k8sService := NewK8sServiceOperations(client)

			// Attempt to get job status - this MUST fail until implementation
			ctx := context.Background()
			status, err := k8sService.GetJobStatus(ctx, tt.jobID, "default")

			if tt.expectedError != "" {
				assert.Error(t, err, "Expected error but got none")
				assert.Contains(t, err.Error(), tt.expectedError, "Error message mismatch")
				return
			}

			require.NoError(t, err, "GetJobStatus should succeed")
			assert.Equal(t, tt.expectedStatus, status.Status, "Job status mismatch")

			// Validate additional status fields based on expected status
			switch tt.expectedStatus {
			case "completed":
				assert.NotNil(t, status.CompletedAt, "CompletedAt should be set for completed jobs")
				assert.Equal(t, 0, status.ExitCode, "Exit code should be 0 for successful jobs")
			case "failed":
				assert.NotNil(t, status.CompletedAt, "CompletedAt should be set for failed jobs")
				assert.NotEqual(t, 0, status.ExitCode, "Exit code should be non-zero for failed jobs")
			case "running":
				assert.NotNil(t, status.StartedAt, "StartedAt should be set for running jobs")
				assert.Nil(t, status.CompletedAt, "CompletedAt should not be set for running jobs")
			}
		})
	}
}

// TestKubernetesJobLogRetrieval tests the contract for retrieving job logs
// This test MUST FAIL until the Kubernetes log retrieval is implemented
func TestKubernetesJobLogRetrieval(t *testing.T) {
	tests := []struct {
		name          string
		jobID         string
		pods          []*corev1.Pod
		podLogs       map[string]string // pod name -> log content
		logOptions    LogOptions
		expectedLogs  []LogEntry
		expectedError string
	}{
		{
			name:  "basic_log_retrieval",
			jobID: "12345678-1234-4234-8234-123456789012",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "joblin-12345678-pod1",
						Namespace: "default",
						Labels: map[string]string{
							"job-name": "joblin-12345678",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			podLogs: map[string]string{
				"joblin-12345678-pod1": "2025-09-15T10:30:00Z Processing data...\n2025-09-15T10:30:01Z Task completed successfully",
			},
			logOptions: LogOptions{},
			expectedLogs: []LogEntry{
				{
					Timestamp: time.Date(2025, 9, 15, 10, 30, 0, 0, time.UTC),
					Source:    "stdout",
					Content:   "Processing data...",
					PodName:   "joblin-12345678-pod1",
				},
				{
					Timestamp: time.Date(2025, 9, 15, 10, 30, 1, 0, time.UTC),
					Source:    "stdout",
					Content:   "Task completed successfully",
					PodName:   "joblin-12345678-pod1",
				},
			},
		},
		{
			name:  "log_retrieval_with_tail",
			jobID: "12345678-1234-4234-8234-123456789013",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "joblin-12345678-pod1",
						Namespace: "default",
						Labels: map[string]string{
							"job-name": "joblin-12345678",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			podLogs: map[string]string{
				"joblin-12345678-pod1": "Line 1\nLine 2\nLine 3\nLine 4\nLine 5",
			},
			logOptions: LogOptions{
				TailLines: 2,
			},
			expectedLogs: []LogEntry{
				{
					Source:  "stdout",
					Content: "Line 4",
					PodName: "joblin-12345678-pod1",
				},
				{
					Source:  "stdout",
					Content: "Line 5",
					PodName: "joblin-12345678-pod1",
				},
			},
		},
		{
			name:  "log_retrieval_with_since",
			jobID: "12345678-1234-4234-8234-123456789014",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "joblin-12345678-pod1",
						Namespace: "default",
						Labels: map[string]string{
							"job-name": "joblin-12345678",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			podLogs: map[string]string{
				"joblin-12345678-pod1": "2025-09-15T10:29:59Z Old log\n2025-09-15T10:30:01Z New log",
			},
			logOptions: LogOptions{
				SinceTime: time.Date(2025, 9, 15, 10, 30, 0, 0, time.UTC),
			},
			expectedLogs: []LogEntry{
				{
					Timestamp: time.Date(2025, 9, 15, 10, 30, 1, 0, time.UTC),
					Source:    "stdout",
					Content:   "New log",
					PodName:   "joblin-12345678-pod1",
				},
			},
		},
		{
			name:  "multiple_pods_logs",
			jobID: "12345678-1234-4234-8234-123456789015",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "joblin-12345678-pod1",
						Namespace: "default",
						Labels: map[string]string{
							"job-name": "joblin-12345678",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "joblin-12345678-pod2",
						Namespace: "default",
						Labels: map[string]string{
							"job-name": "joblin-12345678",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			podLogs: map[string]string{
				"joblin-12345678-pod1": "Pod 1 log",
				"joblin-12345678-pod2": "Pod 2 log",
			},
			logOptions: LogOptions{},
			expectedLogs: []LogEntry{
				{
					Source:  "stdout",
					Content: "Pod 1 log",
					PodName: "joblin-12345678-pod1",
				},
				{
					Source:  "stdout",
					Content: "Pod 2 log",
					PodName: "joblin-12345678-pod2",
				},
			},
		},
		{
			name:          "no_pods_found",
			jobID:         "12345678-1234-4234-8234-123456789016",
			pods:          []*corev1.Pod{},
			expectedError: "no pods found for job",
		},
		{
			name:  "no_logs_available",
			jobID: "12345678-1234-4234-8234-123456789017",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "joblin-12345678-pod1",
						Namespace: "default",
						Labels: map[string]string{
							"job-name": "joblin-12345678",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
					},
				},
			},
			podLogs:       map[string]string{},
			expectedError: "no logs available yet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake Kubernetes client
			client := fake.NewSimpleClientset()

			// Add Pods to the fake client
			for _, pod := range tt.pods {
				_, err := client.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})
				require.NoError(t, err, "Failed to create test Pod")
			}

			// Mock log retrieval
			client.PrependReactor("get", "pods", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				getAction := action.(ktesting.GetAction)
				podName := getAction.GetName()

				// Return logs for the pod if available
				if _, exists := tt.podLogs[podName]; exists {
					// This is a simplified mock - in reality, we'd need to mock the logs subresource
					return false, nil, nil // Let the fake client handle the get
				}

				return false, nil, nil
			})

			// Create the Kubernetes service (this will fail until implemented)
			k8sService := NewK8sServiceOperations(client)

			// Attempt to get job logs - this MUST fail until implementation
			ctx := context.Background()
			logs, err := k8sService.GetJobLogs(ctx, tt.jobID, "default", tt.logOptions)

			if tt.expectedError != "" {
				assert.Error(t, err, "Expected error but got none")
				assert.Contains(t, err.Error(), tt.expectedError, "Error message mismatch")
				return
			}

			require.NoError(t, err, "GetJobLogs should succeed")

			// Validate log entries (ignoring timestamps for some tests)
			assert.Len(t, logs, len(tt.expectedLogs), "Number of log entries mismatch")

			for i, expectedLog := range tt.expectedLogs {
				if i < len(logs) {
					actualLog := logs[i]
					assert.Equal(t, expectedLog.Source, actualLog.Source, "Log source mismatch at index %d", i)
					assert.Equal(t, expectedLog.Content, actualLog.Content, "Log content mismatch at index %d", i)
					assert.Equal(t, expectedLog.PodName, actualLog.PodName, "Pod name mismatch at index %d", i)

					// Only check timestamp if it's set in expected
					if !expectedLog.Timestamp.IsZero() {
						assert.Equal(t, expectedLog.Timestamp, actualLog.Timestamp, "Timestamp mismatch at index %d", i)
					}
				}
			}
		})
	}
}

// TestKubernetesJobTermination tests the contract for terminating jobs
// This test MUST FAIL until the Kubernetes job termination is implemented
func TestKubernetesJobTermination(t *testing.T) {
	tests := []struct {
		name          string
		jobID         string
		kubernetesJob *batchv1.Job
		force         bool
		expectedError string
	}{
		{
			name:  "terminate_running_job",
			jobID: "12345678-1234-4234-8234-123456789012",
			kubernetesJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "joblin-12345678",
					Namespace: "default",
					Labels: map[string]string{
						"joblin.io/job-id": "12345678-1234-4234-8234-123456789012",
					},
				},
				Status: batchv1.JobStatus{
					Active: 1,
				},
			},
			force: false,
		},
		{
			name:  "force_terminate_job",
			jobID: "12345678-1234-4234-8234-123456789013",
			kubernetesJob: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "joblin-12345678",
					Namespace: "default",
					Labels: map[string]string{
						"joblin.io/job-id": "12345678-1234-4234-8234-123456789013",
					},
				},
				Status: batchv1.JobStatus{
					Active: 1,
				},
			},
			force: true,
		},
		{
			name:          "terminate_nonexistent_job",
			jobID:         "nonexistent-job-id",
			kubernetesJob: nil,
			expectedError: "job not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake Kubernetes client
			client := fake.NewSimpleClientset()

			// Add the Job to the fake client if it exists
			if tt.kubernetesJob != nil {
				_, err := client.BatchV1().Jobs("default").Create(context.Background(), tt.kubernetesJob, metav1.CreateOptions{})
				require.NoError(t, err, "Failed to create test Job")
			}

			// Track deletion actions
			var deletedJob string
			client.PrependReactor("delete", "jobs", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				deleteAction := action.(ktesting.DeleteAction)
				deletedJob = deleteAction.GetName()
				return false, nil, nil
			})

			// Create the Kubernetes service (this will fail until implemented)
			k8sService := NewK8sServiceOperations(client)

			// Attempt to terminate job - this MUST fail until implementation
			ctx := context.Background()
			err := k8sService.TerminateJob(ctx, tt.jobID, "default", tt.force)

			if tt.expectedError != "" {
				assert.Error(t, err, "Expected error but got none")
				assert.Contains(t, err.Error(), tt.expectedError, "Error message mismatch")
				return
			}

			require.NoError(t, err, "TerminateJob should succeed")

			// Verify the job was deleted
			if tt.kubernetesJob != nil {
				assert.Equal(t, tt.kubernetesJob.Name, deletedJob, "Wrong job deleted")
			}
		})
	}
}

// Helper types and interfaces

// JobStatus represents the status of a Kubernetes job
type JobStatus struct {
	Status      string
	StartedAt   *time.Time
	CompletedAt *time.Time
	ExitCode    int
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time
	Source    string // stdout, stderr, system
	Content   string
	PodName   string
}

// LogOptions represents options for log retrieval
type LogOptions struct {
	TailLines  int
	SinceTime  time.Time
	Follow     bool
	Timestamps bool
}

// Extended K8sService interface for operations
type K8sServiceOperations interface {
	K8sService
	GetJobStatus(ctx context.Context, jobID, namespace string) (*JobStatus, error)
	GetJobLogs(ctx context.Context, jobID, namespace string, options LogOptions) ([]LogEntry, error)
	TerminateJob(ctx context.Context, jobID, namespace string, force bool) error
}

// Override the NewK8sService to return extended interface
func NewK8sServiceOperations(client kubernetes.Interface) K8sServiceOperations {
	// This should return the actual implementation
	// For now, this will cause a compile error, which is expected
	panic("K8sServiceOperations not implemented - this test should fail until T024 is complete")
}

// Mock log reader for testing
type mockLogReader struct {
	content string
}

func (m *mockLogReader) Read(p []byte) (n int, err error) {
	if len(m.content) == 0 {
		return 0, io.EOF
	}

	n = copy(p, m.content)
	m.content = m.content[n:]

	if len(m.content) == 0 {
		err = io.EOF
	}

	return n, err
}

func (m *mockLogReader) Close() error {
	return nil
}

// parseLogLine parses a log line with timestamp
func parseLogLine(line string) (time.Time, string, error) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		return time.Time{}, line, nil
	}

	timestamp, err := time.Parse(time.RFC3339, parts[0])
	if err != nil {
		return time.Time{}, line, nil
	}

	return timestamp, parts[1], nil
}
