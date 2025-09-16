package joblib

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/berkunal/joblin/src/models"
	"github.com/berkunal/joblin/src/services/k8slib"
	"github.com/berkunal/joblin/src/services/notifylib"
	"github.com/berkunal/joblin/src/services/storage"
	"github.com/sirupsen/logrus"
)

type JobService struct {
	storage      *storage.StorageService
	k8s          *k8slib.K8sService
	notification *notifylib.NotificationService
	logger       *logrus.Logger
}

type JobCreateRequest struct {
	Name         string
	ScriptPath   string
	ScriptContent []byte
	Dependencies []string
	Namespace    string
	Context      string
	Resources    *models.ResourceSpec
	TTL          time.Duration
	WebhookURL   string
	Labels       map[string]string
}

type JobListOptions struct {
	Status    models.JobStatus
	Namespace string
	Labels    map[string]string
	Limit     int
}

func NewJobService(storage *storage.StorageService, k8s *k8slib.K8sService, notification *notifylib.NotificationService, logger *logrus.Logger) *JobService {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.InfoLevel)
	}

	return &JobService{
		storage:      storage,
		k8s:          k8s,
		notification: notification,
		logger:       logger,
	}
}

func (js *JobService) CreateJob(ctx context.Context, req *JobCreateRequest) (*models.Job, error) {
	if err := js.validateCreateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid create request: %w", err)
	}

	job, err := models.NewJob(req.Name, req.ScriptPath, req.ScriptContent, req.Dependencies)
	if err != nil {
		return nil, fmt.Errorf("failed to create job model: %w", err)
	}

	job.Namespace = req.Namespace
	job.ClusterContext = req.Context

	if req.Resources != nil {
		job.ResourceLimits = *req.Resources
	}

	if req.TTL > 0 {
		job.TTL = req.TTL
	}

	job.WebhookURL = req.WebhookURL

	if req.Labels != nil {
		job.Labels = req.Labels
	}

	if err := js.storage.SaveJob(job); err != nil {
		return nil, fmt.Errorf("failed to save job to storage: %w", err)
	}

	js.logger.WithFields(logrus.Fields{
		"job_id":    job.ID,
		"job_name":  job.Name,
		"namespace": job.Namespace,
	}).Info("Job created successfully")

	if err := js.k8s.CreateJob(ctx, job); err != nil {
		js.storage.DeleteJob(job.ID)
		return nil, fmt.Errorf("failed to create Kubernetes job: %w", err)
	}

	job.SetStarted()
	js.storage.UpdateJob(job)

	js.logger.WithFields(logrus.Fields{
		"job_id":         job.ID,
		"k8s_job_name":   job.KubernetesJobName,
		"namespace":      job.Namespace,
	}).Info("Kubernetes job created successfully")

	go js.monitorJob(context.Background(), job.ID)

	return job, nil
}

func (js *JobService) validateCreateRequest(req *JobCreateRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}

	if req.Name == "" {
		return fmt.Errorf("job name cannot be empty")
	}

	if req.ScriptPath == "" {
		return fmt.Errorf("script path cannot be empty")
	}

	if len(req.ScriptContent) == 0 {
		return fmt.Errorf("script content cannot be empty")
	}

	if req.Namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}

	if req.Resources != nil {
		if err := req.Resources.Validate(); err != nil {
			return fmt.Errorf("invalid resource specification: %w", err)
		}
	}

	if req.WebhookURL != "" {
		// Validate webhook URL format
		testNotification, err := models.NewNotification("test", models.NotificationSuccess, req.WebhookURL)
		if err != nil {
			return fmt.Errorf("invalid webhook URL: %w", err)
		}
		_ = testNotification
	}

	return nil
}

func (js *JobService) GetJob(jobID string) (*models.Job, error) {
	job, err := js.storage.GetJob(jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job from storage: %w", err)
	}

	return job, nil
}

