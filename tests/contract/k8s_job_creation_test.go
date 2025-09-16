package contract

import (
	"context"
	"regexp"
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

// TestKubernetesJobCreation tests the contract for creating Kubernetes Jobs and ConfigMaps
// This test MUST FAIL until the Kubernetes integration is implemented
func TestKubernetesJobCreation(t *testing.T) {
	tests := []struct {
		name               string
		jobSpec            JobSpec
		expectJobFields    map[string]interface{}
		expectConfigFields map[string]interface{}
		expectError        string
	}{
		{
			name: "basic_job_creation",
			jobSpec: JobSpec{
				ID:            "12345678-1234-4234-8234-123456789012",
				Name:          "test-job",
				ScriptContent: "print('Hello, World!')",
				Dependencies:  []string{},
				Namespace:     "default",
				ResourceLimits: ResourceLimits{
					CPU:    "100m",
					Memory: "128Mi",
				},
				TTL: 24 * time.Hour,
			},
			expectJobFields: map[string]interface{}{
				"metadata.name":                          "joblin-12345678",
				"metadata.namespace":                     "default",
				"metadata.labels.app.kubernetes.io/name": "joblin",
				"metadata.labels.joblin.io/job-id":       "12345678-1234-4234-8234-123456789012",
				"spec.ttlSecondsAfterFinished":           int32(86400),
				"spec.backoffLimit":                      int32(3),
				"spec.activeDeadlineSeconds":             int32(3600),
			},
			expectConfigFields: map[string]interface{}{
				"metadata.name":      "joblin-script-12345678",
				"metadata.namespace": "default",
				"data.script.py":     "print('Hello, World!')",
			},
		},
		{
			name: "job_with_dependencies",
			jobSpec: JobSpec{
				ID:            "12345678-1234-4234-8234-123456789013",
				Name:          "test-deps",
				ScriptContent: "import requests\nprint('Dependencies work!')",
				Dependencies:  []string{"requests==2.31.0", "pandas==2.1.0"},
				Namespace:     "test-namespace",
				ResourceLimits: ResourceLimits{
					CPU:    "200m",
					Memory: "256Mi",
				},
				TTL: 12 * time.Hour,
			},
			expectJobFields: map[string]interface{}{
				"metadata.name":      "joblin-12345678",
				"metadata.namespace": "test-namespace",
				"spec.template.spec.containers[0].resources.limits.cpu":    "200m",
				"spec.template.spec.containers[0].resources.limits.memory": "256Mi",
			},
			expectConfigFields: map[string]interface{}{
				"data.requirements.txt": "requests==2.31.0\npandas==2.1.0",
			},
		},
		{
			name: "job_with_environment_variables",
			jobSpec: JobSpec{
				ID:            "12345678-1234-4234-8234-123456789014",
				Name:          "test-env",
				ScriptContent: "import os\nprint(f'ENV_VAR: {os.environ.get(\"ENV_VAR\")}')",
				EnvVars: map[string]string{
					"ENV_VAR": "test_value",
					"DEBUG":   "true",
				},
				Namespace: "default",
				ResourceLimits: ResourceLimits{
					CPU:    "100m",
					Memory: "128Mi",
				},
			},
			expectJobFields: map[string]interface{}{
				"spec.template.spec.containers[0].env": []corev1.EnvVar{
					{Name: "ENV_VAR", Value: "test_value"},
					{Name: "DEBUG", Value: "true"},
				},
			},
		},
		{
			name: "job_with_labels",
			jobSpec: JobSpec{
				ID:            "12345678-1234-4234-8234-123456789015",
				Name:          "test-labels",
				ScriptContent: "print('Labeled job')",
				Labels: map[string]string{
					"team":    "data-science",
					"project": "analysis",
				},
				Namespace: "default",
				ResourceLimits: ResourceLimits{
					CPU:    "100m",
					Memory: "128Mi",
				},
			},
			expectJobFields: map[string]interface{}{
				"metadata.labels.user.joblin.io/team":    "data-science",
				"metadata.labels.user.joblin.io/project": "analysis",
			},
		},
		{
			name: "job_with_webhook",
			jobSpec: JobSpec{
				ID:            "12345678-1234-4234-8234-123456789016",
				Name:          "test-webhook",
				ScriptContent: "print('Job with webhook')",
				WebhookURL:    "https://teams.microsoft.com/webhook/test",
				Namespace:     "default",
				ResourceLimits: ResourceLimits{
					CPU:    "100m",
					Memory: "128Mi",
				},
			},
			expectJobFields: map[string]interface{}{
				"metadata.annotations.joblin.io/webhook-url": "https://teams.microsoft.com/webhook/test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake Kubernetes client
			client := fake.NewSimpleClientset()

			// Track actions performed on the fake client
			var createdJob *batchv1.Job
			var createdConfigMap *corev1.ConfigMap
			client.PrependReactor("create", "jobs", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				createAction := action.(ktesting.CreateAction)
				job := createAction.GetObject().(*batchv1.Job)
				createdJob = job
				return false, nil, nil // Let the fake client handle the creation
			})
			client.PrependReactor("create", "configmaps", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				createAction := action.(ktesting.CreateAction)
				configMap := createAction.GetObject().(*corev1.ConfigMap)
				createdConfigMap = configMap
				return false, nil, nil // Let the fake client handle the creation
			})

			// Create the Kubernetes service (this will fail until implemented)
			k8sService := NewK8sService(client)

			// Attempt to create job - this MUST fail until implementation
			ctx := context.Background()
			err := k8sService.CreateJob(ctx, tt.jobSpec)

			if tt.expectError != "" {
				assert.Error(t, err, "Expected error but got none")
				assert.Contains(t, err.Error(), tt.expectError, "Error message mismatch")
				return
			}

			// If no error expected, verify the Job and ConfigMap were created correctly
			require.NoError(t, err, "Job creation should succeed")
			require.NotNil(t, createdJob, "Job should be created")
			require.NotNil(t, createdConfigMap, "ConfigMap should be created")

			// Validate Job fields
			validateJobFields(t, createdJob, tt.expectJobFields)

			// Validate ConfigMap fields
			validateConfigMapFields(t, createdConfigMap, tt.expectConfigFields)

			// Validate OwnerReference
			assert.Len(t, createdConfigMap.OwnerReferences, 1, "ConfigMap should have one owner reference")
			ownerRef := createdConfigMap.OwnerReferences[0]
			assert.Equal(t, "batch/v1", ownerRef.APIVersion, "Owner reference API version")
			assert.Equal(t, "Job", ownerRef.Kind, "Owner reference kind")
			assert.Equal(t, createdJob.Name, ownerRef.Name, "Owner reference name")
			assert.True(t, *ownerRef.Controller, "Owner reference should be controller")
		})
	}
}

