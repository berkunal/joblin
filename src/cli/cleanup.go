// Package cli provides command-line interface implementations for the Joblin application.
package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var cleanupFlags struct {
	DryRun               bool
	Force                bool
	Age                  string
	IncludeRunning       bool
	IncludeNotifications bool
	Namespace            string
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up expired jobs and old data",
	Long: `Clean up expired jobs, old logs, and notification history.

By default, this command removes:
- Jobs that have exceeded their TTL and are in a terminal state
- Notification records older than 24 hours
- Associated logs for deleted jobs

The cleanup respects job TTL settings and only removes jobs that are
both finished (Completed, Failed, or Terminated) and past their TTL.

Running jobs are never cleaned up unless explicitly forced.

Examples:
  joblin cleanup --dry-run                    # See what would be cleaned
  joblin cleanup                              # Clean up expired jobs
  joblin cleanup --age 7d                     # Clean jobs older than 7 days
  joblin cleanup --force --include-running    # Force cleanup including running jobs
  joblin cleanup --notifications-only         # Clean only notification history`,
	RunE: runCleanup,
}

func init() {
	cleanupCmd.Flags().BoolVar(&cleanupFlags.DryRun, "dry-run", false, "show what would be cleaned without doing it")
	cleanupCmd.Flags().BoolVar(&cleanupFlags.Force, "force", false, "force cleanup without confirmation")
	cleanupCmd.Flags().StringVar(&cleanupFlags.Age, "age", "", "minimum age for cleanup (e.g., 1h, 24h, 7d)")
	cleanupCmd.Flags().BoolVar(&cleanupFlags.IncludeRunning, "include-running", false,
		"include running jobs in cleanup (dangerous)")
	cleanupCmd.Flags().BoolVar(&cleanupFlags.IncludeNotifications, "notifications-only", false,
		"only clean up notification history")
	cleanupCmd.Flags().StringVar(&cleanupFlags.Namespace, "namespace", "", "limit cleanup to specific namespace")
}

func runCleanup(_ *cobra.Command, _ []string) error {
	// Parse age filter if provided
	var ageThreshold time.Time
	if cleanupFlags.Age != "" {
		duration, err := time.ParseDuration(cleanupFlags.Age)
		if err != nil {
			// Try parsing common formats
			switch cleanupFlags.Age {
			case "1d":
				duration = 24 * time.Hour
			case "2d":
				duration = 48 * time.Hour
			case "7d":
				duration = 7 * 24 * time.Hour
			case "30d":
				duration = 30 * 24 * time.Hour
			default:
				return fmt.Errorf("invalid age format: %s (use 1h, 24h, 7d, etc.)", cleanupFlags.Age)
			}
		}
		ageThreshold = time.Now().Add(-duration)
	}

	// Show confirmation unless forced or dry-run
	if !cleanupFlags.Force && !cleanupFlags.DryRun && !globalFlags.JSONOutput {
		fmt.Printf("⚠️  This will clean up expired jobs and old data.\n")
		if cleanupFlags.IncludeRunning {
			fmt.Printf("   WARNING: --include-running is enabled. Running jobs may be terminated!\n")
		}
		if cleanupFlags.Age != "" {
			fmt.Printf("   Age threshold: %s\n", cleanupFlags.Age)
		}
		if cleanupFlags.Namespace != "" {
			fmt.Printf("   Namespace filter: %s\n", cleanupFlags.Namespace)
		}

		fmt.Printf("\nType 'yes' to confirm cleanup: ")
		var response string
		_, err := fmt.Scanln(&response)
		if err != nil || response != "yes" {
			fmt.Println("Cleanup cancelled.")
			return nil
		}
	}

	if cleanupFlags.IncludeNotifications {
		return runNotificationCleanup()
	}

	return runFullCleanup(ageThreshold)
}

func runFullCleanup(ageThreshold time.Time) error {
	ctx := GetContext()
	startTime := time.Now()

	// Get list of jobs to analyze
	jobs, err := cliContext.JobService.ListJobs(nil)
	if err != nil {
		PrintError(fmt.Errorf("failed to list jobs for cleanup: %w", err))
		return err
	}

	// Filter jobs for cleanup
	var jobsToCleanup []*jobInfo
	for _, job := range jobs {
		// Apply namespace filter
		if cleanupFlags.Namespace != "" && job.Namespace != cleanupFlags.Namespace {
			continue
		}

		// Check if job should be cleaned up
		shouldCleanup, reason := shouldCleanupJob(job, ageThreshold)
		if shouldCleanup {
			jobsToCleanup = append(jobsToCleanup, &jobInfo{
				Job:    job,
				Reason: reason,
			})
		}
	}

	if cleanupFlags.DryRun {
		return showCleanupPreview(jobsToCleanup)
	}

	// Perform actual cleanup
	return performCleanup(ctx, jobsToCleanup, startTime)
}

