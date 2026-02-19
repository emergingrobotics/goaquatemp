package goaquatemp

import "time"

// Option is a functional option for configuring a Client.
type Option func(*Config)

// WithUsername sets the username for authentication.
func WithUsername(username string) Option {
	return func(c *Config) {
		c.Username = username
	}
}

// WithPassword sets the password for authentication.
func WithPassword(password string) Option {
	return func(c *Config) {
		c.Password = password
	}
}

// WithAPIVersion sets the API version to use.
func WithAPIVersion(version APIVersion) Option {
	return func(c *Config) {
		c.APIVersion = version
	}
}

// WithBaseURL overrides the default base URL for the API.
func WithBaseURL(url string) Option {
	return func(c *Config) {
		c.BaseURL = url
	}
}

// WithTimeout sets the HTTP request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

// WithRetryCount sets the number of retry attempts for failed requests.
func WithRetryCount(count int) Option {
	return func(c *Config) {
		c.RetryCount = count
	}
}

// WithDebug enables debug logging.
func WithDebug(debug bool) Option {
	return func(c *Config) {
		c.Debug = debug
	}
}

// WithDefaultDevice sets the default device code.
func WithDefaultDevice(deviceCode string) Option {
	return func(c *Config) {
		c.DefaultDevice = deviceCode
	}
}

// WithConfig applies an existing Config.
func WithConfig(cfg *Config) Option {
	return func(c *Config) {
		if cfg.Username != "" {
			c.Username = cfg.Username
		}
		if cfg.Password != "" {
			c.Password = cfg.Password
		}
		if cfg.APIVersion != "" {
			c.APIVersion = cfg.APIVersion
		}
		if cfg.BaseURL != "" {
			c.BaseURL = cfg.BaseURL
		}
		if cfg.Timeout > 0 {
			c.Timeout = cfg.Timeout
		}
		if cfg.RetryCount > 0 {
			c.RetryCount = cfg.RetryCount
		}
		c.Debug = cfg.Debug
		if cfg.DefaultDevice != "" {
			c.DefaultDevice = cfg.DefaultDevice
		}
	}
}

// WithConfigFile loads configuration from a file and applies it.
func WithConfigFile(path string) Option {
	return func(c *Config) {
		cfg, err := LoadConfigFile(path)
		if err != nil {
			return // Silently ignore file load errors
		}
		WithConfig(cfg)(c)
	}
}

// WithEnvConfig loads configuration from environment variables.
func WithEnvConfig() Option {
	return func(c *Config) {
		cfg := LoadConfigFromEnv()
		WithConfig(cfg)(c)
	}
}