// TestKubernetesJobNaming tests the naming convention for Jobs and ConfigMaps
// This test MUST FAIL until the Kubernetes integration is implemented
func TestKubernetesJobNaming(t *testing.T) {
	tests := []struct {
		name               string
		jobID              string
		expectedJobName    string
		expectedConfigName string
	}{
		{
			name:               "standard_uuid",
			jobID:              "12345678-1234-4234-8234-123456789012",
			expectedJobName:    "joblin-12345678",
			expectedConfigName: "joblin-script-12345678",
		},
		{
			name:               "another_uuid",
			jobID:              "abcdef01-2345-6789-abcd-ef0123456789",
			expectedJobName:    "joblin-abcdef01",
			expectedConfigName: "joblin-script-abcdef01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()

			var createdJob *batchv1.Job
			var createdConfigMap *corev1.ConfigMap
			client.PrependReactor("create", "jobs", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				createAction := action.(ktesting.CreateAction)
				job := createAction.GetObject().(*batchv1.Job)
				createdJob = job
				return false, nil, nil
			})
			client.PrependReactor("create", "configmaps", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				createAction := action.(ktesting.CreateAction)
				configMap := createAction.GetObject().(*corev1.ConfigMap)
				createdConfigMap = configMap
				return false, nil, nil
			})

			k8sService := NewK8sService(client)
			jobSpec := JobSpec{
				ID:             tt.jobID,
				Name:           "test-job",
				ScriptContent:  "print('test')",
				Namespace:      "default",
				ResourceLimits: ResourceLimits{CPU: "100m", Memory: "128Mi"},
			}

			ctx := context.Background()
			err := k8sService.CreateJob(ctx, jobSpec)
			require.NoError(t, err, "Job creation should succeed")

			assert.Equal(t, tt.expectedJobName, createdJob.Name, "Job name mismatch")
			assert.Equal(t, tt.expectedConfigName, createdConfigMap.Name, "ConfigMap name mismatch")

			// Validate names are DNS-1123 compliant
			dnsRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
			assert.True(t, dnsRegex.MatchString(createdJob.Name), "Job name should be DNS-1123 compliant")
			assert.True(t, dnsRegex.MatchString(createdConfigMap.Name), "ConfigMap name should be DNS-1123 compliant")
		})
	}
}

