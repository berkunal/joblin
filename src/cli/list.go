package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/berkunal/joblin/src/models"
	"github.com/berkunal/joblin/src/services/joblib"
	"github.com/spf13/cobra"
)

var listFlags struct {
	Status  string
	Labels  []string
	Limit   int
	SortBy  string
	Reverse bool
	Wide    bool
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List jobs",
	Long: `List jobs with optional filtering and sorting.

By default, shows all jobs sorted by creation time (newest first).
Use filters to narrow down the results or sorting options to change the order.

Status Filters:
  Pending    - Jobs waiting to be scheduled
  Running    - Jobs currently executing
  Completed  - Successfully finished jobs
  Failed     - Jobs that failed with errors
  Terminated - Manually stopped jobs
  Unknown    - Jobs with undetermined status

Examples:
  joblin list
  joblin list --status Running
  joblin list --status Completed --limit 10
  joblin list --label env=prod --label team=data
  joblin list --sort-by name --wide
  joblin list --sort-by duration --reverse`,
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVar(&listFlags.Status, "status", "", "filter by job status")
	listCmd.Flags().StringSliceVar(&listFlags.Labels, "label", []string{}, "filter by labels (key=value)")
	listCmd.Flags().IntVar(&listFlags.Limit, "limit", 0, "limit number of results (0 = no limit)")
	listCmd.Flags().StringVar(&listFlags.SortBy, "sort-by", "created", "sort by: name, status, created, started, duration")
	listCmd.Flags().BoolVar(&listFlags.Reverse, "reverse", false, "reverse sort order")
	listCmd.Flags().BoolVarP(&listFlags.Wide, "wide", "w", false, "show additional columns")

	// Set up completion for the status flag
	listCmd.RegisterFlagCompletionFunc("status", jobStatusCompletion)

	// Set up completion for the sort-by flag
	listCmd.RegisterFlagCompletionFunc("sort-by", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		sortOptions := []string{"name", "status", "created", "started", "duration"}
		var filtered []string
		for _, option := range sortOptions {
			if strings.HasPrefix(option, toComplete) {
				switch option {
				case "name":
					filtered = append(filtered, option+"\tSort by job name")
				case "status":
					filtered = append(filtered, option+"\tSort by job status")
				case "created":
					filtered = append(filtered, option+"\tSort by creation time")
				case "started":
					filtered = append(filtered, option+"\tSort by start time")
				case "duration":
					filtered = append(filtered, option+"\tSort by job duration")
				}
			}
		}
		return filtered, cobra.ShellCompDirectiveDefault
	})
}

func runList(cmd *cobra.Command, args []string) error {
	// Parse filters
	var status models.JobStatus
	if listFlags.Status != "" {
		parsedStatus, err := models.ParseJobStatus(listFlags.Status)
		if err != nil {
			return fmt.Errorf("invalid status filter: %w", err)
		}
		status = parsedStatus
	}

	labels, err := parseLabels(listFlags.Labels)
	if err != nil {
		return fmt.Errorf("failed to parse label filters: %w", err)
	}

	// Create list options
	options := &joblib.JobListOptions{
		Status:    status,
		Namespace: GetEffectiveNamespace(),
		Labels:    labels,
		Limit:     listFlags.Limit,
	}

	// Get jobs from service
	jobs, err := cliContext.JobService.ListJobs(options)
	if err != nil {
		PrintError(fmt.Errorf("failed to list jobs: %w", err))
		return err
	}

	// Update job statuses from Kubernetes for running jobs
	ctx := GetContext()
	for _, job := range jobs {
		if !job.IsFinished() {
			updatedJob, err := cliContext.JobService.UpdateJobStatus(ctx, job.ID)
			if err != nil {
				// Log error but continue with other jobs
				if globalFlags.Verbose {
					fmt.Printf("Warning: Failed to update status for job %s: %v\n", job.ID, err)
				}
				continue
			}
			// Update the job in the slice
			*job = *updatedJob
		}
	}

	// Sort jobs
	sortJobs(jobs, listFlags.SortBy, listFlags.Reverse)

	// Apply limit if specified and not already applied by service
	if listFlags.Limit > 0 && len(jobs) > listFlags.Limit {
		jobs = jobs[:listFlags.Limit]
	}

	// Output results
	if globalFlags.JSONOutput {
		result := map[string]interface{}{
			"jobs":  jobs,
			"count": len(jobs),
		}
		PrintJSON(result)
		return nil
	}

	if len(jobs) == 0 {
		fmt.Println("No jobs found matching the specified criteria.")
		return nil
	}

	// Display jobs in table format
	if listFlags.Wide {
		displayJobsWideTable(jobs)
	} else {
		displayJobsTable(jobs)
	}

	// Show summary
	fmt.Printf("\nTotal: %d job(s)", len(jobs))
	if listFlags.Status != "" {
		fmt.Printf(" with status %s", listFlags.Status)
	}
	fmt.Println()

	return nil
}

