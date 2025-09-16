package notifylib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/berkunal/joblin/src/models"
	"github.com/sirupsen/logrus"
)

const (
	DefaultTimeout     = 30 * time.Second
	MaxRetryAttempts   = 5
	RetryBackoffFactor = 2
	InitialRetryDelay  = 1 * time.Second
)

type TeamsMessage struct {
	Type       string                   `json:"@type"`
	Context    string                   `json:"@context"`
	ThemeColor string                   `json:"themeColor"`
	Summary    string                   `json:"summary"`
	Sections   []TeamsMessageSection    `json:"sections"`
	Actions    []TeamsMessageAction     `json:"potentialAction,omitempty"`
}

type TeamsMessageSection struct {
	ActivityTitle    string                    `json:"activityTitle"`
	ActivitySubtitle string                    `json:"activitySubtitle,omitempty"`
	ActivityImage    string                    `json:"activityImage,omitempty"`
	Facts            []TeamsMessageFact        `json:"facts,omitempty"`
	Text             string                    `json:"text,omitempty"`
}

type TeamsMessageFact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type TeamsMessageAction struct {
	Type    string                       `json:"@type"`
	Name    string                       `json:"name"`
	Targets []TeamsMessageActionTarget   `json:"targets"`
}

type TeamsMessageActionTarget struct {
	OS  string `json:"os"`
	URI string `json:"uri"`
}

type NotificationService struct {
	httpClient *http.Client
	logger     *logrus.Logger
}

func NewNotificationService(logger *logrus.Logger) *NotificationService {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.InfoLevel)
	}

	return &NotificationService{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		logger: logger,
	}
}

func (n *NotificationService) SendNotification(ctx context.Context, notification *models.Notification, job *models.Job) error {
	if err := notification.Validate(); err != nil {
		return fmt.Errorf("invalid notification: %w", err)
	}

	message, err := n.buildTeamsMessage(notification, job)
	if err != nil {
		return fmt.Errorf("failed to build Teams message: %w", err)
	}

	return n.sendTeamsMessage(ctx, notification.WebhookURL, message)
}

func (n *NotificationService) SendNotificationWithRetry(ctx context.Context, notification *models.Notification, job *models.Job) error {
	var lastError error

	for attempt := 0; attempt < MaxRetryAttempts; attempt++ {
		if attempt > 0 {
			delay := n.calculateRetryDelay(attempt)
			n.logger.WithFields(logrus.Fields{
				"attempt":      attempt + 1,
				"max_attempts": MaxRetryAttempts,
				"delay":        delay,
				"job_id":       job.ID,
			}).Info("Retrying notification delivery")

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Continue with retry
			}
		}

		err := n.SendNotification(ctx, notification, job)
		if err == nil {
			n.logger.WithFields(logrus.Fields{
				"attempt": attempt + 1,
				"job_id":  job.ID,
			}).Info("Notification delivered successfully")
			return nil
		}

		lastError = err
		notification.IncrementRetry(err.Error())

		n.logger.WithFields(logrus.Fields{
			"attempt": attempt + 1,
			"error":   err.Error(),
			"job_id":  job.ID,
		}).Warn("Notification delivery failed")
	}

	return fmt.Errorf("notification delivery failed after %d attempts: %w", MaxRetryAttempts, lastError)
}

func (n *NotificationService) calculateRetryDelay(attempt int) time.Duration {
	delay := InitialRetryDelay
	for i := 0; i < attempt; i++ {
		delay *= RetryBackoffFactor
	}
	return delay
}

func (n *NotificationService) buildTeamsMessage(notification *models.Notification, job *models.Job) (*TeamsMessage, error) {
	facts := n.buildMessageFacts(notification, job)

	message := &TeamsMessage{
		Type:       "MessageCard",
		Context:    "http://schema.org/extensions",
		ThemeColor: notification.GetColor(),
		Summary:    fmt.Sprintf("Job %s: %s", job.Name, notification.Type),
		Sections: []TeamsMessageSection{
			{
				ActivityTitle:    notification.GetTitle(),
				ActivitySubtitle: fmt.Sprintf("Job: %s", job.Name),
				Facts:           facts,
			},
		},
	}

	return message, nil
}