func (js *JobService) UpdateJobStatus(ctx context.Context, jobID string) (*models.Job, error) {
	job, err := js.storage.GetJob(jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	if job.IsFinished() {
		return job, nil // No need to update finished jobs
	}

	status, err := js.k8s.GetJobStatus(ctx, job)
	if err != nil {
		js.logger.WithFields(logrus.Fields{
			"job_id": jobID,
			"error":  err.Error(),
		}).Warn("Failed to get job status from Kubernetes")
		return job, nil // Return existing job on K8s error
	}

	if status != job.Status {
		job.Status = status

		if status.IsTerminal() {
			exitCode, err := js.k8s.GetJobExitCode(ctx, job)
			if err == nil && exitCode != nil {
				if *exitCode == 0 {
					job.SetCompleted(*exitCode)
				} else {
					job.SetFailed(*exitCode)
				}
			} else {
				if status == models.StatusCompleted {
					job.SetCompleted(0)
				} else if status == models.StatusFailed {
					job.SetFailed(1)
				}
			}
		}

		if err := js.storage.UpdateJob(job); err != nil {
			return nil, fmt.Errorf("failed to update job in storage: %w", err)
		}

		js.logger.WithFields(logrus.Fields{
			"job_id":     jobID,
			"old_status": job.Status,
			"new_status": status,
		}).Info("Job status updated")

		if status.IsTerminal() && job.WebhookURL != "" {
			go js.sendCompletionNotification(context.Background(), job)
		}
	}

	return job, nil
}

func (js *JobService) monitorJob(ctx context.Context, jobID string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job, err := js.UpdateJobStatus(ctx, jobID)
			if err != nil {
				js.logger.WithFields(logrus.Fields{
					"job_id": jobID,
					"error":  err.Error(),
				}).Error("Failed to update job status during monitoring")
				continue
			}

			if job.IsFinished() {
				js.logger.WithFields(logrus.Fields{
					"job_id": jobID,
					"status": job.Status,
				}).Info("Job monitoring completed")
				return
			}
		}
	}
}

func (js *JobService) sendCompletionNotification(ctx context.Context, job *models.Job) {
	if job.WebhookURL == "" {
		return
	}

	notification, err := js.notification.SendJobCompletionNotification(ctx, job, job.WebhookURL)
	if err != nil {
		js.logger.WithFields(logrus.Fields{
			"job_id": job.ID,
			"error":  err.Error(),
		}).Error("Failed to send completion notification")

		if notification != nil {
			js.storage.SaveNotification(notification)
		}
		return
	}

	js.storage.SaveNotification(notification)

	js.logger.WithFields(logrus.Fields{
		"job_id": job.ID,
		"type":   notification.Type,
	}).Info("Completion notification sent successfully")
}

