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
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type ConfigManager struct {
	cliConfig       *models.CLIConfig
	kubeConfigPath  string
	currentContext  string
	logger          *logrus.Logger
}

type ServiceContainer struct {
	Config           *models.CLIConfig
	Storage          *storage.StorageService
	K8sService       *k8slib.K8sService
	Notification     *notifylib.NotificationService
	JobService       *joblib.JobService
	Logger           *logrus.Logger
}

type ConfigOptions struct {
	ConfigFile     string
	KubeConfig     string
	Context        string
	Namespace      string
	LogLevel       string
	Verbose        bool
}

func NewConfigManager(options *ConfigOptions) (*ConfigManager, error) {
	// Initialize logger first
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: false,
		FullTimestamp:    true,
	})

	// Set log level
	logLevel := determineLogLevel(options)
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}
	logger.SetLevel(level)

	// Load CLI configuration
	cliConfig, err := loadCLIConfig(options.ConfigFile, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load CLI configuration: %w", err)
	}

	// Override with command-line options
	applyConfigOverrides(cliConfig, options)

	// Determine kubeconfig path
	kubeConfigPath := determineKubeConfigPath(options.KubeConfig)

	// Validate kubeconfig and context
	currentContext, err := validateKubeConfig(kubeConfigPath, options.Context, logger)
	if err != nil {
		return nil, fmt.Errorf("kubeconfig validation failed: %w", err)
	}

	return &ConfigManager{
		cliConfig:      cliConfig,
		kubeConfigPath: kubeConfigPath,
		currentContext: currentContext,
		logger:         logger,
	}, nil
}

func (cm *ConfigManager) InitializeServices() (*ServiceContainer, error) {
	cm.logger.Debug("Initializing services...")

	// Ensure data directory exists
	if err := cm.cliConfig.EnsureDataDirectory(); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Initialize storage service
	storageService, err := storage.NewStorageService(cm.cliConfig.GetDatabasePath())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage service: %w", err)
	}

	// Initialize Kubernetes service
	k8sService, err := k8slib.NewK8sService(cm.kubeConfigPath, cm.currentContext)
	if err != nil {
		// Try in-cluster config if external config fails
		cm.logger.Debug("External kubeconfig failed, trying in-cluster config")
		k8sService, err = k8slib.NewK8sServiceFromCluster()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Kubernetes service: %w", err)
		}
	}

	// Initialize notification service
	notificationService := notifylib.NewNotificationService(cm.logger)

	// Initialize job service (orchestrates all other services)
	jobService := joblib.NewJobService(storageService, k8sService, notificationService, cm.logger)

	// Test connections
	if err := cm.testConnections(k8sService); err != nil {
		cm.logger.Warnf("Service connection test failed: %v", err)
	}

	cm.logger.Info("All services initialized successfully")

	return &ServiceContainer{
		Config:       cm.cliConfig,
		Storage:      storageService,
		K8sService:   k8sService,
		Notification: notificationService,
		JobService:   jobService,
		Logger:       cm.logger,
	}, nil
}

func (cm *ConfigManager) GetEffectiveNamespace(override string) string {
	if override != "" {
		return override
	}
	return cm.cliConfig.DefaultNamespace
}

func (cm *ConfigManager) GetEffectiveContext() string {
	return cm.currentContext
}

func (cm *ConfigManager) GetConfig() *models.CLIConfig {
	return cm.cliConfig
}

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

func loadCLIConfig(configFile string, logger *logrus.Logger) (*models.CLIConfig, error) {
	var config *models.CLIConfig
	var err error

	if configFile != "" {
		logger.Debugf("Loading configuration from: %s", configFile)
		config, err = models.LoadCLIConfig(configFile)
	} else {
		logger.Debug("Loading default configuration")
		config, err = models.LoadDefaultCLIConfig()
	}

	if err != nil {
		return nil, err
	}

	logger.Debugf("Configuration loaded from: %s", config.GetConfigPath())
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

func validateKubeConfig(kubeConfigPath, contextOverride string, logger *logrus.Logger) (string, error) {
	if kubeConfigPath == "" {
		logger.Debug("No kubeconfig path specified, will try in-cluster config")
		return "", nil
	}

	// Check if kubeconfig file exists
	if _, err := os.Stat(kubeConfigPath); os.IsNotExist(err) {
		return "", fmt.Errorf("kubeconfig file not found: %s", kubeConfigPath)
	}

	// Load kubeconfig to validate and get context
	config, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Determine effective context
	effectiveContext := contextOverride
	if effectiveContext == "" {
		effectiveContext = config.CurrentContext
	}

	// Validate context exists
	if effectiveContext != "" {
		if _, exists := config.Contexts[effectiveContext]; !exists {
			return "", fmt.Errorf("context '%s' not found in kubeconfig", effectiveContext)
		}
	}

	logger.Debugf("Using kubeconfig: %s, context: %s", kubeConfigPath, effectiveContext)
	return effectiveContext, nil
}

func (cm *ConfigManager) testConnections(k8sService *k8slib.K8sService) error {
	// Test Kubernetes connection
	ctx := context.Background()
	if err := k8sService.TestConnection(ctx); err != nil {
		return fmt.Errorf("Kubernetes connection test failed: %w", err)
	}

	cm.logger.Debug("Kubernetes connection test successful")
	return nil
}

func (cm *ConfigManager) GetContext() context.Context {
	return context.Background()
}

func (cm *ConfigManager) GetKubeConfigInfo() map[string]interface{} {
	return map[string]interface{}{
		"kubeconfig_path": cm.kubeConfigPath,
		"current_context": cm.currentContext,
		"namespace":      cm.cliConfig.DefaultNamespace,
		"data_dir":       cm.cliConfig.DataDir,
	}
}