func (n *NotificationService) buildMessageFacts(notification *models.Notification, job *models.Job) []TeamsMessageFact {
	facts := []TeamsMessageFact{
		{Name: "Job ID", Value: job.ID},
		{Name: "Status", Value: string(notification.Type)},
		{Name: "Created", Value: job.CreatedAt.Format("2006-01-02 15:04:05 UTC")},
	}

	if job.StartedAt != nil {
		facts = append(facts, TeamsMessageFact{
			Name:  "Started",
			Value: job.StartedAt.Format("2006-01-02 15:04:05 UTC"),
		})
	}

	if job.CompletedAt != nil {
		facts = append(facts, TeamsMessageFact{
			Name:  "Completed",
			Value: job.CompletedAt.Format("2006-01-02 15:04:05 UTC"),
		})

		if notification.Duration != "" {
			facts = append(facts, TeamsMessageFact{
				Name:  "Duration",
				Value: notification.Duration,
			})
		}
	}

	if notification.ExitCode != nil {
		facts = append(facts, TeamsMessageFact{
			Name:  "Exit Code",
			Value: fmt.Sprintf("%d", *notification.ExitCode),
		})
	}

	facts = append(facts, TeamsMessageFact{
		Name:  "Namespace",
		Value: job.Namespace,
	})

	facts = append(facts, TeamsMessageFact{
		Name:  "Context",
		Value: job.ClusterContext,
	})

	if len(job.Labels) > 0 {
		labels := ""
		for key, value := range job.Labels {
			if labels != "" {
				labels += ", "
			}
			labels += fmt.Sprintf("%s=%s", key, value)
		}
		facts = append(facts, TeamsMessageFact{
			Name:  "Labels",
			Value: labels,
		})
	}

	return facts
}

func (n *NotificationService) sendTeamsMessage(ctx context.Context, webhookURL string, message *TeamsMessage) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal Teams message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Teams webhook returned non-success status: %d", resp.StatusCode)
	}

	n.logger.WithFields(logrus.Fields{
		"webhook_url": webhookURL,
		"status_code": resp.StatusCode,
	}).Debug("Teams message sent successfully")

	return nil
}

func (n *NotificationService) CreateJobNotification(job *models.Job, webhookURL string) (*models.Notification, error) {
	if !job.IsFinished() {
		return nil, fmt.Errorf("cannot create notification for unfinished job")
	}

	notificationType, err := models.NotificationTypeFromJobStatus(job.Status)
	if err != nil {
		return nil, fmt.Errorf("failed to determine notification type: %w", err)
	}

	notification, err := models.NewNotification(job.ID, notificationType, webhookURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	notification.JobName = job.Name
	if job.StartedAt != nil && job.CompletedAt != nil {
		duration := job.CompletedAt.Sub(*job.StartedAt)
		notification.Duration = duration.String()
	}
	notification.ExitCode = job.ExitCode

	return notification, nil
}

func (n *NotificationService) SendJobCompletionNotification(ctx context.Context, job *models.Job, webhookURL string) (*models.Notification, error) {
	notification, err := n.CreateJobNotification(job, webhookURL)
	if err != nil {
		return nil, err
	}

	err = n.SendNotificationWithRetry(ctx, notification, job)
	if err != nil {
		return notification, fmt.Errorf("failed to send notification: %w", err)
	}

	return notification, nil
}

func (n *NotificationService) TestWebhook(ctx context.Context, webhookURL string) error {
	testJob, err := models.NewJob("test-job", "/tmp/test.py", []byte("print('Hello, World!')"), nil)
	if err != nil {
		return fmt.Errorf("failed to create test job: %w", err)
	}

	testJob.SetCompleted(0)

	notification, err := n.CreateJobNotification(testJob, webhookURL)
	if err != nil {
		return fmt.Errorf("failed to create test notification: %w", err)
	}

	return n.SendNotification(ctx, notification, testJob)
}

func (n *NotificationService) ValidateWebhookURL(ctx context.Context, webhookURL string) error {
	req, err := http.NewRequestWithContext(ctx, "HEAD", webhookURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create validation request: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook URL is not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusMethodNotAllowed {
		// HEAD not allowed, but URL is reachable
		return nil
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook URL returned error status: %d", resp.StatusCode)
	}

	return nil
}

func (n *NotificationService) GetSupportedWebhookTypes() []string {
	return []string{"teams"}
}

func (n *NotificationService) SetTimeout(timeout time.Duration) {
	n.httpClient.Timeout = timeout
}

func (n *NotificationService) SetLogger(logger *logrus.Logger) {
	n.logger = logger
}