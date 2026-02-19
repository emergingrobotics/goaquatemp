package goaquatemp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigFromEnvAllVars(t *testing.T) {
	envVars := map[string]string{
		"AQUATEMP_USERNAME":       "testuser",
		"AQUATEMP_PASSWORD":       "testpass",
		"AQUATEMP_API_VERSION":    "legacy",
		"AQUATEMP_BASE_URL":       "https://custom.example.com",
		"AQUATEMP_TIMEOUT":        "60s",
		"AQUATEMP_DEBUG":          "true",
		"AQUATEMP_DEFAULT_DEVICE": "DEV001",
		"AQUATEMP_CONFIG":         "/custom/config.yaml",
	}

	for k, v := range envVars {
		os.Setenv(k, v)
	}
	defer func() {
		for k := range envVars {
			os.Unsetenv(k)
		}
	}()

	config := LoadConfigFromEnv()

	if config.Username != "testuser" {
		t.Errorf("Username = %q, want %q", config.Username, "testuser")
	}
	if config.Password != "testpass" {
		t.Errorf("Password = %q, want %q", config.Password, "testpass")
	}
	if config.APIVersion != APILegacy {
		t.Errorf("APIVersion = %q, want %q", config.APIVersion, APILegacy)
	}
	if config.BaseURL != "https://custom.example.com" {
		t.Errorf("BaseURL = %q, want %q", config.BaseURL, "https://custom.example.com")
	}
	if config.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want %v", config.Timeout, 60*time.Second)
	}
	if !config.Debug {
		t.Error("Debug = false, want true")
	}
	if config.DefaultDevice != "DEV001" {
		t.Errorf("DefaultDevice = %q, want %q", config.DefaultDevice, "DEV001")
	}
	if config.ConfigPath != "/custom/config.yaml" {
		t.Errorf("ConfigPath = %q, want %q", config.ConfigPath, "/custom/config.yaml")
	}
}

func TestLoadConfigFromEnvPartialVars(t *testing.T) {
	os.Setenv("AQUATEMP_USERNAME", "onlyuser")
	defer os.Unsetenv("AQUATEMP_USERNAME")

	config := LoadConfigFromEnv()

	if config.Username != "onlyuser" {
		t.Errorf("Username = %q, want %q", config.Username, "onlyuser")
	}
	if config.APIVersion != APIModern {
		t.Errorf("APIVersion = %q, want %q (default)", config.APIVersion, APIModern)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v (default)", config.Timeout, 30*time.Second)
	}
}

func TestLoadConfigFromEnvInvalidTimeout(t *testing.T) {
	os.Setenv("AQUATEMP_TIMEOUT", "notaduration")
	defer os.Unsetenv("AQUATEMP_TIMEOUT")

	config := LoadConfigFromEnv()

	if config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v (default on invalid input)", config.Timeout, 30*time.Second)
	}
}

func TestLoadConfigFileValidYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	content := `username: "fileuser"
password: "filepass"
api_version: "hitemp"
base_url: "https://file.example.com"
timeout: 45s
retry_count: 5
debug: true
default_device: "FILEDEV"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFile() error = %v", err)
	}

	if config.Username != "fileuser" {
		t.Errorf("Username = %q, want %q", config.Username, "fileuser")
	}
	if config.Password != "filepass" {
		t.Errorf("Password = %q, want %q", config.Password, "filepass")
	}
	if config.APIVersion != APIHiTemp {
		t.Errorf("APIVersion = %q, want %q", config.APIVersion, APIHiTemp)
	}
	if config.BaseURL != "https://file.example.com" {
		t.Errorf("BaseURL = %q, want %q", config.BaseURL, "https://file.example.com")
	}
	if config.RetryCount != 5 {
		t.Errorf("RetryCount = %d, want %d", config.RetryCount, 5)
	}
	if !config.Debug {
		t.Error("Debug = false, want true")
	}
	if config.DefaultDevice != "FILEDEV" {
		t.Errorf("DefaultDevice = %q, want %q", config.DefaultDevice, "FILEDEV")
	}
}

func TestLoadConfigFileMissingFile(t *testing.T) {
	_, err := LoadConfigFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("LoadConfigFile() expected error for missing file, got nil")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if path == "" {
		t.Skip("could not determine home directory")
	}

	if filepath.Base(path) != "config.yaml" {
		t.Errorf("DefaultConfigPath() basename = %q, want %q", filepath.Base(path), "config.yaml")
	}
	if filepath.Base(filepath.Dir(path)) != "aquatemp" {
		t.Errorf("DefaultConfigPath() parent dir = %q, want %q", filepath.Base(filepath.Dir(path)), "aquatemp")
	}
}