// TestKubernetesJobValidation tests input validation for Job creation
// This test MUST FAIL until the Kubernetes integration is implemented
func TestKubernetesJobValidation(t *testing.T) {
	tests := []struct {
		name        string
		jobSpec     JobSpec
		expectError string
	}{
		{
			name: "invalid_cpu_format",
			jobSpec: JobSpec{
				ID:            "12345678-1234-4234-8234-123456789012",
				Name:          "test-job",
				ScriptContent: "print('test')",
				ResourceLimits: ResourceLimits{
					CPU:    "invalid",
					Memory: "128Mi",
				},
			},
			expectError: "invalid CPU quantity",
		},
		{
			name: "invalid_memory_format",
			jobSpec: JobSpec{
				ID:            "12345678-1234-4234-8234-123456789012",
				Name:          "test-job",
				ScriptContent: "print('test')",
				ResourceLimits: ResourceLimits{
					CPU:    "100m",
					Memory: "999ZZ",
				},
			},
			expectError: "invalid memory quantity",
		},
		{
			name: "empty_script_content",
			jobSpec: JobSpec{
				ID:            "12345678-1234-4234-8234-123456789012",
				Name:          "test-job",
				ScriptContent: "",
				ResourceLimits: ResourceLimits{
					CPU:    "100m",
					Memory: "128Mi",
				},
			},
			expectError: "script content cannot be empty",
		},
		{
			name: "invalid_job_name",
			jobSpec: JobSpec{
				ID:            "12345678-1234-4234-8234-123456789012",
				Name:          "INVALID_NAME_WITH_UPPERCASE",
				ScriptContent: "print('test')",
				ResourceLimits: ResourceLimits{
					CPU:    "100m",
					Memory: "128Mi",
				},
			},
			expectError: "invalid job name format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			k8sService := NewK8sService(client)

			ctx := context.Background()
			err := k8sService.CreateJob(ctx, tt.jobSpec)

			assert.Error(t, err, "Should return error for invalid input")
			assert.Contains(t, err.Error(), tt.expectError, "Error message should contain expected text")
		})
	}
}

// Helper functions and types

// JobSpec represents the specification for creating a Kubernetes Job
type JobSpec struct {
	ID             string
	Name           string
	ScriptContent  string
	Dependencies   []string
	EnvVars        map[string]string
	Labels         map[string]string
	Namespace      string
	ResourceLimits ResourceLimits
	TTL            time.Duration
	WebhookURL     string
}

// ResourceLimits represents resource constraints
type ResourceLimits struct {
	CPU    string
	Memory string
}

// K8sService interface - this will be implemented in the actual service
type K8sService interface {
	CreateJob(ctx context.Context, spec JobSpec) error
}

// NewK8sService creates a new Kubernetes service - this will fail until implemented
func NewK8sService(client kubernetes.Interface) K8sService {
	// This should return the actual implementation
	// For now, this will cause a compile error, which is expected
	panic("K8sService not implemented - this test should fail until T024 is complete")
}

