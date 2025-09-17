package models

import (
	"encoding/json"
	"fmt"
)

// JobStatus represents the current state of a job execution
type JobStatus string

const (
	// StatusPending indicates a job that has been created but not yet started
	StatusPending    JobStatus = "Pending"
	// StatusRunning indicates a job that is currently executing
	StatusRunning    JobStatus = "Running"
	// StatusCompleted indicates a job that has finished successfully
	StatusCompleted  JobStatus = "Completed"
	// StatusFailed indicates a job that has failed to complete successfully
	StatusFailed     JobStatus = "Failed"
	// StatusTerminated indicates a job that was manually terminated
	StatusTerminated JobStatus = "Terminated"
	// StatusUnknown indicates a job with an unknown status
	StatusUnknown    JobStatus = "Unknown"
)

var validStatuses = map[JobStatus]bool{
	StatusPending:    true,
	StatusRunning:    true,
	StatusCompleted:  true,
	StatusFailed:     true,
	StatusTerminated: true,
	StatusUnknown:    true,
}

func (s JobStatus) String() string {
	return string(s)
}

// IsValid checks if the JobStatus is one of the valid values
func (s JobStatus) IsValid() bool {
	return validStatuses[s]
}

// IsTerminal returns true if this status indicates the job has finished
func (s JobStatus) IsTerminal() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusTerminated
}

// IsActive returns true if this status indicates the job is still running
func (s JobStatus) IsActive() bool {
	return s == StatusPending || s == StatusRunning
}

// CanTransitionTo checks if a status transition is valid
func (s JobStatus) CanTransitionTo(target JobStatus) bool {
	switch s {
	case StatusPending:
		return target == StatusRunning || target == StatusFailed
	case StatusRunning:
		return target == StatusCompleted || target == StatusFailed || target == StatusTerminated
	case StatusCompleted, StatusFailed, StatusTerminated:
		return false // Terminal states cannot transition
	case StatusUnknown:
		return target != StatusUnknown // Can transition from unknown to any known state
	default:
		return false
	}
}

// Description returns a human-readable description of the status
func (s JobStatus) Description() string {
	switch s {
	case StatusPending:
		return "Job created but not yet scheduled on Kubernetes"
	case StatusRunning:
		return "Job is executing on the cluster"
	case StatusCompleted:
		return "Job finished successfully (exit code 0)"
	case StatusFailed:
		return "Job finished with error (non-zero exit code)"
	case StatusTerminated:
		return "Job was manually stopped by user"
	case StatusUnknown:
		return "Unable to determine job status (cluster unreachable)"
	default:
		return "Invalid job status"
	}
}

// UnmarshalJSON implements JSON unmarshaling for JobStatus
func (s *JobStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	status := JobStatus(str)
	if !status.IsValid() {
		return fmt.Errorf("invalid job status: %s", str)
	}

	*s = status
	return nil
}

// MarshalJSON implements JSON marshaling for JobStatus
func (s JobStatus) MarshalJSON() ([]byte, error) {
	if !s.IsValid() {
		return nil, fmt.Errorf("invalid job status: %s", s)
	}
	return json.Marshal(string(s))
}

// ParseJobStatus parses a string into a JobStatus, returning an error if invalid
func ParseJobStatus(s string) (JobStatus, error) {
	status := JobStatus(s)
	if !status.IsValid() {
		return "", fmt.Errorf("invalid job status: %s", s)
	}
	return status, nil
}

// AllJobStatuses returns all valid JobStatus values
func AllJobStatuses() []JobStatus {
	return []JobStatus{
		StatusPending,
		StatusRunning,
		StatusCompleted,
		StatusFailed,
		StatusTerminated,
		StatusUnknown,
	}
}

// ActiveStatuses returns all job statuses that indicate an active (non-terminal) job
func ActiveStatuses() []JobStatus {
	return []JobStatus{
		StatusPending,
		StatusRunning,
	}
}

// TerminalStatuses returns all job statuses that indicate a finished job
func TerminalStatuses() []JobStatus {
	return []JobStatus{
		StatusCompleted,
		StatusFailed,
		StatusTerminated,
	}
}
