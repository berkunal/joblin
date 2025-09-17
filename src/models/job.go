package models

import (
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// Job represents a Python script execution job in the Kubernetes cluster
type Job struct {
	ID                string            `json:"id" bson:"_id"`
	Name              string            `json:"name"`
	ScriptPath        string            `json:"script_path"`
	ScriptContent     []byte            `json:"script_content"`
	Dependencies      []string          `json:"dependencies"`
	Status            JobStatus         `json:"status"`
	ResourceLimits    ResourceSpec      `json:"resource_limits"`
	CreatedAt         time.Time         `json:"created_at"`
	StartedAt         *time.Time        `json:"started_at,omitempty"`
	CompletedAt       *time.Time        `json:"completed_at,omitempty"`
	KubernetesJobName string            `json:"kubernetes_job_name"`
	Namespace         string            `json:"namespace"`
	ClusterContext    string            `json:"cluster_context"`
	ExitCode          *int              `json:"exit_code,omitempty"`
	TTL               time.Duration     `json:"ttl"`
	WebhookURL        string            `json:"webhook_url,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	EnvironmentVars   map[string]string `json:"environment_vars,omitempty"`
}

// NewJob creates a new Job with the provided parameters
func NewJob(name, scriptPath string, scriptContent []byte, dependencies []string) (*Job, error) {
	if err := validateJobName(name); err != nil {
		return nil, fmt.Errorf("invalid job name: %w", err)
	}

	if scriptPath == "" {
		return nil, fmt.Errorf("script path cannot be empty")
	}

	if len(scriptContent) == 0 {
		return nil, fmt.Errorf("script content cannot be empty")
	}

	job := &Job{
		ID:            uuid.New().String(),
		Name:          name,
		ScriptPath:    scriptPath,
		ScriptContent: scriptContent,
		Dependencies:  dependencies,
		Status:        StatusPending,
		ResourceLimits: ResourceSpec{
			CPU:              "100m",
			Memory:           "128Mi",
			EphemeralStorage: "1Gi",
		},
		CreatedAt:         time.Now().UTC(),
		KubernetesJobName: generateKubernetesJobName(name),
		TTL:               24 * time.Hour, // Default 24 hours
		Labels:            make(map[string]string),
	}

	return job, nil
}

// Validate checks if the Job has valid field values
func (j *Job) Validate() error {
	if j.ID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	if _, err := uuid.Parse(j.ID); err != nil {
		return fmt.Errorf("job ID must be valid UUID: %w", err)
	}

	if err := validateJobName(j.Name); err != nil {
		return fmt.Errorf("invalid job name: %w", err)
	}

	if j.ScriptPath == "" {
		return fmt.Errorf("script path cannot be empty")
	}

	if len(j.ScriptContent) == 0 {
		return fmt.Errorf("script content cannot be empty")
	}

	if err := j.ResourceLimits.Validate(); err != nil {
		return fmt.Errorf("invalid resource limits: %w", err)
	}

	if j.TTL < time.Minute {
		return fmt.Errorf("TTL must be at least 1 minute")
	}

	if j.TTL > 7*24*time.Hour {
		return fmt.Errorf("TTL cannot exceed 7 days")
	}

	return nil
}

// SetStarted marks the job as started and sets the start time
func (j *Job) SetStarted() {
	now := time.Now().UTC()
	j.StartedAt = &now
	j.Status = StatusRunning
}

// SetCompleted marks the job as completed with the given exit code
func (j *Job) SetCompleted(exitCode int) {
	now := time.Now().UTC()
	j.CompletedAt = &now
	j.ExitCode = &exitCode

	if exitCode == 0 {
		j.Status = StatusCompleted
	} else {
		j.Status = StatusFailed
	}
}

// SetTerminated marks the job as terminated
func (j *Job) SetTerminated() {
	now := time.Now().UTC()
	j.CompletedAt = &now
	j.Status = StatusTerminated
}

// SetFailed marks the job as failed with the given exit code
func (j *Job) SetFailed(exitCode int) {
	now := time.Now().UTC()
	j.CompletedAt = &now
	j.ExitCode = &exitCode
	j.Status = StatusFailed
}

// IsFinished returns true if the job has completed, failed, or been terminated
func (j *Job) IsFinished() bool {
	return j.Status == StatusCompleted || j.Status == StatusFailed || j.Status == StatusTerminated
}

// Duration returns the time elapsed since the job started, or total duration if finished
func (j *Job) Duration() time.Duration {
	if j.StartedAt == nil {
		return 0
	}

	if j.CompletedAt != nil {
		return j.CompletedAt.Sub(*j.StartedAt)
	}

	return time.Since(*j.StartedAt)
}

// IsExpired returns true if the job has exceeded its TTL and should be cleaned up
func (j *Job) IsExpired() bool {
	return time.Since(j.CreatedAt) > j.TTL
}

func validateJobName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if len(name) > 63 {
		return fmt.Errorf("name cannot exceed 63 characters")
	}

	// DNS-1123 compliance: lowercase alphanumeric and hyphens, start/end with alphanumeric
	dnsPattern := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !dnsPattern.MatchString(name) {
		return fmt.Errorf("name must be DNS-1123 compliant (lowercase alphanumeric and hyphens, start/end with alphanumeric)")
	}

	return nil
}

func generateKubernetesJobName(jobName string) string {
	// Generate a unique Kubernetes job name by appending timestamp
	timestamp := time.Now().Unix()
	return fmt.Sprintf("joblin-%s-%d", jobName, timestamp)
}
