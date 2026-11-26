package goaquatemp

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// APIVersion represents the version of the Aqua Temp API to use.
type APIVersion string

const (
	// APILegacy is for accounts created before app v1.5.8.
	APILegacy APIVersion = "legacy"
	// APIHiTemp is for HiTemp branded devices (same endpoint as modern).
	APIHiTemp APIVersion = "hitemp"
	// APIModern is for accounts created with app v1.5.9+ (default).
	APIModern APIVersion = "modern"
)

// API base URLs for different versions.
const (
	BaseURLLegacy = "https://cloud.linked-go.com/cloudservice/api"
	BaseURLHiTemp = "https://cloud.linked-go.com:449/crmservice/api"
	BaseURLModern = "https://cloud.linked-go.com:449/crmservice/api"
)

// Config holds the configuration for the Aqua Temp client.
type Config struct {
	Username      string        `yaml:"username"`
	Password      string        `yaml:"password"`
	APIVersion    APIVersion    `yaml:"api_version"`
	BaseURL       string        `yaml:"base_url"`
	Timeout       time.Duration `yaml:"timeout"`
	RetryCount    int           `yaml:"retry_count"`
	Debug         bool          `yaml:"debug"`
	DefaultDevice string        `yaml:"default_device"`
	ConfigPath    string        `yaml:"-"`
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		APIVersion: APIModern,
		Timeout:    30 * time.Second,
		RetryCount: 3,
		Debug:      false,
	}
}

// GetBaseURL returns the base URL for the configured API version.
func (c *Config) GetBaseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}

	switch c.APIVersion {
	case APILegacy:
		return BaseURLLegacy
	case APIHiTemp:
		return BaseURLHiTemp
	case APIModern:
		return BaseURLModern
	default:
		return BaseURLModern
	}
}

// LoadConfigFile loads configuration from a YAML file.
func LoadConfigFile(path string) (*Config, error) {
	// Expand ~ to home directory
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, path[1:])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// LoadConfigFromEnv loads configuration from environment variables.
func LoadConfigFromEnv() *Config {
	config := DefaultConfig()

	if v := os.Getenv("AQUATEMP_USERNAME"); v != "" {
		config.Username = v
	}
	if v := os.Getenv("AQUATEMP_PASSWORD"); v != "" {
		config.Password = v
	}
	if v := os.Getenv("AQUATEMP_API_VERSION"); v != "" {
		config.APIVersion = APIVersion(v)
	}
	if v := os.Getenv("AQUATEMP_BASE_URL"); v != "" {
		config.BaseURL = v
	}
	if v := os.Getenv("AQUATEMP_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			config.Timeout = d
		}
	}
	if v := os.Getenv("AQUATEMP_DEBUG"); v != "" {
		config.Debug, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("AQUATEMP_DEFAULT_DEVICE"); v != "" {
		config.DefaultDevice = v
	}
	if v := os.Getenv("AQUATEMP_CONFIG"); v != "" {
		config.ConfigPath = v
	}

	return config
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "aquatemp", "config.yaml")
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.Username == "" {
		return &ValidationError{Field: "username", Value: "", Message: "username is required"}
	}
	if c.Password == "" {
		return &ValidationError{Field: "password", Value: "", Message: "password is required"}
	}
	return nil
}
