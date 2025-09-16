package storage

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/berkunal/joblin/src/models"
	"go.etcd.io/bbolt"
)

const (
	JobsBucket          = "jobs"
	LogsBucket          = "logs"
	NotificationsBucket = "notifications"
	MetadataBucket      = "metadata"
)

type StorageService struct {
	db *bbolt.DB
}

func NewStorageService(dbPath string) (*StorageService, error) {
	db, err := bbolt.Open(dbPath, 0644, &bbolt.Options{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", dbPath, err)
	}

	service := &StorageService{db: db}

	if err := service.initializeBuckets(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database buckets: %w", err)
	}

	return service, nil
}

func (s *StorageService) initializeBuckets() error {
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

func (s *StorageService) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *StorageService) SaveJob(job *models.Job) error {
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

func (s *StorageService) GetJob(jobID string) (*models.Job, error) {
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

func (s *StorageService) UpdateJob(job *models.Job) error {
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

func (s *StorageService) DeleteJob(jobID string) error {
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

func (s *StorageService) ListJobs() ([]*models.Job, error) {
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

func (s *StorageService) ListJobsByStatus(status models.JobStatus) ([]*models.Job, error) {
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

func (s *StorageService) SaveJobLog(log *models.JobLog) error {
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

func (s *StorageService) GetJobLogs(jobID string) (models.JobLogCollection, error) {
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

		for k, v := cursor.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = cursor.Next() {
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

func (s *StorageService) DeleteJobLogs(jobID string) error {
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
		for k, _ := cursor.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, _ = cursor.Next() {
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

func (s *StorageService) SaveNotification(notification *models.Notification) error {
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

func (s *StorageService) GetNotifications(jobID string) ([]*models.Notification, error) {
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

		for k, v := cursor.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = cursor.Next() {
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

func (s *StorageService) CleanupExpiredJobs() (int, error) {
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

func (s *StorageService) CleanupOldNotifications() (int, error) {
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

func (s *StorageService) SetMetadata(key string, value string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(MetadataBucket))
		if bucket == nil {
			return fmt.Errorf("metadata bucket not found")
		}

		return bucket.Put([]byte(key), []byte(value))
	})
}

func (s *StorageService) GetMetadata(key string) (string, error) {
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

func (s *StorageService) GetStats() (map[string]int, error) {
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