package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/berkunal/joblin/src/models"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration settings",
	Long: `Manage Joblin configuration settings.

Configuration is stored in ~/.joblin/config.yaml and includes:
- Default Kubernetes cluster and namespace
- Default resource limits (CPU, memory, storage)
- Default job TTL (time-to-live)
- Teams webhook URL for notifications
- Log level and data directory

Examples:
  joblin config view
  joblin config set webhook-url https://hooks.teams.microsoft.com/...
  joblin config set default-namespace production
  joblin config set default-cpu 500m
  joblin config set default-memory 1Gi
  joblin config set default-ttl 48h
  joblin config set log-level debug
  joblin config get webhook-url
  joblin config unset webhook-url`,
}

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "View current configuration",
	Long:  `Display the current configuration settings.`,
	RunE:  runConfigView,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Long: `Get the value of a specific configuration key.

Available keys:
  default-cluster       - Default Kubernetes context
  default-namespace     - Default namespace for jobs
  default-cpu          - Default CPU limit
  default-memory       - Default memory limit
  default-storage      - Default ephemeral storage limit
  default-ttl          - Default job time-to-live
  webhook-url          - Teams webhook URL
  log-level           - Logging verbosity level
  data-dir            - Local data directory path`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set the value of a configuration key.

See 'joblin config get --help' for available keys.

Examples:
  joblin config set webhook-url https://hooks.teams.microsoft.com/...
  joblin config set default-namespace production
  joblin config set default-cpu 500m
  joblin config set log-level debug`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

var configUnsetCmd = &cobra.Command{
	Use:   "unset <key>",
	Short: "Unset a configuration value",
	Long:  `Remove a configuration value, reverting to default.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigUnset,
}

func init() {
	configCmd.AddCommand(configViewCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configUnsetCmd)
}

