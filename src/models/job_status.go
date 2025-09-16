package models

import (
	"encoding/json"
	"fmt"
)

type JobStatus string

const (
	StatusPending    JobStatus = "Pending"
	StatusRunning    JobStatus = "Running"
	StatusCompleted  JobStatus = "Completed"
	StatusFailed     JobStatus = "Failed"
	StatusTerminated JobStatus = "Terminated"
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

func (s JobStatus) IsValid() bool {
	return validStatuses[s]
}

func (s JobStatus) IsTerminal() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusTerminated
}

func (s JobStatus) IsActive() bool {
	return s == StatusPending || s == StatusRunning
}

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

func (s JobStatus) MarshalJSON() ([]byte, error) {
	if !s.IsValid() {
		return nil, fmt.Errorf("invalid job status: %s", s)
	}
	return json.Marshal(string(s))
}

func ParseJobStatus(s string) (JobStatus, error) {
	status := JobStatus(s)
	if !status.IsValid() {
		return "", fmt.Errorf("invalid job status: %s", s)
	}
	return status, nil
}

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

func ActiveStatuses() []JobStatus {
	return []JobStatus{
		StatusPending,
		StatusRunning,
	}
}

func TerminalStatuses() []JobStatus {
	return []JobStatus{
		StatusCompleted,
		StatusFailed,
		StatusTerminated,
	}
}