// validateJobFields validates that the created Job has the expected fields
func validateJobFields(t *testing.T, job *batchv1.Job, expectedFields map[string]interface{}) {
	for fieldPath, expectedValue := range expectedFields {
		actualValue := getFieldValue(t, job, fieldPath)

		switch expected := expectedValue.(type) {
		case string:
			assert.Equal(t, expected, actualValue, "Field %s value mismatch", fieldPath)
		case int32:
			assert.Equal(t, expected, actualValue, "Field %s value mismatch", fieldPath)
		case []corev1.EnvVar:
			actualEnvVars, ok := actualValue.([]corev1.EnvVar)
			assert.True(t, ok, "Field %s should be []corev1.EnvVar", fieldPath)
			assert.ElementsMatch(t, expected, actualEnvVars, "Environment variables mismatch")
		default:
			assert.Equal(t, expected, actualValue, "Field %s value mismatch", fieldPath)
		}
	}
}

// validateConfigMapFields validates that the created ConfigMap has the expected fields
func validateConfigMapFields(t *testing.T, configMap *corev1.ConfigMap, expectedFields map[string]interface{}) {
	for fieldPath, expectedValue := range expectedFields {
		actualValue := getConfigMapFieldValue(t, configMap, fieldPath)
		assert.Equal(t, expectedValue, actualValue, "ConfigMap field %s value mismatch", fieldPath)
	}
}

// getFieldValue extracts a field value from a Job object using dot notation
func getFieldValue(t *testing.T, job *batchv1.Job, fieldPath string) interface{} {
	parts := strings.Split(fieldPath, ".")
	var current interface{} = job

	for _, part := range parts {
		switch obj := current.(type) {
		case *batchv1.Job:
			switch part {
			case "metadata":
				current = &obj.ObjectMeta
			case "spec":
				current = &obj.Spec
			default:
				t.Fatalf("Unknown Job field: %s", part)
			}
		case *metav1.ObjectMeta:
			switch part {
			case "name":
				return obj.Name
			case "namespace":
				return obj.Namespace
			case "labels":
				current = obj.Labels
			case "annotations":
				current = obj.Annotations
			default:
				t.Fatalf("Unknown ObjectMeta field: %s", part)
			}
		case *batchv1.JobSpec:
			switch part {
			case "ttlSecondsAfterFinished":
				if obj.TTLSecondsAfterFinished != nil {
					return *obj.TTLSecondsAfterFinished
				}
				return nil
			case "backoffLimit":
				if obj.BackoffLimit != nil {
					return *obj.BackoffLimit
				}
				return nil
			case "activeDeadlineSeconds":
				if obj.ActiveDeadlineSeconds != nil {
					return *obj.ActiveDeadlineSeconds
				}
				return nil
			case "template":
				current = &obj.Template
			default:
				t.Fatalf("Unknown JobSpec field: %s", part)
			}
		case map[string]string:
			if strings.HasPrefix(part, "app.kubernetes.io/") || strings.HasPrefix(part, "joblin.io/") || strings.HasPrefix(part, "user.joblin.io/") {
				return obj[part]
			}
			t.Fatalf("Unknown map key: %s", part)
		default:
			// Handle more complex field paths as needed
			t.Fatalf("Unhandled field path: %s (current type: %T)", fieldPath, current)
		}
	}

	return current
}

// getConfigMapFieldValue extracts a field value from a ConfigMap object
func getConfigMapFieldValue(t *testing.T, configMap *corev1.ConfigMap, fieldPath string) interface{} {
	parts := strings.Split(fieldPath, ".")

	switch parts[0] {
	case "metadata":
		switch parts[1] {
		case "name":
			return configMap.Name
		case "namespace":
			return configMap.Namespace
		}
	case "data":
		if len(parts) >= 2 {
			return configMap.Data[parts[1]]
		}
	}

	t.Fatalf("Unknown ConfigMap field path: %s", fieldPath)
	return nil
}
