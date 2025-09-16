package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/berkunal/joblin/src/models"
	"github.com/berkunal/joblin/src/services/joblib"
	"github.com/spf13/cobra"
)

var deployFlags struct {
	Name         string
	Requirements string
	CPU          string
	Memory       string
	Storage      string
	TTL          string
	WebhookURL   string
	Labels       []string
	DryRun       bool
}

var deployCmd = &cobra.Command{
	Use:   "deploy <script.py>",
	Short: "Deploy a Python script to Kubernetes",
	Long: `Deploy a Python script to Kubernetes as a Job.

The script file must be a valid Python file. Dependencies can be specified
either via a requirements.txt file in the same directory or using the
--requirements flag.

Examples:
  joblin deploy script.py --name my-job
  joblin deploy script.py --name data-processing --cpu 500m --memory 1Gi
  joblin deploy script.py --requirements pandas,numpy --webhook-url https://...
  joblin deploy script.py --label env=prod --label team=data --ttl 2h
  joblin deploy script.py --dry-run  # validate without creating`,
	Args: cobra.ExactArgs(1),
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().StringVar(&deployFlags.Name, "name", "", "job name (default: derived from script filename)")
	deployCmd.Flags().StringVar(&deployFlags.Requirements, "requirements", "", "comma-separated list of Python packages")
	deployCmd.Flags().StringVar(&deployFlags.CPU, "cpu", "", "CPU limit (e.g., 100m, 1, 2)")
	deployCmd.Flags().StringVar(&deployFlags.Memory, "memory", "", "memory limit (e.g., 128Mi, 1Gi)")
	deployCmd.Flags().StringVar(&deployFlags.Storage, "storage", "", "ephemeral storage limit (e.g., 1Gi, 10Gi)")
	deployCmd.Flags().StringVar(&deployFlags.TTL, "ttl", "", "time-to-live for job cleanup (e.g., 1h, 24h, 7d)")
	deployCmd.Flags().StringVar(&deployFlags.WebhookURL, "webhook-url", "", "Teams webhook URL for notifications")
	deployCmd.Flags().StringSliceVar(&deployFlags.Labels, "label", []string{}, "labels to apply to the job (key=value)")
	deployCmd.Flags().BoolVar(&deployFlags.DryRun, "dry-run", false, "validate job without creating it")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	scriptPath := args[0]

	// Validate script file exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("script file not found: %s", scriptPath)
	}

	// Read script content
	scriptContent, err := ioutil.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to read script file: %w", err)
	}

	// Generate job name if not provided
	jobName := deployFlags.Name
	if jobName == "" {
		jobName = generateJobName(scriptPath)
	}

	// Parse dependencies
	dependencies, err := parseDependencies(scriptPath, deployFlags.Requirements)
	if err != nil {
		return fmt.Errorf("failed to parse dependencies: %w", err)
	}

	// Parse resource specifications
	resources, err := parseResourceSpec()
	if err != nil {
		return fmt.Errorf("failed to parse resource specifications: %w", err)
	}

	// Parse TTL
	ttl, err := parseTTL()
	if err != nil {
		return fmt.Errorf("failed to parse TTL: %w", err)
	}

	// Parse labels
	labels, err := parseLabels(deployFlags.Labels)
	if err != nil {
		return fmt.Errorf("failed to parse labels: %w", err)
	}

	// Get effective values
	namespace := GetEffectiveNamespace()
	context := GetEffectiveContext()
	webhookURL := deployFlags.WebhookURL
	if webhookURL == "" {
		webhookURL = cliContext.Config.TeamsWebhookURL
	}

	// Create job request
	request := &joblib.JobCreateRequest{
		Name:          jobName,
		ScriptPath:    scriptPath,
		ScriptContent: scriptContent,
		Dependencies:  dependencies,
		Namespace:     namespace,
		Context:       context,
		Resources:     resources,
		TTL:           ttl,
		WebhookURL:    webhookURL,
		Labels:        labels,
	}

	if deployFlags.DryRun {
		return runDryRun(request)
	}

	// Create and deploy job
	ctx := GetContext()
	job, err := cliContext.JobService.CreateJob(ctx, request)
	if err != nil {
		PrintError(fmt.Errorf("failed to deploy job: %w", err))
		return err
	}

	// Output result
	if globalFlags.JSONOutput {
		PrintJSON(job)
	} else {
		fmt.Printf("Job deployed successfully!\n")
		fmt.Printf("Job ID: %s\n", job.ID)
		fmt.Printf("Job Name: %s\n", job.Name)
		fmt.Printf("Kubernetes Job: %s\n", job.KubernetesJobName)
		fmt.Printf("Namespace: %s\n", job.Namespace)
		fmt.Printf("Status: %s\n", job.Status)

		if globalFlags.Verbose {
			fmt.Printf("\nJob Details:\n")
			fmt.Printf("  Script: %s\n", job.ScriptPath)
			fmt.Printf("  Dependencies: %v\n", job.Dependencies)
			fmt.Printf("  Resources: %s\n", job.ResourceLimits.String())
			fmt.Printf("  TTL: %s\n", job.TTL.String())
			if len(job.Labels) > 0 {
				fmt.Printf("  Labels: %v\n", job.Labels)
			}
		}

		fmt.Printf("\nNext steps:\n")
		fmt.Printf("  Check status: joblin status %s\n", job.ID)
		fmt.Printf("  View logs:    joblin logs %s --follow\n", job.ID)
		fmt.Printf("  Terminate:    joblin terminate %s\n", job.ID)
	}

	return nil
}

