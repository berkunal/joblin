package models

import (
	"encoding/json"
	"fmt"
	"time"
)

type LogSource string

const (
	LogSourceStdout LogSource = "Stdout"
	LogSourceStderr LogSource = "Stderr"
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

func (ls LogSource) IsValid() bool {
	return validLogSources[ls]
}

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

func (ls LogSource) MarshalJSON() ([]byte, error) {
	if !ls.IsValid() {
		return nil, fmt.Errorf("invalid log source: %s", ls)
	}
	return json.Marshal(string(ls))
}

type JobLog struct {
	JobID     string    `json:"job_id"`
	Timestamp time.Time `json:"timestamp"`
	Source    LogSource `json:"source"`
	Content   string    `json:"content"`
	PodName   string    `json:"pod_name"`
}

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

func (jl *JobLog) GetStorageKey() string {
	return fmt.Sprintf("%s:%d", jl.JobID, jl.Timestamp.UnixNano())
}

func (jl *JobLog) IsSystemLog() bool {
	return jl.Source == LogSourceSystem
}

func (jl *JobLog) IsApplicationLog() bool {
	return jl.Source == LogSourceStdout || jl.Source == LogSourceStderr
}

func (jl *JobLog) IsErrorLog() bool {
	return jl.Source == LogSourceStderr
}

func (jl *JobLog) Format() string {
	timeStr := jl.Timestamp.Format("2006-01-02 15:04:05")
	sourceStr := string(jl.Source)

	if jl.PodName != "" {
		return fmt.Sprintf("[%s] [%s] [%s] %s", timeStr, sourceStr, jl.PodName, jl.Content)
	}

	return fmt.Sprintf("[%s] [%s] %s", timeStr, sourceStr, jl.Content)
}

func (jl *JobLog) Clone() *JobLog {
	return &JobLog{
		JobID:     jl.JobID,
		Timestamp: jl.Timestamp,
		Source:    jl.Source,
		Content:   jl.Content,
		PodName:   jl.PodName,
	}
}

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

func (jlc JobLogCollection) FilterBySource(source LogSource) JobLogCollection {
	var filtered JobLogCollection
	for _, log := range jlc {
		if log.Source == source {
			filtered = append(filtered, log)
		}
	}
	return filtered
}

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

func (jlc JobLogCollection) GetErrorLogs() JobLogCollection {
	return jlc.FilterBySource(LogSourceStderr)
}

func (jlc JobLogCollection) GetSystemLogs() JobLogCollection {
	return jlc.FilterBySource(LogSourceSystem)
}

func (jlc JobLogCollection) GetApplicationLogs() JobLogCollection {
	var appLogs JobLogCollection
	for _, log := range jlc {
		if log.IsApplicationLog() {
			appLogs = append(appLogs, log)
		}
	}
	return appLogs
}

func ParseLogSource(s string) (LogSource, error) {
	source := LogSource(s)
	if !source.IsValid() {
		return "", fmt.Errorf("invalid log source: %s", s)
	}
	return source, nil
}

func AllLogSources() []LogSource {
	return []LogSource{
		LogSourceStdout,
		LogSourceStderr,
		LogSourceSystem,
	}
}
