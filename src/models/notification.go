package models

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

const (
	ErrInvalidNotificationType = "invalid notification type: %s"
)

type NotificationType string

const (
	NotificationSuccess    NotificationType = "Success"
	NotificationFailure    NotificationType = "Failure"
	NotificationTerminated NotificationType = "Terminated"
)

var validNotificationTypes = map[NotificationType]bool{
	NotificationSuccess:    true,
	NotificationFailure:    true,
	NotificationTerminated: true,
}

func (nt NotificationType) String() string {
	return string(nt)
}

func (nt NotificationType) IsValid() bool {
	return validNotificationTypes[nt]
}

func (nt *NotificationType) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	notificationType := NotificationType(str)
	if !notificationType.IsValid() {
		return fmt.Errorf(ErrInvalidNotificationType, str)
	}

	*nt = notificationType
	return nil
}

func (nt NotificationType) MarshalJSON() ([]byte, error) {
	if !nt.IsValid() {
		return nil, fmt.Errorf(ErrInvalidNotificationType, nt)
	}
	return json.Marshal(string(nt))
}

type Notification struct {
	JobID      string           `json:"job_id"`
	Type       NotificationType `json:"type"`
	SentAt     time.Time        `json:"sent_at"`
	WebhookURL string           `json:"webhook_url"`
	MessageID  string           `json:"message_id,omitempty"`
	RetryCount int              `json:"retry_count"`
	LastError  string           `json:"last_error,omitempty"`
	JobName    string           `json:"job_name,omitempty"`
	Duration   string           `json:"duration,omitempty"`
	ExitCode   *int             `json:"exit_code,omitempty"`
}

func NewNotification(jobID string, notificationType NotificationType, webhookURL string) (*Notification, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job ID cannot be empty")
	}

	if !notificationType.IsValid() {
		return nil, fmt.Errorf(ErrInvalidNotificationType, notificationType)
	}

	if err := validateWebhookURL(webhookURL); err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}

	return &Notification{
		JobID:      jobID,
		Type:       notificationType,
		SentAt:     time.Now().UTC(),
		WebhookURL: webhookURL,
		RetryCount: 0,
	}, nil
}

func (n *Notification) Validate() error {
	if n.JobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	if !n.Type.IsValid() {
		return fmt.Errorf(ErrInvalidNotificationType, n.Type)
	}

	if err := validateWebhookURL(n.WebhookURL); err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}

	if n.RetryCount < 0 {
		return fmt.Errorf("retry count cannot be negative")
	}

	if n.RetryCount > 5 {
		return fmt.Errorf("retry count cannot exceed 5")
	}

	if n.SentAt.IsZero() {
		return fmt.Errorf("sent at timestamp cannot be zero")
	}

	return nil
}

func (n *Notification) CanRetry() bool {
	return n.RetryCount < 5
}

func (n *Notification) IncrementRetry(errorMessage string) {
	n.RetryCount++
	n.LastError = errorMessage
	n.SentAt = time.Now().UTC()
}

func (n *Notification) MarkSent(messageID string) {
	n.MessageID = messageID
	n.LastError = ""
}

func (n *Notification) GetStorageKey() string {
	return fmt.Sprintf("%s:%d", n.JobID, n.SentAt.UnixNano())
}

func (n *Notification) IsOlderThan(duration time.Duration) bool {
	return time.Since(n.SentAt) > duration
}

func (n *Notification) ShouldArchive() bool {
	return n.IsOlderThan(24 * time.Hour)
}

func (n *Notification) GetTitle() string {
	switch n.Type {
	case NotificationSuccess:
		return "✅ Job Completed Successfully"
	case NotificationFailure:
		return "❌ Job Failed"
	case NotificationTerminated:
		return "🛑 Job Terminated"
	default:
		return "📋 Job Notification"
	}
}

func (n *Notification) GetColor() string {
	switch n.Type {
	case NotificationSuccess:
		return "00FF00" // Green
	case NotificationFailure:
		return "FF0000" // Red
	case NotificationTerminated:
		return "FFA500" // Orange
	default:
		return "0078D4" // Blue
	}
}

func (n *Notification) FormatMessage(job *Job) string {
	message := fmt.Sprintf("**Job:** %s\n", job.Name)
	message += fmt.Sprintf("**ID:** %s\n", job.ID)
	message += fmt.Sprintf("**Status:** %s\n", n.Type)

	if n.Duration != "" {
		message += fmt.Sprintf("**Duration:** %s\n", n.Duration)
	}

	if n.ExitCode != nil {
		message += fmt.Sprintf("**Exit Code:** %d\n", *n.ExitCode)
	}

	if len(job.Labels) > 0 {
		message += "**Labels:** "
		for key, value := range job.Labels {
			message += fmt.Sprintf("%s=%s ", key, value)
		}
		message += "\n"
	}

	message += fmt.Sprintf("**Created:** %s\n", job.CreatedAt.Format("2006-01-02 15:04:05 UTC"))

	if n.Type == NotificationFailure && n.LastError != "" {
		message += fmt.Sprintf("**Error:** %s\n", n.LastError)
	}

	return message
}

func (n *Notification) Clone() *Notification {
	clone := &Notification{
		JobID:      n.JobID,
		Type:       n.Type,
		SentAt:     n.SentAt,
		WebhookURL: n.WebhookURL,
		MessageID:  n.MessageID,
		RetryCount: n.RetryCount,
		LastError:  n.LastError,
		JobName:    n.JobName,
		Duration:   n.Duration,
	}

	if n.ExitCode != nil {
		exitCode := *n.ExitCode
		clone.ExitCode = &exitCode
	}

	return clone
}

func validateWebhookURL(webhookURL string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL cannot be empty")
	}

	parsedURL, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme != "https" {
		return fmt.Errorf("webhook URL must use HTTPS")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("webhook URL must have a valid host")
	}

	return nil
}

func ParseNotificationType(s string) (NotificationType, error) {
	notificationType := NotificationType(s)
	if !notificationType.IsValid() {
		return "", fmt.Errorf(ErrInvalidNotificationType, s)
	}
	return notificationType, nil
}

func AllNotificationTypes() []NotificationType {
	return []NotificationType{
		NotificationSuccess,
		NotificationFailure,
		NotificationTerminated,
	}
}

func NotificationTypeFromJobStatus(status JobStatus) (NotificationType, error) {
	switch status {
	case StatusCompleted:
		return NotificationSuccess, nil
	case StatusFailed:
		return NotificationFailure, nil
	case StatusTerminated:
		return NotificationTerminated, nil
	default:
		return "", fmt.Errorf("no notification type for job status: %s", status)
	}
}