func generateJobName(scriptPath string) string {
	filename := filepath.Base(scriptPath)
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Make DNS-1123 compliant
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, " ", "-")

	// Add timestamp to ensure uniqueness
	timestamp := time.Now().Format("150405")
	return fmt.Sprintf("%s-%s", name, timestamp)
}

func parseDependencies(scriptPath, requirementsFlag string) ([]string, error) {
	var dependencies []string

	// Check for requirements.txt in same directory as script
	scriptDir := filepath.Dir(scriptPath)
	requirementsPath := filepath.Join(scriptDir, "requirements.txt")

	if _, err := os.Stat(requirementsPath); err == nil {
		content, err := ioutil.ReadFile(requirementsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read requirements.txt: %w", err)
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				dependencies = append(dependencies, line)
			}
		}
	}

	// Override/append with flag dependencies
	if requirementsFlag != "" {
		flagDeps := strings.Split(requirementsFlag, ",")
		for _, dep := range flagDeps {
			dep = strings.TrimSpace(dep)
			if dep != "" {
				dependencies = append(dependencies, dep)
			}
		}
	}

	return dependencies, nil
}

func parseResourceSpec() (*models.ResourceSpec, error) {
	if deployFlags.CPU == "" && deployFlags.Memory == "" && deployFlags.Storage == "" {
		return nil, nil // Use defaults from config
	}

	resources := &models.ResourceSpec{
		CPU:              deployFlags.CPU,
		Memory:           deployFlags.Memory,
		EphemeralStorage: deployFlags.Storage,
	}

	// Set defaults for empty values
	if resources.CPU == "" {
		resources.CPU = cliContext.Config.DefaultResources.CPU
	}
	if resources.Memory == "" {
		resources.Memory = cliContext.Config.DefaultResources.Memory
	}
	if resources.EphemeralStorage == "" {
		resources.EphemeralStorage = cliContext.Config.DefaultResources.EphemeralStorage
	}

	return resources, nil
}

func parseTTL() (time.Duration, error) {
	if deployFlags.TTL == "" {
		return 0, nil // Use default from config
	}

	ttl, err := time.ParseDuration(deployFlags.TTL)
	if err != nil {
		// Try parsing common formats
		switch deployFlags.TTL {
		case "1d":
			ttl = 24 * time.Hour
		case "2d":
			ttl = 48 * time.Hour
		case "3d":
			ttl = 72 * time.Hour
		case "7d":
			ttl = 7 * 24 * time.Hour
		default:
			return 0, fmt.Errorf("invalid TTL format: %s (use 1h, 24h, 7d, etc.)", deployFlags.TTL)
		}
	}

	return ttl, nil
}

func parseLabels(labelSlice []string) (map[string]string, error) {
	labels := make(map[string]string)

	for _, label := range labelSlice {
		parts := strings.SplitN(label, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid label format: %s (use key=value)", label)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return nil, fmt.Errorf("label key cannot be empty: %s", label)
		}

		labels[key] = value
	}

	return labels, nil
}

func runDryRun(request *joblib.JobCreateRequest) error {
	// Create a temporary job for validation
	job, err := models.NewJob(request.Name, request.ScriptPath, request.ScriptContent, request.Dependencies)
	if err != nil {
		return fmt.Errorf("job validation failed: %w", err)
	}

	job.Namespace = request.Namespace
	job.ClusterContext = request.Context

	if request.Resources != nil {
		job.ResourceLimits = *request.Resources
	}

	if request.TTL > 0 {
		job.TTL = request.TTL
	}

	job.WebhookURL = request.WebhookURL
	job.Labels = request.Labels

	// Validate the job
	if err := job.Validate(); err != nil {
		return fmt.Errorf("job validation failed: %w", err)
	}

	// Test Kubernetes connection
	ctx := GetContext()
	if err := cliContext.K8sService.TestConnection(ctx); err != nil {
		return fmt.Errorf("Kubernetes connection test failed: %w", err)
	}

	// Test webhook URL if provided
	if job.WebhookURL != "" {
		if err := cliContext.Notification.ValidateWebhookURL(ctx, job.WebhookURL); err != nil {
			cliContext.Logger.Warnf("Webhook URL validation failed: %v", err)
		}
	}

	// Output dry-run results
	if globalFlags.JSONOutput {
		dryRunResult := map[string]interface{}{
			"valid":      true,
			"job":        job,
			"would_create": true,
		}
		PrintJSON(dryRunResult)
	} else {
		fmt.Printf("Dry-run validation successful!\n")
		fmt.Printf("\nJob would be created with the following configuration:\n")
		fmt.Printf("  Name: %s\n", job.Name)
		fmt.Printf("  Kubernetes Job: %s\n", job.KubernetesJobName)
		fmt.Printf("  Namespace: %s\n", job.Namespace)
		fmt.Printf("  Context: %s\n", job.ClusterContext)
		fmt.Printf("  Resources: %s\n", job.ResourceLimits.String())
		fmt.Printf("  TTL: %s\n", job.TTL.String())
		fmt.Printf("  Dependencies: %v\n", job.Dependencies)
		if len(job.Labels) > 0 {
			fmt.Printf("  Labels: %v\n", job.Labels)
		}
		if job.WebhookURL != "" {
			fmt.Printf("  Webhook URL: %s\n", job.WebhookURL)
		}
		fmt.Printf("\nTo actually deploy the job, run the same command without --dry-run\n")
	}

	return nil
}