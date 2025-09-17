// Package storage provides persistent data storage functionality using BBolt database.
// It handles jobs, logs, notifications, and metadata storage with efficient querying and cleanup.
package storage

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/berkunal/joblin/src/models"
	"go.etcd.io/bbolt"
)

const (
	// JobsBucket is the name of the BBolt bucket for storing job data
	JobsBucket          = "jobs"
	// LogsBucket is the name of the BBolt bucket for storing job logs
	LogsBucket          = "logs"
	// NotificationsBucket is the name of the BBolt bucket for storing notification data
	NotificationsBucket = "notifications"
	// MetadataBucket is the name of the BBolt bucket for storing metadata
	MetadataBucket      = "metadata"
)

// Service provides persistent storage operations using BBolt database
// Service provides persistent storage operations using BBolt database
type Service struct {
	db *bbolt.DB
}

// NewService creates a new storage service with the specified database path
func NewService(dbPath string) (*Service, error) {
	db, err := bbolt.Open(dbPath, 0644, &bbolt.Options{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", dbPath, err)
	}

	service := &Service{db: db}

	if err := service.initializeBuckets(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to initialize database buckets and close db: %w, %w", err, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize database buckets: %w", err)
	}

	return service, nil
}

func (s *Service) initializeBuckets() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		buckets := []string{JobsBucket, LogsBucket, NotificationsBucket, MetadataBucket}

		for _, bucket := range buckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}

		return nil
	})
}

// Close closes the underlying database connection
func (s *Service) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// SaveJob stores a job in the database
func (s *Service) SaveJob(job *models.Job) error {
	if err := job.Validate(); err != nil {
		return fmt.Errorf("invalid job: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(JobsBucket))
		if bucket == nil {
			return fmt.Errorf("jobs bucket not found")
		}

		data, err := json.Marshal(job)
		if err != nil {
			return fmt.Errorf("failed to marshal job: %w", err)
		}

		if err := bucket.Put([]byte(job.ID), data); err != nil {
			return fmt.Errorf("failed to save job: %w", err)
		}

		return nil
	})
}

// GetJob retrieves a job by ID from the database
func (s *Service) GetJob(jobID string) (*models.Job, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job ID cannot be empty")
	}

	var job models.Job

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(JobsBucket))
		if bucket == nil {
			return fmt.Errorf("jobs bucket not found")
		}

		data := bucket.Get([]byte(jobID))
		if data == nil {
			return fmt.Errorf("job not found: %s", jobID)
		}

		return json.Unmarshal(data, &job)
	})

	if err != nil {
		return nil, err
	}

	return &job, nil
}

// UpdateJob updates an existing job in the database
func (s *Service) UpdateJob(job *models.Job) error {
	if err := job.Validate(); err != nil {
		return fmt.Errorf("invalid job: %w", err)
	}

	existing, err := s.GetJob(job.ID)
	if err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	_ = existing // Ensure job exists

	return s.SaveJob(job)
}

// DeleteJob removes a job and all its associated data from storage
func (s *Service) DeleteJob(jobID string) error {
	if jobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(JobsBucket))
		if bucket == nil {
			return fmt.Errorf("jobs bucket not found")
		}

		if err := bucket.Delete([]byte(jobID)); err != nil {
			return fmt.Errorf("failed to delete job: %w", err)
		}

		return nil
	})
}

// ListJobs retrieves all jobs from the database
func (s *Service) ListJobs() ([]*models.Job, error) {
	var jobs []*models.Job

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(JobsBucket))
		if bucket == nil {
			return fmt.Errorf("jobs bucket not found")
		}

		return bucket.ForEach(func(k, v []byte) error {
			var job models.Job
			if err := json.Unmarshal(v, &job); err != nil {
				return fmt.Errorf("failed to unmarshal job %s: %w", string(k), err)
			}
			jobs = append(jobs, &job)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return jobs, nil
}

// ListJobsByStatus retrieves all jobs with the specified status
func (s *Service) ListJobsByStatus(status models.JobStatus) ([]*models.Job, error) {
	allJobs, err := s.ListJobs()
	if err != nil {
		return nil, err
	}

	var filteredJobs []*models.Job
	for _, job := range allJobs {
		if job.Status == status {
			filteredJobs = append(filteredJobs, job)
		}
	}

	return filteredJobs, nil
}

// SaveJobLog stores a job log entry in the database
func (s *Service) SaveJobLog(log *models.JobLog) error {
	if err := log.Validate(); err != nil {
		return fmt.Errorf("invalid job log: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(LogsBucket))
		if bucket == nil {
			return fmt.Errorf("logs bucket not found")
		}

		data, err := json.Marshal(log)
		if err != nil {
			return fmt.Errorf("failed to marshal job log: %w", err)
		}

		key := log.GetStorageKey()
		if err := bucket.Put([]byte(key), data); err != nil {
			return fmt.Errorf("failed to save job log: %w", err)
		}

		return nil
	})
}

// GetJobLogs retrieves all stored logs for a specific job
func (s *Service) GetJobLogs(jobID string) (models.JobLogCollection, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job ID cannot be empty")
	}

	var logs models.JobLogCollection

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(LogsBucket))
		if bucket == nil {
			return fmt.Errorf("logs bucket not found")
		}

		prefix := []byte(jobID + ":")
		cursor := bucket.Cursor()

		for k, v := cursor.Seek(prefix); k != nil && len(k) >= len(prefix) &&
			string(k[:len(prefix)]) == string(prefix); k, v = cursor.Next() {
			var log models.JobLog
			if err := json.Unmarshal(v, &log); err != nil {
				return fmt.Errorf("failed to unmarshal job log %s: %w", string(k), err)
			}
			logs = append(logs, &log)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return logs, nil
}

