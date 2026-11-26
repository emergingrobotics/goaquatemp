package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/user/goaquatemp"
)

func TestVersionCmd(t *testing.T) {
	cmd := newVersionCmd()

	if cmd.Use != "version" {
		t.Errorf("Use = %q, want %q", cmd.Use, "version")
	}

	if cmd.Short != "Show version information" {
		t.Errorf("Short = %q, want %q", cmd.Short, "Show version information")
	}
}

func TestVersionCmdExecution(t *testing.T) {
	// Save original values
	origVersion := version
	origCommit := commit
	origDate := date

	// Set test values
	version = "1.0.0"
	commit = "abc123"
	date = "2024-01-01"

	defer func() {
		version = origVersion
		commit = origCommit
		date = origDate
	}()

	cmd := newVersionCmd()

	// Capture output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute command
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestDevicesCmd(t *testing.T) {
	cmd := newDevicesCmd()

	if cmd.Use != "devices" {
		t.Errorf("Use = %q, want %q", cmd.Use, "devices")
	}

	// Should have list and info subcommands
	subcommands := cmd.Commands()
	if len(subcommands) != 2 {
		t.Errorf("subcommand count = %d, want %d", len(subcommands), 2)
	}

	hasListCmd := false
	hasInfoCmd := false
	for _, sub := range subcommands {
		if sub.Use == "list" {
			hasListCmd = true
		}
		if sub.Use == "info" {
			hasInfoCmd = true
		}
	}

	if !hasListCmd {
		t.Error("missing 'list' subcommand")
	}
	if !hasInfoCmd {
		t.Error("missing 'info' subcommand")
	}
}

func TestLoginCmd(t *testing.T) {
	cmd := newLoginCmd()

	if cmd.Use != "login" {
		t.Errorf("Use = %q, want %q", cmd.Use, "login")
	}

	if cmd.Short != "Authenticate with the Aqua Temp API" {
		t.Errorf("Short = %q, want %q", cmd.Short, "Authenticate with the Aqua Temp API")
	}
}

func TestTempCmd(t *testing.T) {
	cmd := newTempCmd()

	if cmd.Use != "temp" {
		t.Errorf("Use = %q, want %q", cmd.Use, "temp")
	}

	subcommands := cmd.Commands()
	expected := map[string]bool{
		"get":     false,
		"history": false,
	}

	for _, sub := range subcommands {
		if _, ok := expected[sub.Use]; ok {
			expected[sub.Use] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("missing '%s' subcommand", name)
		}
	}
}

func TestTempHistoryFlags(t *testing.T) {
	cmd := newTempCmd()

	var historyCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "history" {
			historyCmd = sub
			break
		}
	}

	if historyCmd == nil {
		t.Fatal("missing 'history' subcommand")
	}

	hoursFlag := historyCmd.Flags().Lookup("hours")
	if hoursFlag == nil {
		t.Error("missing --hours flag on history subcommand")
	}

	intervalFlag := historyCmd.Flags().Lookup("interval")
	if intervalFlag == nil {
		t.Error("missing --interval flag on history subcommand")
	}
}

func TestStatusCmd(t *testing.T) {
	cmd := newStatusCmd()

	if cmd.Use != "status" {
		t.Errorf("Use = %q, want %q", cmd.Use, "status")
	}
}

func TestControlCmd(t *testing.T) {
	cmd := newControlCmd()

	if cmd.Use != "control" {
		t.Errorf("Use = %q, want %q", cmd.Use, "control")
	}

	subcommands := cmd.Commands()
	if len(subcommands) != 4 {
		t.Errorf("subcommand count = %d, want %d", len(subcommands), 4)
	}

	expected := map[string]bool{
		"power":     false,
		"set-temp":  false,
		"mode":      false,
		"fan-speed": false,
	}

	for _, sub := range subcommands {
		if _, ok := expected[sub.Use]; ok {
			expected[sub.Use] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("missing '%s' subcommand", name)
		}
	}
}