func (js *JobService) TerminateJob(ctx context.Context, jobID string) error {
	job, err := js.storage.GetJob(jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	if job.IsFinished() {
		return fmt.Errorf("job is already finished")
	}

	if err := js.k8s.DeleteJob(ctx, job); err != nil {
		return fmt.Errorf("failed to delete Kubernetes job: %w", err)
	}

	job.SetTerminated()
	if err := js.storage.UpdateJob(job); err != nil {
		return fmt.Errorf("failed to update job in storage: %w", err)
	}

	js.logger.WithFields(logrus.Fields{
		"job_id": jobID,
	}).Info("Job terminated successfully")

	if job.WebhookURL != "" {
		go js.sendCompletionNotification(context.Background(), job)
	}

	return nil
}

func (js *JobService) ListJobs(options *JobListOptions) ([]*models.Job, error) {
	if options == nil {
		return js.storage.ListJobs()
	}

	if options.Status != "" {
		return js.storage.ListJobsByStatus(options.Status)
	}

	jobs, err := js.storage.ListJobs()
	if err != nil {
		return nil, err
	}

	if len(options.Labels) > 0 {
		jobs = js.filterJobsByLabels(jobs, options.Labels)
	}

	if options.Namespace != "" {
		jobs = js.filterJobsByNamespace(jobs, options.Namespace)
	}

	if options.Limit > 0 && len(jobs) > options.Limit {
		jobs = jobs[:options.Limit]
	}

	return jobs, nil
}

func (js *JobService) filterJobsByLabels(jobs []*models.Job, labels map[string]string) []*models.Job {
	var filtered []*models.Job

	for _, job := range jobs {
		matches := true
		for key, value := range labels {
			if jobValue, exists := job.Labels[key]; !exists || jobValue != value {
				matches = false
				break
			}
		}
		if matches {
			filtered = append(filtered, job)
		}
	}

	return filtered
}

func (js *JobService) filterJobsByNamespace(jobs []*models.Job, namespace string) []*models.Job {
	var filtered []*models.Job

	for _, job := range jobs {
		if job.Namespace == namespace {
			filtered = append(filtered, job)
		}
	}

	return filtered
}

func (js *JobService) GetJobLogs(ctx context.Context, jobID string, follow bool) (io.ReadCloser, error) {
	job, err := js.storage.GetJob(jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return js.k8s.GetJobLogs(ctx, job, follow)
}

func (js *JobService) GetStoredJobLogs(jobID string) (models.JobLogCollection, error) {
	return js.storage.GetJobLogs(jobID)
}

func (js *JobService) SaveJobLog(log *models.JobLog) error {
	return js.storage.SaveJobLog(log)
}

func (js *JobService) DeleteJob(ctx context.Context, jobID string) error {
	job, err := js.storage.GetJob(jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	if !job.IsFinished() {
		if err := js.k8s.DeleteJob(ctx, job); err != nil {
			js.logger.WithFields(logrus.Fields{
				"job_id": jobID,
				"error":  err.Error(),
			}).Warn("Failed to delete Kubernetes job, continuing with storage cleanup")
		}
	}

	if err := js.storage.DeleteJob(jobID); err != nil {
		return fmt.Errorf("failed to delete job from storage: %w", err)
	}

	if err := js.storage.DeleteJobLogs(jobID); err != nil {
		js.logger.WithFields(logrus.Fields{
			"job_id": jobID,
			"error":  err.Error(),
		}).Warn("Failed to delete job logs")
	}

	js.logger.WithFields(logrus.Fields{
		"job_id": jobID,
	}).Info("Job deleted successfully")

	return nil
}

func (js *JobService) CleanupJobs(ctx context.Context) (int, int, error) {
	expiredJobs, err := js.storage.CleanupExpiredJobs()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to cleanup expired jobs: %w", err)
	}

	oldNotifications, err := js.storage.CleanupOldNotifications()
	if err != nil {
		return expiredJobs, 0, fmt.Errorf("failed to cleanup old notifications: %w", err)
	}

	js.logger.WithFields(logrus.Fields{
		"expired_jobs":       expiredJobs,
		"old_notifications": oldNotifications,
	}).Info("Cleanup completed")

	return expiredJobs, oldNotifications, nil
}

func (js *JobService) GetJobStats() (map[string]interface{}, error) {
	stats, err := js.storage.GetStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get storage stats: %w", err)
	}

	jobs, err := js.storage.ListJobs()
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs for stats: %w", err)
	}

	statusCounts := make(map[string]int)
	for _, job := range jobs {
		statusCounts[string(job.Status)]++
	}

	result := map[string]interface{}{
		"storage_stats":  stats,
		"status_counts":  statusCounts,
		"total_jobs":     len(jobs),
	}

	return result, nil
}

func (js *JobService) TestConnections(ctx context.Context) error {
	if err := js.k8s.TestConnection(ctx); err != nil {
		return fmt.Errorf("Kubernetes connection test failed: %w", err)
	}

	return nil
}

func (js *JobService) Close() error {
	if js.storage != nil {
		return js.storage.Close()
	}
	return nil
}