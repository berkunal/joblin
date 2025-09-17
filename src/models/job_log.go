package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// LogSource represents the source of a log entry (stdout, stderr, or system)
type LogSource string

const (
	// LogSourceStdout represents standard output logs
	LogSourceStdout LogSource = "Stdout"
	// LogSourceStderr represents standard error logs
	LogSourceStderr LogSource = "Stderr"
	// LogSourceSystem represents system-generated logs
	LogSourceSystem LogSource = "System"
)

var validLogSources = map[LogSource]bool{
	LogSourceStdout: true,
	LogSourceStderr: true,
	LogSourceSystem: true,
}

func (ls LogSource) String() string {
	return string(ls)
}

// IsValid checks if the LogSource is one of the valid values
func (ls LogSource) IsValid() bool {
	return validLogSources[ls]
}

// UnmarshalJSON implements JSON unmarshaling for LogSource
func (ls *LogSource) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	source := LogSource(str)
	if !source.IsValid() {
		return fmt.Errorf("invalid log source: %s", str)
	}

	*ls = source
	return nil
}

// MarshalJSON implements JSON marshaling for LogSource
func (ls LogSource) MarshalJSON() ([]byte, error) {
	if !ls.IsValid() {
		return nil, fmt.Errorf("invalid log source: %s", ls)
	}
	return json.Marshal(string(ls))
}

// JobLog represents a log entry from a job execution
type JobLog struct {
	JobID     string    `json:"job_id"`
	Timestamp time.Time `json:"timestamp"`
	Source    LogSource `json:"source"`
	Content   string    `json:"content"`
	PodName   string    `json:"pod_name"`
}

// NewJobLog creates a new job log entry with the specified parameters
func NewJobLog(jobID string, source LogSource, content string, podName string) (*JobLog, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job ID cannot be empty")
	}

	if !source.IsValid() {
		return nil, fmt.Errorf("invalid log source: %s", source)
	}

	if len(content) > 10*1024 { // 10KB limit
		return nil, fmt.Errorf("log content exceeds 10KB limit")
	}

	return &JobLog{
		JobID:     jobID,
		Timestamp: time.Now().UTC(),
		Source:    source,
		Content:   content,
		PodName:   podName,
	}, nil
}

// Validate checks if the JobLog has valid field values
func (jl *JobLog) Validate() error {
	if jl.JobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	if !jl.Source.IsValid() {
		return fmt.Errorf("invalid log source: %s", jl.Source)
	}

	if len(jl.Content) > 10*1024 { // 10KB limit
		return fmt.Errorf("log content exceeds 10KB limit")
	}

	if jl.Timestamp.IsZero() {
		return fmt.Errorf("timestamp cannot be zero")
	}

	return nil
}

// GetStorageKey returns a unique key for storing this log entry
func (jl *JobLog) GetStorageKey() string {
	return fmt.Sprintf("%s:%d", jl.JobID, jl.Timestamp.UnixNano())
}

// IsSystemLog returns true if this is a system-generated log
func (jl *JobLog) IsSystemLog() bool {
	return jl.Source == LogSourceSystem
}

// IsApplicationLog returns true if this is an application log (stdout/stderr)
func (jl *JobLog) IsApplicationLog() bool {
	return jl.Source == LogSourceStdout || jl.Source == LogSourceStderr
}

// IsErrorLog returns true if this is an error log (stderr)
func (jl *JobLog) IsErrorLog() bool {
	return jl.Source == LogSourceStderr
}

// Format returns a formatted string representation of the log entry
func (jl *JobLog) Format() string {
	timeStr := jl.Timestamp.Format("2006-01-02 15:04:05")
	sourceStr := string(jl.Source)

	if jl.PodName != "" {
		return fmt.Sprintf("[%s] [%s] [%s] %s", timeStr, sourceStr, jl.PodName, jl.Content)
	}

	return fmt.Sprintf("[%s] [%s] %s", timeStr, sourceStr, jl.Content)
}

// Clone creates a deep copy of the JobLog
func (jl *JobLog) Clone() *JobLog {
	return &JobLog{
		JobID:     jl.JobID,
		Timestamp: jl.Timestamp,
		Source:    jl.Source,
		Content:   jl.Content,
		PodName:   jl.PodName,
	}
}

// JobLogCollection represents a collection of job logs with filtering and sorting capabilities
type JobLogCollection []*JobLog

func (jlc JobLogCollection) Len() int {
	return len(jlc)
}

func (jlc JobLogCollection) Less(i, j int) bool {
	return jlc[i].Timestamp.Before(jlc[j].Timestamp)
}

func (jlc JobLogCollection) Swap(i, j int) {
	jlc[i], jlc[j] = jlc[j], jlc[i]
}

// FilterBySource returns logs that match the specified source
func (jlc JobLogCollection) FilterBySource(source LogSource) JobLogCollection {
	var filtered JobLogCollection
	for _, log := range jlc {
		if log.Source == source {
			filtered = append(filtered, log)
		}
	}
	return filtered
}

// FilterByTimeRange returns logs within the specified time range
func (jlc JobLogCollection) FilterByTimeRange(start, end time.Time) JobLogCollection {
	var filtered JobLogCollection
	for _, log := range jlc {
		if (log.Timestamp.Equal(start) || log.Timestamp.After(start)) &&
			(log.Timestamp.Equal(end) || log.Timestamp.Before(end)) {
			filtered = append(filtered, log)
		}
	}
	return filtered
}

// GetErrorLogs returns only stderr logs from the collection
func (jlc JobLogCollection) GetErrorLogs() JobLogCollection {
	return jlc.FilterBySource(LogSourceStderr)
}

// GetSystemLogs returns only system logs from the collection
func (jlc JobLogCollection) GetSystemLogs() JobLogCollection {
	return jlc.FilterBySource(LogSourceSystem)
}

// GetApplicationLogs returns only application logs (stdout/stderr) from the collection
func (jlc JobLogCollection) GetApplicationLogs() JobLogCollection {
	var appLogs JobLogCollection
	for _, log := range jlc {
		if log.IsApplicationLog() {
			appLogs = append(appLogs, log)
		}
	}
	return appLogs
}

// ParseLogSource parses a string into a LogSource, returning an error if invalid
func ParseLogSource(s string) (LogSource, error) {
	source := LogSource(s)
	if !source.IsValid() {
		return "", fmt.Errorf("invalid log source: %s", s)
	}
	return source, nil
}

// AllLogSources returns all valid LogSource values
func AllLogSources() []LogSource {
	return []LogSource{
		LogSourceStdout,
		LogSourceStderr,
		LogSourceSystem,
	}
}