// DeleteJobLogs removes all logs associated with a job
func (s *Service) DeleteJobLogs(jobID string) error {
	if jobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(LogsBucket))
		if bucket == nil {
			return fmt.Errorf("logs bucket not found")
		}

		prefix := []byte(jobID + ":")
		cursor := bucket.Cursor()

		var keysToDelete [][]byte
		for k, _ := cursor.Seek(prefix); k != nil && len(k) >= len(prefix) &&
			string(k[:len(prefix)]) == string(prefix); k, _ = cursor.Next() {
			keysToDelete = append(keysToDelete, append([]byte(nil), k...))
		}

		for _, key := range keysToDelete {
			if err := bucket.Delete(key); err != nil {
				return fmt.Errorf("failed to delete job log %s: %w", string(key), err)
			}
		}

		return nil
	})
}

// SaveNotification stores a notification in the database
func (s *Service) SaveNotification(notification *models.Notification) error {
	if err := notification.Validate(); err != nil {
		return fmt.Errorf("invalid notification: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(NotificationsBucket))
		if bucket == nil {
			return fmt.Errorf("notifications bucket not found")
		}

		data, err := json.Marshal(notification)
		if err != nil {
			return fmt.Errorf("failed to marshal notification: %w", err)
		}

		key := notification.GetStorageKey()
		if err := bucket.Put([]byte(key), data); err != nil {
			return fmt.Errorf("failed to save notification: %w", err)
		}

		return nil
	})
}

// GetNotifications retrieves all notifications for a specific job
func (s *Service) GetNotifications(jobID string) ([]*models.Notification, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job ID cannot be empty")
	}

	var notifications []*models.Notification

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(NotificationsBucket))
		if bucket == nil {
			return fmt.Errorf("notifications bucket not found")
		}

		prefix := []byte(jobID + ":")
		cursor := bucket.Cursor()

		for k, v := cursor.Seek(prefix); k != nil && len(k) >= len(prefix) &&
			string(k[:len(prefix)]) == string(prefix); k, v = cursor.Next() {
			var notification models.Notification
			if err := json.Unmarshal(v, &notification); err != nil {
				return fmt.Errorf("failed to unmarshal notification %s: %w", string(k), err)
			}
			notifications = append(notifications, &notification)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return notifications, nil
}

// CleanupExpiredJobs removes jobs that have exceeded their TTL
func (s *Service) CleanupExpiredJobs() (int, error) {
	jobs, err := s.ListJobs()
	if err != nil {
		return 0, fmt.Errorf("failed to list jobs: %w", err)
	}

	var expiredJobIDs []string
	for _, job := range jobs {
		if job.IsExpired() && job.IsFinished() {
			expiredJobIDs = append(expiredJobIDs, job.ID)
		}
	}

	for _, jobID := range expiredJobIDs {
		if err := s.DeleteJob(jobID); err != nil {
			return 0, fmt.Errorf("failed to delete expired job %s: %w", jobID, err)
		}

		if err := s.DeleteJobLogs(jobID); err != nil {
			return 0, fmt.Errorf("failed to delete logs for expired job %s: %w", jobID, err)
		}
	}

	return len(expiredJobIDs), nil
}

// CleanupOldNotifications removes old notification records
func (s *Service) CleanupOldNotifications() (int, error) {
	var oldNotifications []string

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(NotificationsBucket))
		if bucket == nil {
			return fmt.Errorf("notifications bucket not found")
		}

		return bucket.ForEach(func(k, v []byte) error {
			var notification models.Notification
			if err := json.Unmarshal(v, &notification); err != nil {
				return fmt.Errorf("failed to unmarshal notification %s: %w", string(k), err)
			}

			if notification.ShouldArchive() {
				oldNotifications = append(oldNotifications, string(k))
			}

			return nil
		})
	})

	if err != nil {
		return 0, err
	}

	err = s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(NotificationsBucket))
		if bucket == nil {
			return fmt.Errorf("notifications bucket not found")
		}

		for _, key := range oldNotifications {
			if err := bucket.Delete([]byte(key)); err != nil {
				return fmt.Errorf("failed to delete old notification %s: %w", key, err)
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return len(oldNotifications), nil
}

// SetMetadata stores a key-value pair in the metadata bucket
func (s *Service) SetMetadata(key string, value string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(MetadataBucket))
		if bucket == nil {
			return fmt.Errorf("metadata bucket not found")
		}

		return bucket.Put([]byte(key), []byte(value))
	})
}

// GetMetadata retrieves a value from the metadata bucket
func (s *Service) GetMetadata(key string) (string, error) {
	var value string

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(MetadataBucket))
		if bucket == nil {
			return fmt.Errorf("metadata bucket not found")
		}

		data := bucket.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("metadata key not found: %s", key)
		}

		value = string(data)
		return nil
	})

	return value, err
}

// GetStats returns statistical information about stored data
func (s *Service) GetStats() (map[string]int, error) {
	stats := make(map[string]int)

	err := s.db.View(func(tx *bbolt.Tx) error {
		if bucket := tx.Bucket([]byte(JobsBucket)); bucket != nil {
			stats["total_jobs"] = bucket.Stats().KeyN
		}

		if bucket := tx.Bucket([]byte(LogsBucket)); bucket != nil {
			stats["total_logs"] = bucket.Stats().KeyN
		}

		if bucket := tx.Bucket([]byte(NotificationsBucket)); bucket != nil {
			stats["total_notifications"] = bucket.Stats().KeyN
		}

		return nil
	})

	return stats, err
}