func runConfigView(cmd *cobra.Command, args []string) error {
	config := cliContext.Config

	if globalFlags.JSONOutput {
		if err := PrintJSON(config); err != nil {
			return fmt.Errorf("failed to print JSON output: %w", err)
		}
		return nil
	}

	fmt.Printf("Joblin Configuration\n")
	fmt.Printf("====================\n\n")

	fmt.Printf("Kubernetes Settings:\n")
	fmt.Printf("  %-20s %s\n", "Default Cluster:", valueOrDefault(config.DefaultCluster, "(not set)"))
	fmt.Printf("  %-20s %s\n", "Default Namespace:", config.DefaultNamespace)

	fmt.Printf("\nDefault Resources:\n")
	fmt.Printf("  %-20s %s\n", "CPU:", config.DefaultResources.CPU)
	fmt.Printf("  %-20s %s\n", "Memory:", config.DefaultResources.Memory)
	fmt.Printf("  %-20s %s\n", "Storage:", config.DefaultResources.EphemeralStorage)
	fmt.Printf("  %-20s %s\n", "TTL:", config.DefaultTTL.String())

	fmt.Printf("\nNotification Settings:\n")
	fmt.Printf("  %-20s %s\n", "Webhook URL:", valueOrDefault(config.TeamsWebhookURL, "(not set)"))

	fmt.Printf("\nSystem Settings:\n")
	fmt.Printf("  %-20s %s\n", "Log Level:", config.LogLevel)
	fmt.Printf("  %-20s %s\n", "Data Directory:", config.DataDir)
	fmt.Printf("  %-20s %s\n", "Config Version:", config.ConfigVersion)
	fmt.Printf("  %-20s %s\n", "Last Updated:", config.LastUpdated.Format("2006-01-02 15:04:05 UTC"))

	fmt.Printf("\nConfiguration file: %s\n", config.GetConfigPath())
	fmt.Printf("Database file: %s\n", config.GetDatabasePath())

	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	config := cliContext.Config

	value, err := getConfigValue(config, key)
	if err != nil {
		PrintError(err)
		return err
	}

	if globalFlags.JSONOutput {
		result := map[string]interface{}{
			"key":   key,
			"value": value,
		}
		PrintJSON(result)
	} else {
		fmt.Printf("%s\n", value)
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	config := cliContext.Config

	err := setConfigValue(config, key, value)
	if err != nil {
		PrintError(fmt.Errorf("failed to set configuration: %w", err))
		return err
	}

	// Save configuration
	if err := config.Save(); err != nil {
		PrintError(fmt.Errorf("failed to save configuration: %w", err))
		return err
	}

	if globalFlags.JSONOutput {
		result := map[string]interface{}{
			"success": true,
			"key":     key,
			"value":   value,
			"message": fmt.Sprintf("Configuration updated: %s = %s", key, value),
		}
		PrintJSON(result)
	} else {
		fmt.Printf("✅ Configuration updated: %s = %s\n", key, value)
		fmt.Printf("Configuration saved to: %s\n", config.GetConfigPath())
	}

	return nil
}

func runConfigUnset(cmd *cobra.Command, args []string) error {
	key := args[0]
	config := cliContext.Config

	err := unsetConfigValue(config, key)
	if err != nil {
		PrintError(fmt.Errorf("failed to unset configuration: %w", err))
		return err
	}

	// Save configuration
	if err := config.Save(); err != nil {
		PrintError(fmt.Errorf("failed to save configuration: %w", err))
		return err
	}

	if globalFlags.JSONOutput {
		result := map[string]interface{}{
			"success": true,
			"key":     key,
			"message": fmt.Sprintf("Configuration unset: %s", key),
		}
		PrintJSON(result)
	} else {
		fmt.Printf("✅ Configuration unset: %s\n", key)
		fmt.Printf("Configuration saved to: %s\n", config.GetConfigPath())
	}

	return nil
}

func getConfigValue(config *models.CLIConfig, key string) (string, error) {
	switch key {
	case "default-cluster":
		return config.DefaultCluster, nil
	case "default-namespace":
		return config.DefaultNamespace, nil
	case "default-cpu":
		return config.DefaultResources.CPU, nil
	case "default-memory":
		return config.DefaultResources.Memory, nil
	case "default-storage":
		return config.DefaultResources.EphemeralStorage, nil
	case "default-ttl":
		return config.DefaultTTL.String(), nil
	case "webhook-url":
		return config.TeamsWebhookURL, nil
	case "log-level":
		return config.LogLevel, nil
	case "data-dir":
		return config.DataDir, nil
	default:
		return "", fmt.Errorf("unknown configuration key: %s", key)
	}
}

func setConfigValue(config *models.CLIConfig, key, value string) error {
	switch key {
	case "default-cluster":
		config.DefaultCluster = value
	case "default-namespace":
		if value == "" {
			return fmt.Errorf("default namespace cannot be empty")
		}
		config.DefaultNamespace = value
	case "default-cpu":
		if value == "" {
			return fmt.Errorf("default CPU cannot be empty")
		}
		// Validate CPU format
		tempResources := models.ResourceSpec{
			CPU:              value,
			Memory:           "128Mi",
			EphemeralStorage: "1Gi",
		}
		if err := tempResources.Validate(); err != nil {
			return fmt.Errorf("invalid CPU value: %w", err)
		}
		config.DefaultResources.CPU = value
	case "default-memory":
		if value == "" {
			return fmt.Errorf("default memory cannot be empty")
		}
		// Validate memory format
		tempResources := models.ResourceSpec{
			CPU:              "100m",
			Memory:           value,
			EphemeralStorage: "1Gi",
		}
		if err := tempResources.Validate(); err != nil {
			return fmt.Errorf("invalid memory value: %w", err)
		}
		config.DefaultResources.Memory = value
	case "default-storage":
		if value == "" {
			return fmt.Errorf("default storage cannot be empty")
		}
		// Validate storage format
		tempResources := models.ResourceSpec{
			CPU:              "100m",
			Memory:           "128Mi",
			EphemeralStorage: value,
		}
		if err := tempResources.Validate(); err != nil {
			return fmt.Errorf("invalid storage value: %w", err)
		}
		config.DefaultResources.EphemeralStorage = value
	case "default-ttl":
		if value == "" {
			return fmt.Errorf("default TTL cannot be empty")
		}
		ttl, err := parseTTLValue(value)
		if err != nil {
			return fmt.Errorf("invalid TTL value: %w", err)
		}
		config.DefaultTTL = ttl
	case "webhook-url":
		if value != "" {
			if err := config.UpdateWebhookURL(value); err != nil {
				return err
			}
		} else {
			config.TeamsWebhookURL = ""
		}
	case "log-level":
		if err := config.UpdateLogLevel(value); err != nil {
			return err
		}
	case "data-dir":
		if value == "" {
			return fmt.Errorf("data directory cannot be empty")
		}
		config.DataDir = value
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}

	return nil
}

func unsetConfigValue(config *models.CLIConfig, key string) error {
	switch key {
	case "default-cluster":
		config.DefaultCluster = ""
	case "webhook-url":
		config.TeamsWebhookURL = ""
	case "default-namespace":
		return fmt.Errorf("default namespace cannot be unset (use 'set' to change it)")
	case "default-cpu", "default-memory", "default-storage":
		return fmt.Errorf("default resources cannot be unset (use 'set' to change them)")
	case "default-ttl":
		return fmt.Errorf("default TTL cannot be unset (use 'set' to change it)")
	case "log-level":
		config.LogLevel = "info" // Reset to default
	case "data-dir":
		return fmt.Errorf("data directory cannot be unset (use 'set' to change it)")
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}

	return nil
}

func parseTTLValue(value string) (time.Duration, error) {
	// Try parsing as duration first
	if ttl, err := time.ParseDuration(value); err == nil {
		if ttl < time.Minute {
			return 0, fmt.Errorf("TTL must be at least 1 minute")
		}
		if ttl > 7*24*time.Hour {
			return 0, fmt.Errorf("TTL cannot exceed 7 days")
		}
		return ttl, nil
	}

	// Try parsing common formats
	switch value {
	case "1d":
		return 24 * time.Hour, nil
	case "2d":
		return 48 * time.Hour, nil
	case "3d":
		return 72 * time.Hour, nil
	case "7d":
		return 7 * 24 * time.Hour, nil
	default:
		// Try parsing as number of hours
		if hours, err := strconv.Atoi(value); err == nil {
			if hours < 1 {
				return 0, fmt.Errorf("TTL must be at least 1 hour")
			}
			if hours > 24*7 {
				return 0, fmt.Errorf("TTL cannot exceed 168 hours (7 days)")
			}
			return time.Duration(hours) * time.Hour, nil
		}
	}

	return 0, fmt.Errorf("invalid TTL format: %s (use 1h, 24h, 7d, etc.)", value)
}

func valueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