func displayJobsTable(jobs []*models.Job) {
	fmt.Printf("%-8s %-20s %-12s %-10s %-20s\n", "ID", "NAME", "STATUS", "NAMESPACE", "CREATED")
	fmt.Printf("%-8s %-20s %-12s %-10s %-20s\n",
		strings.Repeat("-", 8),
		strings.Repeat("-", 20),
		strings.Repeat("-", 12),
		strings.Repeat("-", 10),
		strings.Repeat("-", 20))

	for _, job := range jobs {
		// Truncate long names and IDs for table display
		id := truncateString(job.ID, 8)
		name := truncateString(job.Name, 20)
		namespace := truncateString(job.Namespace, 10)

		// Format creation time
		created := job.CreatedAt.Format("2006-01-02 15:04")

		// Add status emoji/color
		status := formatJobStatus(job.Status, false)

		fmt.Printf("%-8s %-20s %-12s %-10s %-20s\n",
			id, name, status, namespace, created)
	}
}

func displayJobsWideTable(jobs []*models.Job) {
	fmt.Printf("%-8s %-15s %-12s %-10s %-15s %-10s %-20s %-10s\n",
		"ID", "NAME", "STATUS", "NAMESPACE", "STARTED", "DURATION", "CREATED", "LABELS")
	fmt.Printf("%-8s %-15s %-12s %-10s %-15s %-10s %-20s %-10s\n",
		strings.Repeat("-", 8),
		strings.Repeat("-", 15),
		strings.Repeat("-", 12),
		strings.Repeat("-", 10),
		strings.Repeat("-", 15),
		strings.Repeat("-", 10),
		strings.Repeat("-", 20),
		strings.Repeat("-", 10))

	for _, job := range jobs {
		// Truncate fields
		id := truncateString(job.ID, 8)
		name := truncateString(job.Name, 15)
		namespace := truncateString(job.Namespace, 10)

		// Format times
		var started, duration string
		if job.StartedAt != nil {
			started = job.StartedAt.Format("15:04:05")
			if job.IsFinished() && job.CompletedAt != nil {
				d := job.CompletedAt.Sub(*job.StartedAt)
				duration = formatDuration(d)
			} else {
				d := time.Since(*job.StartedAt)
				duration = formatDuration(d)
			}
		} else {
			started = "-"
			duration = "-"
		}

		created := job.CreatedAt.Format("2006-01-02 15:04")

		// Format labels
		labels := formatLabels(job.Labels, 10)

		// Add status with color
		status := formatJobStatus(job.Status, false)

		fmt.Printf("%-8s %-15s %-12s %-10s %-15s %-10s %-20s %-10s\n",
			id, name, status, namespace, started, duration, created, labels)
	}
}

func formatJobStatus(status models.JobStatus, useColor bool) string {
	if !useColor || globalFlags.JSONOutput {
		return string(status)
	}

	switch status {
	case models.StatusCompleted:
		return "✅ " + string(status)
	case models.StatusFailed:
		return "❌ " + string(status)
	case models.StatusRunning:
		return "🏃 " + string(status)
	case models.StatusPending:
		return "⏳ " + string(status)
	case models.StatusTerminated:
		return "🛑 " + string(status)
	case models.StatusUnknown:
		return "❓ " + string(status)
	default:
		return string(status)
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}

func formatLabels(labels map[string]string, maxLen int) string {
	if len(labels) == 0 {
		return "-"
	}

	var parts []string
	for key, value := range labels {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}

	result := strings.Join(parts, ",")
	return truncateString(result, maxLen)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func sortJobs(jobs []*models.Job, sortBy string, reverse bool) {
	var lessFn func(i, j int) bool

	switch sortBy {
	case "name":
		lessFn = func(i, j int) bool {
			return jobs[i].Name < jobs[j].Name
		}
	case "status":
		lessFn = func(i, j int) bool {
			return string(jobs[i].Status) < string(jobs[j].Status)
		}
	case "created":
		lessFn = func(i, j int) bool {
			return jobs[i].CreatedAt.After(jobs[j].CreatedAt) // Newer first by default
		}
	case "started":
		lessFn = func(i, j int) bool {
			if jobs[i].StartedAt == nil && jobs[j].StartedAt == nil {
				return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
			}
			if jobs[i].StartedAt == nil {
				return false // Not started jobs go to end
			}
			if jobs[j].StartedAt == nil {
				return true
			}
			return jobs[i].StartedAt.After(*jobs[j].StartedAt)
		}
	case "duration":
		lessFn = func(i, j int) bool {
			durI := jobs[i].Duration()
			durJ := jobs[j].Duration()
			return durI > durJ // Longer duration first by default
		}
	default:
		// Default to created time
		lessFn = func(i, j int) bool {
			return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
		}
	}

	if reverse {
		originalLessFn := lessFn
		lessFn = func(i, j int) bool {
			return originalLessFn(j, i)
		}
	}

	sort.Slice(jobs, lessFn)
}
