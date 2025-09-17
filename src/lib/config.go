package lib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/berkunal/joblin/src/models"
	"github.com/berkunal/joblin/src/services/joblib"
	"github.com/berkunal/joblin/src/services/k8slib"
	"github.com/berkunal/joblin/src/services/notifylib"
	"github.com/berkunal/joblin/src/services/storage"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// ConfigManager manages CLI configuration and service initialization
type ConfigManager struct {
	cliConfig      *models.CLIConfig
	kubeConfigPath string
	currentContext string
	logger         *Logger
	services       *ServiceContainer
}

// ServiceContainer holds all initialized services for the application
type ServiceContainer struct {
	Config       *models.CLIConfig
	Storage      *storage.Service
	K8sService   *k8slib.K8sService
	Notification *notifylib.NotificationService
	JobService   *joblib.JobService
	Logger       *Logger
}

// ConfigOptions holds command-line configuration options
type ConfigOptions struct {
	ConfigFile string
	KubeConfig string
	Context    string
	Namespace  string
	LogLevel   string
	Verbose    bool
}

// NewConfigManager creates a new configuration manager with the provided options
func NewConfigManager(options *ConfigOptions) (*ConfigManager, error) {
	// Initialize structured logger
	logger := NewConsoleLogger("config-manager")

	// Set log level
	logLevel := determineLogLevel(options)
	if err := logger.SetLevel(logLevel); err != nil {
		return nil, NewConfigurationError("INVALID_LOG_LEVEL",
			fmt.Sprintf("Invalid log level: %s", logLevel)).WithCause(err)
	}

	// Load CLI configuration
	cliConfig, err := loadCLIConfig(options.ConfigFile, logger)
	if err != nil {
		return nil, NewConfigurationError("CONFIG_LOAD_FAILED",
			"Failed to load CLI configuration").WithCause(err).WithContext("config_file", options.ConfigFile)
	}

	// Override with command-line options
	applyConfigOverrides(cliConfig, options)

	// Determine kubeconfig path
	kubeConfigPath := determineKubeConfigPath(options.KubeConfig)

	// Validate kubeconfig and context
	currentContext, err := validateKubeConfig(kubeConfigPath, options.Context, logger)
	if err != nil {
		return nil, NewKubernetesError("KUBECONFIG_INVALID",
			"Kubeconfig validation failed").WithCause(err).
			WithContext("kubeconfig_path", kubeConfigPath).
			WithContext("context", options.Context)
	}

	return &ConfigManager{
		cliConfig:      cliConfig,
		kubeConfigPath: kubeConfigPath,
		currentContext: currentContext,
		logger:         logger,
	}, nil
}

// InitializeServices initializes all application services
func (cm *ConfigManager) InitializeServices() (*ServiceContainer, error) {
	ctx := WithOperationContext(context.Background(), "initialize-services")

	err := cm.logger.LogOperation(ctx, "service-initialization", func() error {
		// Ensure data directory exists
		if err := cm.cliConfig.EnsureDataDirectory(); err != nil {
			return NewStorageError("DATA_DIR_CREATE_FAILED",
				"Failed to create data directory").WithCause(err).
				WithContext("data_dir", cm.cliConfig.DataDir)
		}

		// Initialize storage service
		storageService, err := storage.NewService(cm.cliConfig.GetDatabasePath())
		if err != nil {
			return NewStorageError("STORAGE_INIT_FAILED",
				"Failed to initialize storage service").WithCause(err).
				WithContext("database_path", cm.cliConfig.GetDatabasePath())
		}

		// Initialize Kubernetes service
		k8sService, err := k8slib.NewK8sService(cm.kubeConfigPath, cm.currentContext)
		if err != nil {
			// Try in-cluster config if external config fails
			cm.logger.WithContext(ctx).Debug("External kubeconfig failed, trying in-cluster config")
			k8sService, err = k8slib.NewK8sServiceFromCluster()
			if err != nil {
				return NewKubernetesError("K8S_INIT_FAILED",
					"Failed to initialize Kubernetes service").WithCause(err).
					WithContext("kubeconfig_path", cm.kubeConfigPath).
					WithContext("context", cm.currentContext)
			}
		}

		// Initialize notification service with proper logger
		logrusLogger := cm.logger.Logger // Extract underlying logrus logger
		notificationService := notifylib.NewNotificationService(logrusLogger)

		// Initialize job service (orchestrates all other services)
		jobService := joblib.NewJobService(storageService, k8sService, notificationService, logrusLogger)

		// Test connections
		if err := cm.testConnections(k8sService); err != nil {
			cm.logger.WithContext(ctx).WithError(err).Warn("Service connection test failed")
		}

		// Store services in container
		cm.services = &ServiceContainer{
			Config:       cm.cliConfig,
			Storage:      storageService,
			K8sService:   k8sService,
			Notification: notificationService,
			JobService:   jobService,
			Logger:       cm.logger,
		}

		cm.logger.WithContext(ctx).Info("All services initialized successfully")
		return nil
	})

	if err != nil {
		return nil, err
	}

	if cm.services == nil {
		return nil, NewInternalError("SERVICE_INIT_INCOMPLETE", "Service initialization did not complete properly")
	}

	return cm.services, nil
}

