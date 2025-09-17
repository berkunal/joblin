package models

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

const (
	// ErrInvalidNotificationType is the error message format for invalid notification types
	ErrInvalidNotificationType = "invalid notification type: %s"
)

// NotificationType represents the type of notification to send
type NotificationType string

const (
	// NotificationSuccess indicates a successful job completion
	NotificationSuccess NotificationType = "Success"
	// NotificationFailure indicates a failed job completion
	NotificationFailure NotificationType = "Failure"
	// NotificationTerminated indicates a job that was terminated before completion
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

// IsValid checks if the NotificationType is one of the valid values
func (nt NotificationType) IsValid() bool {
	return validNotificationTypes[nt]
}

// UnmarshalJSON implements JSON unmarshaling for NotificationType
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

// MarshalJSON implements JSON marshaling for NotificationType
func (nt NotificationType) MarshalJSON() ([]byte, error) {
	if !nt.IsValid() {
		return nil, fmt.Errorf(ErrInvalidNotificationType, nt)
	}
	return json.Marshal(string(nt))
}

// Notification represents a webhook notification for job events
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

// NewNotification creates a new Notification with the provided parameters
func NewNotification(jobID string, notificationType NotificationType, webhookURL string) (*Notification, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job ID cannot be empty")
	}

	if !notificationType.IsValid() {
		return nil, fmt.Errorf(ErrInvalidNotificationType, notificationType)
	}

	if err := ValidateWebhookURL(webhookURL); err != nil {
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

// Validate checks if the Notification has valid field values
func (n *Notification) Validate() error {
	if n.JobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	if !n.Type.IsValid() {
		return fmt.Errorf(ErrInvalidNotificationType, n.Type)
	}

	if err := ValidateWebhookURL(n.WebhookURL); err != nil {
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

// CanRetry returns true if the notification can be retried
func (n *Notification) CanRetry() bool {
	return n.RetryCount < 5
}

// IncrementRetry increments the retry count and sets the error message
func (n *Notification) IncrementRetry(errorMessage string) {
	n.RetryCount++
	n.LastError = errorMessage
	n.SentAt = time.Now().UTC()
}

// MarkSent marks the notification as successfully sent
func (n *Notification) MarkSent(messageID string) {
	n.MessageID = messageID
	n.LastError = ""
}

// GetStorageKey returns a unique key for storing this notification
func (n *Notification) GetStorageKey() string {
	return fmt.Sprintf("%s:%d", n.JobID, n.SentAt.UnixNano())
}

// IsOlderThan returns true if the notification is older than the specified duration
func (n *Notification) IsOlderThan(duration time.Duration) bool {
	return time.Since(n.SentAt) > duration
}

// ShouldArchive returns true if the notification should be archived
func (n *Notification) ShouldArchive() bool {
	return n.IsOlderThan(24 * time.Hour)
}

// GetTitle returns the notification title based on the notification type
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

// GetColor returns the color for the notification based on the type
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

// FormatMessage formats the notification message with job details
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

// Clone creates a deep copy of the Notification
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

// ValidateWebhookURL validates that the webhook URL is properly formatted
func ValidateWebhookURL(webhookURL string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL cannot be empty")
	}

	parsedURL, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL format: %w", err)
	}

	// Check for unsupported protocols
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return fmt.Errorf("unsupported protocol: %s", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("webhook URL must have a valid host")
	}

	// Additional validation for production use - suggest HTTPS
	if parsedURL.Scheme == "http" {
		return fmt.Errorf("webhook URL should use HTTPS for security")
	}

	return nil
}

// ParseNotificationType parses a string into a NotificationType
func ParseNotificationType(s string) (NotificationType, error) {
	notificationType := NotificationType(s)
	if !notificationType.IsValid() {
		return "", fmt.Errorf(ErrInvalidNotificationType, s)
	}
	return notificationType, nil
}

// AllNotificationTypes returns all valid NotificationType values
func AllNotificationTypes() []NotificationType {
	return []NotificationType{
		NotificationSuccess,
		NotificationFailure,
		NotificationTerminated,
	}
}

// NotificationTypeFromJobStatus converts a JobStatus to the appropriate NotificationType
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
