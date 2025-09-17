package cli

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/berkunal/joblin/src/models"
	"github.com/spf13/cobra"
)

var logsFlags struct {
	Follow    bool
	Tail      int
	Since     string
	Source    string
	NoColor   bool
	Timestamp bool
}

var logsCmd = &cobra.Command{
	Use:   "logs <job-id>",
	Short: "View logs from a job",
	Long: `View logs from a deployed job.

This command can show logs from both running and completed jobs.
For running jobs, use --follow to see live log output.

Log Sources:
  stdout - Standard output from the Python script (default)
  stderr - Standard error from the Python script
  system - Kubernetes system messages
  all    - All log sources combined

Examples:
  joblin logs abc123-def456-ghi789
  joblin logs abc123 --follow
  joblin logs abc123 --tail 100
  joblin logs abc123 --source stderr
  joblin logs abc123 --since 5m --timestamp
  joblin logs abc123 --follow --no-color`,
	Args: cobra.ExactArgs(1),
	RunE: runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFlags.Follow, "follow", "f", false, "follow log output (for running jobs)")
	logsCmd.Flags().IntVar(&logsFlags.Tail, "tail", 100, "number of lines to show from the end")
	logsCmd.Flags().StringVar(&logsFlags.Since, "since", "",
		"show logs since timestamp (e.g., 5m, 1h, 2006-01-02T15:04:05Z)")
	logsCmd.Flags().StringVar(&logsFlags.Source, "source", "stdout", "log source: stdout, stderr, system, all")
	logsCmd.Flags().BoolVar(&logsFlags.NoColor, "no-color", false, "disable colored output")
	logsCmd.Flags().BoolVar(&logsFlags.Timestamp, "timestamp", false, "show timestamps")

	// Set up job ID completion for the first argument
	logsCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string,
		cobra.ShellCompDirective) {
		if len(args) == 0 {
			return jobIDCompletion(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Set up completion for the source flag
	if err := logsCmd.RegisterFlagCompletionFunc("source",
		func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		sources := []string{"stdout", "stderr", "system", "all"}
		var filtered []string
		for _, source := range sources {
			if strings.HasPrefix(source, toComplete) {
				switch source {
				case "stdout":
					filtered = append(filtered, source+"\tStandard output from the Python script")
				case "stderr":
					filtered = append(filtered, source+"\tStandard error from the Python script")
				case "system":
					filtered = append(filtered, source+"\tKubernetes system messages")
				case "all":
					filtered = append(filtered, source+"\tAll log sources combined")
				}
			}
		}
		return filtered, cobra.ShellCompDirectiveDefault
	}); err != nil {
		// Log error but don't fail the command setup
		fmt.Printf("Warning: Failed to register completion for source flag: %v\n", err)
	}
}

func runLogs(_ *cobra.Command, args []string) error {
	jobID := args[0]

	if err := ValidateJobID(jobID); err != nil {
		PrintError(err)
		return err
	}

	// Validate source flag
	validSources := []string{"stdout", "stderr", "system", "all"}
	if !contains(validSources, logsFlags.Source) {
		return fmt.Errorf("invalid source: %s (must be one of: %s)", logsFlags.Source, strings.Join(validSources, ", "))
	}

	// Get job to check status
	job, err := cliContext.JobService.GetJob(jobID)
	if err != nil {
		PrintError(fmt.Errorf("failed to get job: %w", err))
		return err
	}

	if logsFlags.Follow && job.IsFinished() {
		cliContext.Logger.Warn("Job is finished, --follow flag ignored")
		logsFlags.Follow = false
	}

	if logsFlags.Follow {
		return followLogs(job)
	}

	return showLogs(job)
}

func showLogs(job *models.Job) error {
	// Try to get live logs from Kubernetes first
	ctx := GetContext()
	liveLogReader, err := cliContext.JobService.GetJobLogs(ctx, job.ID, false)
	if err == nil {
		defer func() {
			if closeErr := liveLogReader.Close(); closeErr != nil {
				fmt.Printf("Warning: Failed to close live log reader: %v\n", closeErr)
			}
		}()
		return displayLogsFromReader(liveLogReader, job.ID)
	}

	cliContext.Logger.Debugf("Could not get live logs, falling back to stored logs: %v", err)

	// Fall back to stored logs
	storedLogs, err := cliContext.JobService.GetStoredJobLogs(job.ID)
	if err != nil {
		return fmt.Errorf("failed to get stored logs: %w", err)
	}

	return displayStoredLogs(storedLogs)
}

func followLogs(job *models.Job) error {
	ctx := GetContext()

	fmt.Printf("Following logs for job %s (Ctrl+C to stop)...\n", job.ID)

	logReader, err := cliContext.JobService.GetJobLogs(ctx, job.ID, true)
	if err != nil {
		return fmt.Errorf("failed to get job logs: %w", err)
	}
	defer func() {
		if closeErr := logReader.Close(); closeErr != nil {
			fmt.Printf("Warning: Failed to close log reader: %v\n", closeErr)
		}
	}()

	return displayLogsFromReader(logReader, job.ID)
}

func displayLogsFromReader(reader io.ReadCloser, _ string) error {
	scanner := bufio.NewScanner(reader)
	lineCount := 0

	for scanner.Scan() {
		line := scanner.Text()

		// Apply tail limit for non-follow mode
		if !logsFlags.Follow && logsFlags.Tail > 0 {
			lineCount++
			if lineCount > logsFlags.Tail {
				continue
			}
		}

		formattedLine := formatLogLine(line, models.LogSourceStdout, "", time.Now())
		fmt.Println(formattedLine)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading logs: %w", err)
	}

	return nil
}

func displayStoredLogs(logs models.JobLogCollection) error {
	// Filter logs based on criteria
	filteredLogs := filterLogs(logs)

	// Sort logs by timestamp
	sort.Sort(filteredLogs)

	// Apply tail limit
	if logsFlags.Tail > 0 && len(filteredLogs) > logsFlags.Tail {
		filteredLogs = filteredLogs[len(filteredLogs)-logsFlags.Tail:]
	}

	// Display logs
	for _, log := range filteredLogs {
		formattedLine := formatLogLine(log.Content, log.Source, log.PodName, log.Timestamp)
		fmt.Println(formattedLine)
	}

	if len(filteredLogs) == 0 {
		fmt.Println("No logs found matching the specified criteria.")
		return nil
	}

	if !globalFlags.JSONOutput {
		fmt.Printf("\n--- Showing %d log entries ---\n", len(filteredLogs))
	}

	return nil
}

func filterLogs(logs models.JobLogCollection) models.JobLogCollection {
	var filtered models.JobLogCollection

	// Parse since time if provided
	var sinceTime time.Time
	if logsFlags.Since != "" {
		var err error
		sinceTime, err = parseSinceTime(logsFlags.Since)
		if err != nil {
			cliContext.Logger.Warnf("Invalid since time format, ignoring: %v", err)
		}
	}

	for _, log := range logs {
		// Filter by source
		if logsFlags.Source != "all" {
			expectedSource := mapSourceFlag(logsFlags.Source)
			if log.Source != expectedSource {
				continue
			}
		}

		// Filter by since time
		if !sinceTime.IsZero() && log.Timestamp.Before(sinceTime) {
			continue
		}

		filtered = append(filtered, log)
	}

	return filtered
}

func formatLogLine(content string, source models.LogSource, podName string, timestamp time.Time) string {
	var parts []string

	// Add timestamp if requested
	if logsFlags.Timestamp {
		timeStr := timestamp.Format("2006-01-02T15:04:05.000Z")
		parts = append(parts, timeStr)
	}

	// Add source prefix with color
	if !logsFlags.NoColor {
		switch source {
		case models.LogSourceStdout:
			parts = append(parts, "\033[32m[stdout]\033[0m") // Green
		case models.LogSourceStderr:
			parts = append(parts, "\033[31m[stderr]\033[0m") // Red
		case models.LogSourceSystem:
			parts = append(parts, "\033[33m[system]\033[0m") // Yellow
		default:
			parts = append(parts, fmt.Sprintf("[%s]", source))
		}
	} else {
		parts = append(parts, fmt.Sprintf("[%s]", source))
	}

	// Add pod name if available
	if podName != "" && globalFlags.Verbose {
		parts = append(parts, fmt.Sprintf("[%s]", podName))
	}

	// Add the actual log content
	parts = append(parts, content)

	return strings.Join(parts, " ")
}

func parseSinceTime(since string) (time.Time, error) {
	// Try parsing as duration first (e.g., "5m", "1h")
	if duration, err := time.ParseDuration(since); err == nil {
		return time.Now().Add(-duration), nil
	}

	// Try parsing as absolute timestamp
	formats := []string{
		time.RFC3339,     // 2006-01-02T15:04:05Z07:00
		time.RFC3339Nano, // 2006-01-02T15:04:05.999999999Z07:00
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, since); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", since)
}

func mapSourceFlag(flag string) models.LogSource {
	switch flag {
	case "stdout":
		return models.LogSourceStdout
	case "stderr":
		return models.LogSourceStderr
	case "system":
		return models.LogSourceSystem
	default:
		return models.LogSourceStdout
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

