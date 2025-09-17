// Package models defines the core data structures used throughout the Joblin application.
// It includes configuration, job, notification, and resource specification models.
package models

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	yaml "gopkg.in/yaml.v3"
)

const (
	// DefaultDataDirName is the default directory name for Joblin data files
	DefaultDataDirName = ".joblin"
)

// CLIConfig holds the configuration settings for the Joblin CLI
type CLIConfig struct {
	DefaultCluster   string        `yaml:"default_cluster" json:"default_cluster"`
	DefaultNamespace string        `yaml:"default_namespace" json:"default_namespace"`
	DefaultResources ResourceSpec  `yaml:"default_resources" json:"default_resources"`
	DefaultTTL       time.Duration `yaml:"default_ttl" json:"default_ttl"`
	TeamsWebhookURL  string        `yaml:"teams_webhook_url" json:"teams_webhook_url,omitempty"`
	LogLevel         string        `yaml:"log_level" json:"log_level"`
	DataDir          string        `yaml:"data_dir" json:"data_dir"`
	ConfigVersion    string        `yaml:"config_version" json:"config_version"`
	LastUpdated      time.Time     `yaml:"last_updated" json:"last_updated"`
}

// NewCLIConfig creates a new CLI configuration with default values
func NewCLIConfig() *CLIConfig {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, DefaultDataDirName)

	return &CLIConfig{
		DefaultCluster:   "",
		DefaultNamespace: "default",
		DefaultResources: ResourceSpec{
			CPU:              "100m",
			Memory:           "128Mi",
			EphemeralStorage: "1Gi",
		},
		DefaultTTL:      24 * time.Hour,
		TeamsWebhookURL: "",
		LogLevel:        "info",
		DataDir:         dataDir,
		ConfigVersion:   "1.0",
		LastUpdated:     time.Now().UTC(),
	}
}

// Validate checks the CLIConfig for any invalid configuration values
func (c *CLIConfig) Validate() error {
	if c.DefaultNamespace == "" {
		return fmt.Errorf("default namespace cannot be empty")
	}

	if err := c.DefaultResources.Validate(); err != nil {
		return fmt.Errorf("invalid default resources: %w", err)
	}

	if c.DefaultTTL < time.Minute {
		return fmt.Errorf("default TTL must be at least 1 minute")
	}

	if c.DefaultTTL > 7*24*time.Hour {
		return fmt.Errorf("default TTL cannot exceed 7 days")
	}

	if c.TeamsWebhookURL != "" {
		if err := ValidateWebhookURL(c.TeamsWebhookURL); err != nil {
			return fmt.Errorf("invalid Teams webhook URL: %w", err)
		}
	}

	if !isValidLogLevel(c.LogLevel) {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.LogLevel)
	}

	if c.DataDir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}

	return nil
}

// SetDefaults sets default values for all configuration fields
func (c *CLIConfig) SetDefaults() {
	if c.DefaultNamespace == "" {
		c.DefaultNamespace = "default"
	}

	if c.DefaultResources.CPU == "" || c.DefaultResources.Memory == "" || c.DefaultResources.EphemeralStorage == "" {
		c.DefaultResources.SetDefaults()
	}

	if c.DefaultTTL == 0 {
		c.DefaultTTL = 24 * time.Hour
	}

	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	if c.DataDir == "" {
		homeDir, _ := os.UserHomeDir()
		c.DataDir = filepath.Join(homeDir, DefaultDataDirName)
	}

	if c.ConfigVersion == "" {
		c.ConfigVersion = "1.0"
	}

	c.LastUpdated = time.Now().UTC()
}

// GetConfigPath returns the full path to the configuration file
func (c *CLIConfig) GetConfigPath() string {
	return filepath.Join(c.DataDir, "config.yaml")
}

// GetDatabasePath returns the full path to the database file
func (c *CLIConfig) GetDatabasePath() string {
	return filepath.Join(c.DataDir, "joblin.db")
}