// GetEffectiveNamespace returns the namespace to use, preferring override over default
func (cm *ConfigManager) GetEffectiveNamespace(override string) string {
	if override != "" {
		return override
	}
	return cm.cliConfig.DefaultNamespace
}

// GetEffectiveContext returns the current Kubernetes context
func (cm *ConfigManager) GetEffectiveContext() string {
	return cm.currentContext
}

// GetConfig returns the CLI configuration
func (cm *ConfigManager) GetConfig() *models.CLIConfig {
	return cm.cliConfig
}

// Close cleanly shuts down all services
func (cm *ConfigManager) Close(services *ServiceContainer) error {
	if services != nil && services.JobService != nil {
		return services.JobService.Close()
	}
	return nil
}

func determineLogLevel(options *ConfigOptions) string {
	if options.Verbose {
		return "debug"
	}
	if options.LogLevel != "" {
		return options.LogLevel
	}
	return "info"
}

func loadCLIConfig(configFile string, logger *Logger) (*models.CLIConfig, error) {
	var config *models.CLIConfig
	var err error

	ctx := context.Background()

	if configFile != "" {
		logger.WithContext(ctx).WithField("config_file", configFile).Debug("Loading configuration from file")
		config, err = models.LoadCLIConfig(configFile)
	} else {
		logger.WithContext(ctx).Debug("Loading default configuration")
		config, err = models.LoadDefaultCLIConfig()
	}

	if err != nil {
		return nil, err
	}

	logger.WithContext(ctx).WithField("config_path", config.GetConfigPath()).Debug("Configuration loaded successfully")
	return config, nil
}

func applyConfigOverrides(config *models.CLIConfig, options *ConfigOptions) {
	if options.Context != "" {
		config.DefaultCluster = options.Context
	}

	if options.Namespace != "" {
		config.DefaultNamespace = options.Namespace
	}

	if options.LogLevel != "" {
		config.LogLevel = options.LogLevel
	} else if options.Verbose {
		config.LogLevel = "debug"
	}
}

func determineKubeConfigPath(kubeConfigFlag string) string {
	if kubeConfigFlag != "" {
		return kubeConfigFlag
	}

	// Check KUBECONFIG environment variable
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return kubeconfig
	}

	// Use default location
	if home := homedir.HomeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}

	return ""
}

func validateKubeConfig(kubeConfigPath, contextOverride string, logger *Logger) (string, error) {
	ctx := context.Background()

	if kubeConfigPath == "" {
		logger.WithContext(ctx).Debug("No kubeconfig path specified, will try in-cluster config")
		return "", nil
	}

	// Check if kubeconfig file exists
	if _, err := os.Stat(kubeConfigPath); os.IsNotExist(err) {
		return "", NewKubernetesError("KUBECONFIG_NOT_FOUND",
			"Kubeconfig file not found").WithContext("kubeconfig_path", kubeConfigPath)
	}

	// Load kubeconfig to validate and get context
	config, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		return "", NewKubernetesError("KUBECONFIG_LOAD_FAILED",
			"Failed to load kubeconfig").WithCause(err).WithContext("kubeconfig_path", kubeConfigPath)
	}

	// Determine effective context
	effectiveContext := contextOverride
	if effectiveContext == "" {
		effectiveContext = config.CurrentContext
	}

	// Validate context exists
	if effectiveContext != "" {
		if _, exists := config.Contexts[effectiveContext]; !exists {
			return "", NewKubernetesError("CONTEXT_NOT_FOUND",
				"Kubernetes context not found in kubeconfig").
				WithContext("context", effectiveContext).
				WithContext("kubeconfig_path", kubeConfigPath)
		}
	}

	logger.WithContext(ctx).WithFields(map[string]interface{}{
		"kubeconfig_path": kubeConfigPath,
		"context":         effectiveContext,
	}).Debug("Kubeconfig validated successfully")
	return effectiveContext, nil
}

func (cm *ConfigManager) testConnections(k8sService *k8slib.K8sService) error {
	// Test Kubernetes connection
	ctx := WithOperationContext(context.Background(), "connection-test")

	if err := k8sService.TestConnection(ctx); err != nil {
		return NewKubernetesError("CONNECTION_TEST_FAILED",
			"Kubernetes connection test failed").WithCause(err).
			WithContext("kubeconfig_path", cm.kubeConfigPath).
			WithContext("context", cm.currentContext)
	}

	cm.logger.WithContext(ctx).Debug("Kubernetes connection test successful")
	return nil
}

// GetContext returns a base context for operations
func (cm *ConfigManager) GetContext() context.Context {
	return context.Background()
}

// GetKubeConfigInfo returns information about the current Kubernetes configuration
func (cm *ConfigManager) GetKubeConfigInfo() map[string]interface{} {
	return map[string]interface{}{
		"kubeconfig_path": cm.kubeConfigPath,
		"current_context": cm.currentContext,
		"namespace":       cm.cliConfig.DefaultNamespace,
		"data_dir":        cm.cliConfig.DataDir,
	}
}
