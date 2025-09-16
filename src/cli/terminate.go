package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var terminateFlags struct {
	Force   bool
	Timeout string
}

var terminateCmd = &cobra.Command{
	Use:   "terminate <job-id>",
	Short: "Terminate a running job",
	Long: `Terminate a running job on Kubernetes.

This command will stop the job execution and mark it as terminated.
The job's Kubernetes resources will be cleaned up, but the job record
and logs will remain in storage until cleanup.

Examples:
  joblin terminate abc123-def456-ghi789
  joblin terminate abc123 --force
  joblin terminate abc123 --timeout 30s`,
	Args: cobra.ExactArgs(1),
	RunE: runTerminate,
}

func init() {
	terminateCmd.Flags().BoolVar(&terminateFlags.Force, "force", false, "force termination without confirmation")
	terminateCmd.Flags().StringVar(&terminateFlags.Timeout, "timeout", "30s", "timeout for termination operation")

	// Set up job ID completion for the first argument
	terminateCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return jobIDCompletion(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func runTerminate(cmd *cobra.Command, args []string) error {
	jobID := args[0]

	if err := ValidateJobID(jobID); err != nil {
		PrintError(err)
		return err
	}

	// Get job information first
	job, err := cliContext.JobService.GetJob(jobID)
	if err != nil {
		PrintError(fmt.Errorf("failed to get job: %w", err))
		return err
	}

	// Check if job is already finished
	if job.IsFinished() {
		message := fmt.Sprintf("Job %s is already finished with status: %s", job.ID, job.Status)
		if globalFlags.JSONOutput {
			result := map[string]interface{}{
				"success": false,
				"message": message,
				"job":     job,
			}
			PrintJSON(result)
		} else {
			fmt.Printf("⚠️  %s\n", message)
			fmt.Printf("Current status: %s\n", job.Status)
			if job.CompletedAt != nil {
				fmt.Printf("Completed at: %s\n", job.CompletedAt.Format("2006-01-02 15:04:05 UTC"))
			}
		}
		return nil
	}

	// Parse timeout
	timeout, err := time.ParseDuration(terminateFlags.Timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout format: %w", err)
	}

	// Show confirmation unless forced
	if !terminateFlags.Force && !globalFlags.JSONOutput {
		fmt.Printf("⚠️  Are you sure you want to terminate job '%s'?\n", job.Name)
		fmt.Printf("   Job ID: %s\n", job.ID)
		fmt.Printf("   Status: %s\n", job.Status)
		fmt.Printf("   Namespace: %s\n", job.Namespace)

		if job.StartedAt != nil {
			runningTime := time.Since(*job.StartedAt)
			fmt.Printf("   Running for: %s\n", runningTime.Round(time.Second))
		}

		fmt.Printf("\nThis action cannot be undone. Type 'yes' to confirm: ")

		var response string
		_, err := fmt.Scanln(&response)
		if err != nil || response != "yes" {
			fmt.Println("Termination cancelled.")
			return nil
		}
	}

	// Create context with timeout
	ctx := GetContext()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Terminate the job
	if !globalFlags.JSONOutput {
		fmt.Printf("Terminating job %s...\n", job.ID)
	}

	startTime := time.Now()
	err = cliContext.JobService.TerminateJob(ctx, jobID)
	duration := time.Since(startTime)

	if err != nil {
		terminationError := fmt.Errorf("failed to terminate job: %w", err)
		PrintError(terminationError)
		return terminationError
	}

	// Get updated job status
	updatedJob, err := cliContext.JobService.GetJob(jobID)
	if err != nil {
		cliContext.Logger.Warnf("Failed to get updated job status: %v", err)
		updatedJob = job // Use original job data
	}

	// Output success result
	if globalFlags.JSONOutput {
		result := map[string]interface{}{
			"success":          true,
			"message":          "Job terminated successfully",
			"job":              updatedJob,
			"termination_time": duration.Seconds(),
		}
		PrintJSON(result)
	} else {
		fmt.Printf("✅ Job terminated successfully!\n")
		fmt.Printf("Job ID: %s\n", updatedJob.ID)
		fmt.Printf("Final Status: %s\n", updatedJob.Status)
		fmt.Printf("Termination time: %.2f seconds\n", duration.Seconds())

		if updatedJob.CompletedAt != nil {
			fmt.Printf("Terminated at: %s\n", updatedJob.CompletedAt.Format("2006-01-02 15:04:05 UTC"))
		}

		if updatedJob.StartedAt != nil && updatedJob.CompletedAt != nil {
			runTime := updatedJob.CompletedAt.Sub(*updatedJob.StartedAt)
			fmt.Printf("Total runtime: %s\n", runTime.Round(time.Second))
		}

		fmt.Printf("\nNext steps:\n")
		fmt.Printf("  View logs:    joblin logs %s\n", updatedJob.ID)
		fmt.Printf("  View status:  joblin status %s\n", updatedJob.ID)
		fmt.Printf("  Clean up:     joblin cleanup\n")

		// Show webhook notification info
		if updatedJob.WebhookURL != "" {
			fmt.Printf("\n📢 A termination notification will be sent to the configured webhook.\n")
		}
	}

	return nil
}