// EnsureDataDirectory creates the data directory if it doesn't exist
func (c *CLIConfig) EnsureDataDirectory() error {
	if err := os.MkdirAll(c.DataDir, 0750); err != nil {
		return fmt.Errorf("failed to create data directory %s: %w", c.DataDir, err)
	}
	return nil
}

// Save writes the configuration to the config file
func (c *CLIConfig) Save() error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	if err := c.EnsureDataDirectory(); err != nil {
		return err
	}

	c.LastUpdated = time.Now().UTC()

	configPath := c.GetConfigPath()
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configPath, err)
	}

	return nil
}

// LoadCLIConfig loads configuration from the specified path
func LoadCLIConfig(configPath string) (*CLIConfig, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config if file doesn't exist
		config := NewCLIConfig()
		if err := config.Save(); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return config, nil
	}

	data, err := os.ReadFile(filepath.Clean(configPath)) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config CLIConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Set defaults for any missing fields
	config.SetDefaults()

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration in %s: %w", configPath, err)
	}

	return &config, nil
}

// LoadDefaultCLIConfig loads configuration from the default location
func LoadDefaultCLIConfig() (*CLIConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, DefaultDataDirName, "config.yaml")
	return LoadCLIConfig(configPath)
}

// Clone creates a deep copy of the CLIConfig
func (c *CLIConfig) Clone() *CLIConfig {
	return &CLIConfig{
		DefaultCluster:   c.DefaultCluster,
		DefaultNamespace: c.DefaultNamespace,
		DefaultResources: c.DefaultResources.Clone(),
		DefaultTTL:       c.DefaultTTL,
		TeamsWebhookURL:  c.TeamsWebhookURL,
		LogLevel:         c.LogLevel,
		DataDir:          c.DataDir,
		ConfigVersion:    c.ConfigVersion,
		LastUpdated:      c.LastUpdated,
	}
}

// UpdateWebhookURL updates the webhook URL in the configuration and saves it
func (c *CLIConfig) UpdateWebhookURL(webhookURL string) error {
	if webhookURL != "" {
		if err := ValidateWebhookURL(webhookURL); err != nil {
			return fmt.Errorf("invalid webhook URL: %w", err)
		}
	}

	c.TeamsWebhookURL = webhookURL
	c.LastUpdated = time.Now().UTC()
	return nil
}

// UpdateDefaultResources updates the default resource limits in the configuration and saves it
func (c *CLIConfig) UpdateDefaultResources(resources ResourceSpec) error {
	if err := resources.Validate(); err != nil {
		return fmt.Errorf("invalid resource specification: %w", err)
	}

	c.DefaultResources = resources
	c.LastUpdated = time.Now().UTC()
	return nil
}

// UpdateLogLevel updates the log level in the configuration and saves it
func (c *CLIConfig) UpdateLogLevel(level string) error {
	if !isValidLogLevel(level) {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", level)
	}

	c.LogLevel = level
	c.LastUpdated = time.Now().UTC()
	return nil
}

// GetEffectiveNamespace returns the namespace to use, preferring override over default
func (c *CLIConfig) GetEffectiveNamespace(override string) string {
	if override != "" {
		return override
	}
	return c.DefaultNamespace
}

// GetEffectiveResources returns the resource spec to use, preferring override over default
func (c *CLIConfig) GetEffectiveResources(override *ResourceSpec) ResourceSpec {
	if override != nil {
		return *override
	}
	return c.DefaultResources
}

// GetEffectiveTTL returns the TTL to use, preferring override over default
func (c *CLIConfig) GetEffectiveTTL(override time.Duration) time.Duration {
	if override > 0 {
		return override
	}
	return c.DefaultTTL
}

// GetEffectiveWebhookURL returns the webhook URL to use, preferring override over default
func (c *CLIConfig) GetEffectiveWebhookURL(override string) string {
	if override != "" {
		return override
	}
	return c.TeamsWebhookURL
}

func isValidLogLevel(level string) bool {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	return validLevels[level]
}

// GetValidLogLevels returns a list of all valid log levels
func GetValidLogLevels() []string {
	return []string{"debug", "info", "warn", "error"}
}