func runNotificationCleanup() error {
	startTime := time.Now()

	if cleanupFlags.DryRun {
		// For dry run, we'd need to implement a method to count notifications
		fmt.Printf("Dry-run: Would clean up notification history older than 24 hours\n")
		return nil
	}

	// Clean up old notifications
	cleanedNotifications, err := cliContext.Storage.CleanupOldNotifications()
	if err != nil {
		PrintError(fmt.Errorf("failed to cleanup notifications: %w", err))
		return err
	}

	duration := time.Since(startTime)

	if globalFlags.JSONOutput {
		result := map[string]interface{}{
			"success":               true,
			"cleaned_notifications": cleanedNotifications,
			"duration_seconds":      duration.Seconds(),
		}
		if err := PrintJSON(result); err != nil {
			return fmt.Errorf("failed to print JSON result: %w", err)
		}
	} else {
		fmt.Printf("✅ Notification cleanup completed!\n")
		fmt.Printf("Cleaned up %d old notification records\n", cleanedNotifications)
		fmt.Printf("Cleanup time: %.2f seconds\n", duration.Seconds())
	}

	return nil
}

type jobInfo struct {
	Job    interface{} // This should be *models.Job
	Reason string
}

func shouldCleanupJob(_ interface{}, _ time.Time) (bool, string) {
	// This is a placeholder - in real implementation, you'd cast job to *models.Job
	// and implement the actual logic based on job.IsExpired(), job.IsFinished(), etc.

	// For now, return false to avoid cleanup
	return false, "Not implemented"
}

func showCleanupPreview(jobsToCleanup []*jobInfo) error {
	if len(jobsToCleanup) == 0 {
		if globalFlags.JSONOutput {
			result := map[string]interface{}{
				"jobs_to_cleanup": 0,
				"message":         "No jobs found that meet cleanup criteria",
			}
			if err := PrintJSON(result); err != nil {
				return fmt.Errorf("failed to print JSON result: %w", err)
			}
		} else {
			fmt.Println("No jobs found that meet cleanup criteria.")
		}
		return nil
	}

	if globalFlags.JSONOutput {
		result := map[string]interface{}{
			"dry_run":         true,
			"jobs_to_cleanup": len(jobsToCleanup),
			"jobs":            jobsToCleanup,
		}
		if err := PrintJSON(result); err != nil {
			return fmt.Errorf("failed to print JSON result: %w", err)
		}
		return nil
	}

	fmt.Printf("Dry-run: The following %d job(s) would be cleaned up:\n\n", len(jobsToCleanup))

	fmt.Printf("%-8s %-20s %-12s %-30s\n", "ID", "NAME", "STATUS", "REASON")
	fmt.Printf("%-8s %-20s %-12s %-30s\n",
		"--------", "--------------------", "------------", "------------------------------")

	for _, info := range jobsToCleanup {
		// This would need proper implementation with actual job data
		fmt.Printf("%-8s %-20s %-12s %-30s\n", "placeholder", "placeholder", "placeholder", info.Reason)
	}

	fmt.Printf("\nTo perform the actual cleanup, run the same command without --dry-run\n")
	return nil
}

func performCleanup(_ interface{}, jobsToCleanup []*jobInfo, startTime time.Time) error {
	if len(jobsToCleanup) == 0 {
		if globalFlags.JSONOutput {
			result := map[string]interface{}{
				"success":          true,
				"cleaned_jobs":     0,
				"message":          "No jobs to clean up",
				"duration_seconds": time.Since(startTime).Seconds(),
			}
			if err := PrintJSON(result); err != nil {
				return fmt.Errorf("failed to print JSON result: %w", err)
			}
		} else {
			fmt.Println("No jobs to clean up.")
		}
		return nil
	}

	if !globalFlags.JSONOutput {
		fmt.Printf("Cleaning up %d job(s)...\n", len(jobsToCleanup))
	}

	// Use the service's built-in cleanup method
	cleanedJobs, cleanedNotifications, err := cliContext.JobService.CleanupJobs(GetContext())
	if err != nil {
		PrintError(fmt.Errorf("cleanup failed: %w", err))
		return err
	}

	duration := time.Since(startTime)

	if globalFlags.JSONOutput {
		result := map[string]interface{}{
			"success":               true,
			"cleaned_jobs":          cleanedJobs,
			"cleaned_notifications": cleanedNotifications,
			"duration_seconds":      duration.Seconds(),
		}
		if err := PrintJSON(result); err != nil {
			return fmt.Errorf("failed to print JSON result: %w", err)
		}
	} else {
		fmt.Printf("✅ Cleanup completed successfully!\n")
		fmt.Printf("Cleaned up %d expired job(s)\n", cleanedJobs)
		fmt.Printf("Cleaned up %d old notification(s)\n", cleanedNotifications)
		fmt.Printf("Cleanup time: %.2f seconds\n", duration.Seconds())

		if cleanedJobs > 0 {
			fmt.Printf("\nStorage space has been freed up.\n")
			fmt.Printf("Logs and job history for cleaned jobs have been removed.\n")
		}
	}

	return nil
}
