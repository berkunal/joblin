package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/berkunal/joblin/src/lib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	AppName    = "joblin"
	AppVersion = "0.1.0"
)

type GlobalFlags struct {
	ConfigFile string
	Context    string
	Namespace  string
	JSONOutput bool
	Verbose    bool
	LogLevel   string
}

var (
	globalFlags  = &GlobalFlags{}
	cliContext   *lib.ServiceContainer
	configMgr    *lib.ConfigManager
	errorHandler *lib.CLIErrorHandler
)

var rootCmd = &cobra.Command{
	Use:     AppName,
	Short:   "Joblin - Deploy Python scripts to Kubernetes clusters",
	Long: `Joblin is a CLI tool that enables developers to deploy Python scripts
to Kubernetes clusters easily. It provides job lifecycle management
(create, monitor, terminate) with Teams webhook notifications.

Examples:
  joblin deploy script.py --name my-job
  joblin status my-job-id
  joblin logs my-job-id --follow
  joblin terminate my-job-id
  joblin list --status Running
  joblin cleanup --dry-run
  joblin config set webhook-url https://hooks.teams.microsoft.com/...`,
	Version: AppVersion,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initializeCLI()
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return cleanupCLI()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&globalFlags.ConfigFile, "config", "", "config file (default is $HOME/.joblin/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&globalFlags.Context, "context", "", "Kubernetes context to use (overrides config)")
	rootCmd.PersistentFlags().StringVarP(&globalFlags.Namespace, "namespace", "n", "", "Kubernetes namespace (overrides config)")
	rootCmd.PersistentFlags().BoolVar(&globalFlags.JSONOutput, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&globalFlags.Verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVar(&globalFlags.LogLevel, "log-level", "", "log level (debug, info, warn, error)")

	// Bind flags to viper
	viper.BindPFlag("context", rootCmd.PersistentFlags().Lookup("context"))
	viper.BindPFlag("namespace", rootCmd.PersistentFlags().Lookup("namespace"))
	viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))

	// Add all subcommands
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(terminateCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(configCmd)
}

func initConfig() {
	if globalFlags.ConfigFile != "" {
		viper.SetConfigFile(globalFlags.ConfigFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
			os.Exit(1)
		}

		configDir := filepath.Join(home, ".joblin")
		viper.AddConfigPath(configDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("JOBLIN")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		if globalFlags.Verbose {
			fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
		}
	}
}

func initializeCLI() error {
	// Create configuration options from flags
	options := &lib.ConfigOptions{
		ConfigFile: globalFlags.ConfigFile,
		Context:    globalFlags.Context,
		Namespace:  globalFlags.Namespace,
		LogLevel:   globalFlags.LogLevel,
		Verbose:    globalFlags.Verbose,
	}

	// Initialize configuration manager
	var err error
	configMgr, err = lib.NewConfigManager(options)
	if err != nil {
		return fmt.Errorf("failed to initialize configuration manager: %w", err)
	}

	// Initialize services
	cliContext, err = configMgr.InitializeServices()
	if err != nil {
		return fmt.Errorf("failed to initialize services: %w", err)
	}

	// Initialize error handler
	errorHandler = lib.NewCLIErrorHandler(cliContext.Logger, globalFlags.JSONOutput)

	// Log configuration info if verbose
	if globalFlags.Verbose {
		info := configMgr.GetKubeConfigInfo()
		cliContext.Logger.WithContext(context.Background()).
			WithFields(info).Debug("Configuration loaded successfully")
	}

	return nil
}

func cleanupCLI() error {
	if configMgr != nil {
		return configMgr.Close(cliContext)
	}
	return nil
}

// Helper functions for commands

func GetEffectiveNamespace() string {
	if configMgr != nil {
		return configMgr.GetEffectiveNamespace(globalFlags.Namespace)
	}
	return globalFlags.Namespace
}

func GetEffectiveContext() string {
	if configMgr != nil {
		return configMgr.GetEffectiveContext()
	}
	return globalFlags.Context
}

func PrintJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func PrintError(err error) {
	if errorHandler != nil {
		errorHandler.HandleError(GetContext(), err, "unknown")
	} else {
		// Fallback if error handler not initialized
		if globalFlags.JSONOutput {
			errorData := map[string]interface{}{
				"error":   err.Error(),
				"success": false,
			}
			PrintJSON(errorData)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}

func PrintSuccess(message string, data interface{}) {
	if errorHandler != nil {
		errorHandler.HandleSuccess(GetContext(), message, data)
	} else {
		// Fallback if error handler not initialized
		if globalFlags.JSONOutput {
			result := map[string]interface{}{
				"success": true,
				"message": message,
			}
			if data != nil {
				result["data"] = data
			}
			PrintJSON(result)
		} else {
			fmt.Println(message)
			if data != nil && globalFlags.Verbose {
				fmt.Printf("Details: %+v\n", data)
			}
		}
	}
}

func GetContext() context.Context {
	if configMgr != nil {
		return configMgr.GetContext()
	}
	return context.Background()
}

func ValidateJobID(jobID string) error {
	if errorHandler != nil {
		return errorHandler.RequireJobID(jobID)
	}
	if jobID == "" {
		return fmt.Errorf("job ID is required")
	}
	return nil
}

func GetErrorHandler() *lib.CLIErrorHandler {
	return errorHandler
}

func CreateOperationContext(operation string) context.Context {
	return lib.WithOperationContext(GetContext(), operation)
}

func CreateJobContext(jobID, jobName, namespace string) context.Context {
	return lib.WithJobContext(GetContext(), jobID, jobName, namespace)
}