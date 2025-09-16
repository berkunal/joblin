package models

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultDataDirName = ".joblin"
)

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

func (c *CLIConfig) GetConfigPath() string {
	return filepath.Join(c.DataDir, "config.yaml")
}

func (c *CLIConfig) GetDatabasePath() string {
	return filepath.Join(c.DataDir, "joblin.db")
}

func (c *CLIConfig) EnsureDataDirectory() error {
	if err := os.MkdirAll(c.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory %s: %w", c.DataDir, err)
	}
	return nil
}

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

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configPath, err)
	}

	return nil
}

func LoadCLIConfig(configPath string) (*CLIConfig, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config if file doesn't exist
		config := NewCLIConfig()
		if err := config.Save(); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return config, nil
	}

	data, err := os.ReadFile(configPath)
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

func LoadDefaultCLIConfig() (*CLIConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, DefaultDataDirName, "config.yaml")
	return LoadCLIConfig(configPath)
}

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

func (c *CLIConfig) UpdateDefaultResources(resources ResourceSpec) error {
	if err := resources.Validate(); err != nil {
		return fmt.Errorf("invalid resource specification: %w", err)
	}

	c.DefaultResources = resources
	c.LastUpdated = time.Now().UTC()
	return nil
}

func (c *CLIConfig) UpdateLogLevel(level string) error {
	if !isValidLogLevel(level) {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", level)
	}

	c.LogLevel = level
	c.LastUpdated = time.Now().UTC()
	return nil
}

func (c *CLIConfig) GetEffectiveNamespace(override string) string {
	if override != "" {
		return override
	}
	return c.DefaultNamespace
}

func (c *CLIConfig) GetEffectiveResources(override *ResourceSpec) ResourceSpec {
	if override != nil {
		return *override
	}
	return c.DefaultResources
}

func (c *CLIConfig) GetEffectiveTTL(override time.Duration) time.Duration {
	if override > 0 {
		return override
	}
	return c.DefaultTTL
}

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

func GetValidLogLevels() []string {
	return []string{"debug", "info", "warn", "error"}
}