func TestTempStr(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{25.0, "25.0"},
		{25.5, "25.5"},
		{25.55, "25.6"},
		{0.0, "0.0"},
		{-5.5, "-5.5"},
		{100.123, "100.1"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tempStr(tt.input)
			if got != tt.expected {
				t.Errorf("tempStr(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGetDeviceCode(t *testing.T) {
	// Save original value
	origDeviceCode := deviceCode
	defer func() {
		deviceCode = origDeviceCode
	}()

	tests := []struct {
		name           string
		flagValue      string
		envValue       string
		configDefault  string
		expectedResult string
	}{
		{
			name:           "flag takes precedence",
			flagValue:      "FLAG_CODE",
			envValue:       "ENV_CODE",
			configDefault:  "CONFIG_CODE",
			expectedResult: "FLAG_CODE",
		},
		{
			name:           "env used when flag empty",
			flagValue:      "",
			envValue:       "ENV_CODE",
			configDefault:  "CONFIG_CODE",
			expectedResult: "ENV_CODE",
		},
		{
			name:           "config used when flag and env empty",
			flagValue:      "",
			envValue:       "",
			configDefault:  "CONFIG_CODE",
			expectedResult: "CONFIG_CODE",
		},
		{
			name:           "empty when all sources empty",
			flagValue:      "",
			envValue:       "",
			configDefault:  "",
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deviceCode = tt.flagValue
			os.Setenv("AQUATEMP_DEFAULT_DEVICE", tt.envValue)
			defer os.Unsetenv("AQUATEMP_DEFAULT_DEVICE")

			result := getDeviceCode(&mockClient{defaultDevice: tt.configDefault})
			if result != tt.expectedResult {
				t.Errorf("getDeviceCode() = %q, want %q", result, tt.expectedResult)
			}
		})
	}
}

func TestRequireDeviceCode(t *testing.T) {
	// Save original value
	origDeviceCode := deviceCode
	defer func() {
		deviceCode = origDeviceCode
	}()

	// Test with device code set
	deviceCode = "TEST_CODE"
	code, err := requireDeviceCode(&mockClient{})
	if err != nil {
		t.Errorf("requireDeviceCode() unexpected error: %v", err)
	}
	if code != "TEST_CODE" {
		t.Errorf("requireDeviceCode() = %q, want %q", code, "TEST_CODE")
	}

	// Test with no device code
	deviceCode = ""
	os.Unsetenv("AQUATEMP_DEFAULT_DEVICE")
	_, err = requireDeviceCode(&mockClient{})
	if err == nil {
		t.Error("requireDeviceCode() expected error when no device code, got nil")
	}
}

func TestRootCmdFlags(t *testing.T) {
	rootCmd := &cobra.Command{
		Use: "aquatemp",
	}

	rootCmd.PersistentFlags().StringVarP(&deviceCode, "device", "d", "", "Device code")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&outputCSV, "csv", false, "Output in CSV format")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug output")

	// Check flags exist
	deviceFlag := rootCmd.PersistentFlags().Lookup("device")
	if deviceFlag == nil {
		t.Error("missing 'device' flag")
	}
	if deviceFlag.Shorthand != "d" {
		t.Errorf("device flag shorthand = %q, want %q", deviceFlag.Shorthand, "d")
	}

	jsonFlag := rootCmd.PersistentFlags().Lookup("json")
	if jsonFlag == nil {
		t.Error("missing 'json' flag")
	}

	csvFlag := rootCmd.PersistentFlags().Lookup("csv")
	if csvFlag == nil {
		t.Error("missing 'csv' flag")
	}

	debugFlag := rootCmd.PersistentFlags().Lookup("debug")
	if debugFlag == nil {
		t.Error("missing 'debug' flag")
	}
}

func TestControlFanSpeedFlags(t *testing.T) {
	cmd := newControlCmd()

	var fanSpeedCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Use == "fan-speed" {
			fanSpeedCmd = sub
			break
		}
	}

	if fanSpeedCmd == nil {
		t.Fatal("missing 'fan-speed' subcommand")
	}

	speedFlag := fanSpeedCmd.Flags().Lookup("speed")
	if speedFlag == nil {
		t.Error("missing --speed flag on fan-speed subcommand")
	}
}

// mockClient implements the goaquatemp.Client interface for testing
type mockClient struct {
	defaultDevice string
}

func (m *mockClient) Login(ctx context.Context) error                             { return nil }
func (m *mockClient) Logout(ctx context.Context) error                            { return nil }
func (m *mockClient) GetDevices(ctx context.Context) ([]goaquatemp.Device, error) { return nil, nil }
func (m *mockClient) GetDevice(ctx context.Context, deviceCode string) (*goaquatemp.Device, error) {
	return nil, nil
}
func (m *mockClient) GetDeviceData(ctx context.Context, deviceCode string) (*goaquatemp.DeviceData, error) {
	return nil, nil
}
func (m *mockClient) SetTemperature(ctx context.Context, deviceCode string, temp float64) error {
	return nil
}
func (m *mockClient) SetMode(ctx context.Context, deviceCode string, mode goaquatemp.OperationMode) error {
	return nil
}
func (m *mockClient) SetPower(ctx context.Context, deviceCode string, on bool) error { return nil }
func (m *mockClient) SetFanSpeed(ctx context.Context, deviceCode string, speed goaquatemp.FanSpeed) error {
	return nil
}
func (m *mockClient) Control(ctx context.Context, command goaquatemp.ControlCommand) error {
	return nil
}
func (m *mockClient) IsAuthenticated() bool { return false }
func (m *mockClient) GetDeviceStatus(ctx context.Context, deviceCode string) (*goaquatemp.DeviceStatus, error) {
	return nil, nil
}
func (m *mockClient) GetHistory(ctx context.Context, deviceCode string, q goaquatemp.HistoryQuery) (*goaquatemp.History, error) {
	return nil, nil
}
func (m *mockClient) GetParameters(ctx context.Context, deviceCode string, kind goaquatemp.ParameterKind) ([]goaquatemp.Parameter, error) {
	return nil, nil
}
func (m *mockClient) Config() *goaquatemp.Config {
	return &goaquatemp.Config{
		DefaultDevice: m.defaultDevice,
	}
}
