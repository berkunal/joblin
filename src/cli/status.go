package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/berkunal/joblin/src/models"
	"github.com/spf13/cobra"
)

var statusFlags struct {
	Watch    bool
	Interval string
}

var statusCmd = &cobra.Command{
	Use:   "status <job-id>",
	Short: "Get the status of a job",
	Long: `Get the current status of a deployed job.

The status command shows detailed information about a job including:
- Current status (Pending, Running, Completed, Failed, Terminated)
- Creation, start, and completion times
- Resource usage
- Exit code (if completed)
- Duration

Use --watch to monitor the job status continuously until completion.

Examples:
  joblin status abc123-def456-ghi789
  joblin status abc123 --watch
  joblin status abc123 --watch --interval 5s
  joblin status abc123 --json`,
	Args: cobra.ExactArgs(1),
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().BoolVarP(&statusFlags.Watch, "watch", "w", false, "watch job status until completion")
	statusCmd.Flags().StringVar(&statusFlags.Interval, "interval", "5s", "polling interval for watch mode")

	// Set up job ID completion for the first argument
	statusCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string,
		cobra.ShellCompDirective) {
		if len(args) == 0 {
			return jobIDCompletion(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func runStatus(_ *cobra.Command, args []string) error {
	jobID := args[0]

	if err := ValidateJobID(jobID); err != nil {
		PrintError(err)
		return err
	}

	if statusFlags.Watch {
		return watchJobStatus(jobID)
	}

	return showJobStatus(jobID)
}

func showJobStatus(jobID string) error {
	ctx := GetContext()

	// Update job status from Kubernetes
	job, err := cliContext.JobService.UpdateJobStatus(ctx, jobID)
	if err != nil {
		PrintError(fmt.Errorf("failed to get job status: %w", err))
		return err
	}

	if globalFlags.JSONOutput {
		if err := PrintJSON(job); err != nil {
			return fmt.Errorf("failed to output JSON: %w", err)
		}
		return nil
	}

	printJobStatusTable(job)
	return nil
}

func watchJobStatus(jobID string) error {
	interval, err := time.ParseDuration(statusFlags.Interval)
	if err != nil {
		return fmt.Errorf("invalid interval format: %w", err)
	}

	if globalFlags.JSONOutput {
		return watchJobStatusJSON(jobID, interval)
	}

	return watchJobStatusText(jobID, interval)
}

func watchJobStatusText(jobID string, interval time.Duration) error {
	fmt.Printf("Watching job status (polling every %s, Ctrl+C to stop)...\n\n", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastStatus models.JobStatus

	for {
		ctx := GetContext()
		job, err := cliContext.JobService.UpdateJobStatus(ctx, jobID)
		if err != nil {
			cliContext.Logger.Warnf("Failed to get job status: %v", err)
			time.Sleep(interval)
			continue
		}

		// Only print if status changed or first time
		if job.Status != lastStatus {
			timestamp := time.Now().Format("15:04:05")
			fmt.Printf("[%s] Status: %s", timestamp, job.Status)

			if job.Status == models.StatusRunning && job.StartedAt != nil {
				duration := time.Since(*job.StartedAt)
				fmt.Printf(" (running for %s)", duration.Round(time.Second))
			}

			if job.IsFinished() {
				if job.ExitCode != nil {
					fmt.Printf(" (exit code: %d)", *job.ExitCode)
				}
				if job.StartedAt != nil && job.CompletedAt != nil {
					duration := job.CompletedAt.Sub(*job.StartedAt)
					fmt.Printf(" (duration: %s)", duration.Round(time.Second))
				}
			}

			fmt.Printf("\n")
			lastStatus = job.Status
		}

		if job.IsFinished() {
			fmt.Printf("\nJob completed with status: %s\n", job.Status)
			printJobStatusTable(job)
			return nil
		}

		<-ticker.C
	}
}

func watchJobStatusJSON(jobID string, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		ctx := GetContext()
		job, err := cliContext.JobService.UpdateJobStatus(ctx, jobID)
		if err != nil {
			errorData := map[string]interface{}{
				"error":     err.Error(),
				"timestamp": time.Now(),
				"job_id":    jobID,
			}
			if jsonErr := PrintJSON(errorData); jsonErr != nil {
				fmt.Fprintf(os.Stderr, "Error printing JSON: %v\n", jsonErr)
			}
			time.Sleep(interval)
			continue
		}

		statusUpdate := map[string]interface{}{
			"job":       job,
			"timestamp": time.Now(),
			"finished":  job.IsFinished(),
		}

		if jsonErr := PrintJSON(statusUpdate); jsonErr != nil {
			fmt.Fprintf(os.Stderr, "Error printing JSON: %v\n", jsonErr)
		}

		if job.IsFinished() {
			return nil
		}

		<-ticker.C
	}
}

func printJobStatusTable(job *models.Job) {
	fmt.Printf("Job Status Report\n")
	fmt.Printf("=================\n\n")

	fmt.Printf("%-20s %s\n", "Job ID:", job.ID)
	fmt.Printf("%-20s %s\n", "Name:", job.Name)
	fmt.Printf("%-20s %s\n", "Status:", job.Status)
	fmt.Printf("%-20s %s\n", "Kubernetes Job:", job.KubernetesJobName)
	fmt.Printf("%-20s %s\n", "Namespace:", job.Namespace)
	fmt.Printf("%-20s %s\n", "Context:", job.ClusterContext)

	fmt.Printf("\nTiming Information:\n")
	fmt.Printf("%-20s %s\n", "Created:", job.CreatedAt.Format("2006-01-02 15:04:05 UTC"))

	if job.StartedAt != nil {
		fmt.Printf("%-20s %s\n", "Started:", job.StartedAt.Format("2006-01-02 15:04:05 UTC"))
	} else {
		fmt.Printf("%-20s %s\n", "Started:", "Not yet started")
	}

	if job.CompletedAt != nil {
		fmt.Printf("%-20s %s\n", "Completed:", job.CompletedAt.Format("2006-01-02 15:04:05 UTC"))

		if job.StartedAt != nil {
			duration := job.CompletedAt.Sub(*job.StartedAt)
			fmt.Printf("%-20s %s\n", "Duration:", duration.Round(time.Second))
		}
	} else if job.StartedAt != nil {
		duration := time.Since(*job.StartedAt)
		fmt.Printf("%-20s %s\n", "Running for:", duration.Round(time.Second))
	}

	fmt.Printf("\nResource Configuration:\n")
	fmt.Printf("%-20s %s\n", "CPU Limit:", job.ResourceLimits.CPU)
	fmt.Printf("%-20s %s\n", "Memory Limit:", job.ResourceLimits.Memory)
	fmt.Printf("%-20s %s\n", "Storage Limit:", job.ResourceLimits.EphemeralStorage)
	fmt.Printf("%-20s %s\n", "TTL:", job.TTL.String())

	if job.ExitCode != nil {
		fmt.Printf("\nExecution Result:\n")
		fmt.Printf("%-20s %d\n", "Exit Code:", *job.ExitCode)
	}

	if len(job.Labels) > 0 {
		fmt.Printf("\nLabels:\n")
		for key, value := range job.Labels {
			fmt.Printf("  %-18s %s\n", key+":", value)
		}
	}

	if len(job.Dependencies) > 0 {
		fmt.Printf("\nDependencies:\n")
		for _, dep := range job.Dependencies {
			fmt.Printf("  - %s\n", dep)
		}
	}

	if job.WebhookURL != "" {
		fmt.Printf("\nNotification:\n")
		fmt.Printf("%-20s %s\n", "Webhook URL:", job.WebhookURL)
	}

	// Show status-specific information
	switch job.Status {
	case models.StatusPending:
		fmt.Printf("\n💡 The job is waiting to be scheduled on Kubernetes.\n")
		fmt.Printf("   Check cluster resources if it stays pending for too long.\n")
	case models.StatusRunning:
		fmt.Printf("\n🏃 The job is currently running.\n")
		fmt.Printf("   Use 'joblin logs %s --follow' to see live output.\n", job.ID)
	case models.StatusCompleted:
		fmt.Printf("\n✅ The job completed successfully!\n")
		if job.ExitCode != nil && *job.ExitCode == 0 {
			fmt.Printf("   Exit code: 0 (success)\n")
		}
	case models.StatusFailed:
		fmt.Printf("\n❌ The job failed.\n")
		if job.ExitCode != nil {
			fmt.Printf("   Exit code: %d\n", *job.ExitCode)
		}
		fmt.Printf("   Use 'joblin logs %s' to see error details.\n", job.ID)
	case models.StatusTerminated:
		fmt.Printf("\n🛑 The job was terminated.\n")
		fmt.Printf("   This usually means it was manually stopped.\n")
	case models.StatusUnknown:
		fmt.Printf("\n❓ The job status is unknown.\n")
		fmt.Printf("   This might indicate cluster connectivity issues.\n")
	}

	fmt.Printf("\nNext Actions:\n")
	if !job.IsFinished() {
		fmt.Printf("  View logs:    joblin logs %s --follow\n", job.ID)
		fmt.Printf("  Terminate:    joblin terminate %s\n", job.ID)
	} else {
		fmt.Printf("  View logs:    joblin logs %s\n", job.ID)
		fmt.Printf("  Delete job:   joblin list --delete %s\n", job.ID)
	}
}
