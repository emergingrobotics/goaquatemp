package goaquatemp

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.APIVersion != APIModern {
		t.Errorf("expected APIVersion=%s, got %s", APIModern, config.APIVersion)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("expected Timeout=30s, got %s", config.Timeout)
	}
	if config.RetryCount != 3 {
		t.Errorf("expected RetryCount=3, got %d", config.RetryCount)
	}
}

func TestGetBaseURL(t *testing.T) {
	tests := []struct {
		version  APIVersion
		override string
		expected string
	}{
		{APILegacy, "", BaseURLLegacy},
		{APIHiTemp, "", BaseURLHiTemp},
		{APIModern, "", BaseURLModern},
		{APIModern, "https://custom.example.com", "https://custom.example.com"},
	}

	for _, tt := range tests {
		config := &Config{
			APIVersion: tt.version,
			BaseURL:    tt.override,
		}
		got := config.GetBaseURL()
		if got != tt.expected {
			t.Errorf("GetBaseURL(%s, %s) = %s; want %s", tt.version, tt.override, got, tt.expected)
		}
	}
}

func TestValidateTemperature(t *testing.T) {
	tests := []struct {
		temp    float64
		wantErr bool
	}{
		{15.0, false},
		{25.0, false},
		{40.0, false},
		{14.9, true},
		{40.1, true},
		{0, true},
		{100, true},
	}

	for _, tt := range tests {
		err := ValidateTemperature(tt.temp)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateTemperature(%f) error = %v; wantErr %v", tt.temp, err, tt.wantErr)
		}
	}
}

func TestValidateMode(t *testing.T) {
	tests := []struct {
		mode    OperationMode
		wantErr bool
	}{
		{ModeOff, false},
		{ModeHeat, false},
		{ModeCool, false},
		{ModeAuto, false},
		{OperationMode("invalid"), true},
	}

	for _, tt := range tests {
		err := ValidateMode(tt.mode)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateMode(%s) error = %v; wantErr %v", tt.mode, err, tt.wantErr)
		}
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		wantErr  bool
	}{
		{"valid", "user@example.com", "password", false},
		{"missing username", "", "password", true},
		{"missing password", "user@example.com", "", true},
		{"both missing", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Username: tt.username,
				Password: tt.password,
			}
			err := config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v; wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeviceDataHelpers(t *testing.T) {
	data := &DeviceData{
		Power:     true,
		ErrorCode: 0,
	}

	if !data.IsPoweredOn() {
		t.Error("expected IsPoweredOn() = true")
	}

	if data.HasError() {
		t.Error("expected HasError() = false")
	}

	data.Power = false
	data.ErrorCode = 1

	if data.IsPoweredOn() {
		t.Error("expected IsPoweredOn() = false")
	}

	if !data.HasError() {
		t.Error("expected HasError() = true")
	}
}

func TestNewClientWithOptions(t *testing.T) {
	client, err := NewClient(
		WithUsername("test@example.com"),
		WithPassword("testpass"),
		WithAPIVersion(APILegacy),
		WithTimeout(60*time.Second),
		WithRetryCount(5),
		WithDebug(true),
		WithDefaultDevice("DEVICE001"),
	)

	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	config := client.Config()

	if config.Username != "test@example.com" {
		t.Errorf("Username = %s; want test@example.com", config.Username)
	}
	if config.Password != "testpass" {
		t.Errorf("Password = %s; want testpass", config.Password)
	}
	if config.APIVersion != APILegacy {
		t.Errorf("APIVersion = %s; want %s", config.APIVersion, APILegacy)
	}
	if config.Timeout != 60*time.Second {
		t.Errorf("Timeout = %s; want 60s", config.Timeout)
	}
	if config.RetryCount != 5 {
		t.Errorf("RetryCount = %d; want 5", config.RetryCount)
	}
	if !config.Debug {
		t.Error("Debug = false; want true")
	}
	if config.DefaultDevice != "DEVICE001" {
		t.Errorf("DefaultDevice = %s; want DEVICE001", config.DefaultDevice)
	}
}

func TestValidateTemperatureWrapsErrInvalidTemperature(t *testing.T) {
	err := ValidateTemperature(100.0)
	if err == nil {
		t.Fatal("expected error for temperature 100.0")
	}
	if !errors.Is(err, ErrInvalidTemperature) {
		t.Errorf("ValidateTemperature(100.0) should wrap ErrInvalidTemperature, got: %v", err)
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Errorf("ValidateTemperature(100.0) should return *ValidationError, got: %T", err)
	}
}

func TestValidateModeWrapsErrInvalidMode(t *testing.T) {
	err := ValidateMode(OperationMode("invalid"))
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("ValidateMode(invalid) should wrap ErrInvalidMode, got: %v", err)
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Errorf("ValidateMode(invalid) should return *ValidationError, got: %T", err)
	}
}

func TestControlCommandValidation(t *testing.T) {
	temp := 100.0
	command := ControlCommand{
		DeviceCode: "DEVICE001",
		TargetTemp: &temp,
	}

	c, _ := NewClient(
		WithUsername("test@example.com"),
		WithPassword("testpass"),
	)

	err := c.Control(nil, command)
	if err == nil {
		t.Fatal("expected error for out-of-range temperature")
	}
	if !errors.Is(err, ErrInvalidTemperature) {
		t.Errorf("Control with invalid temp should wrap ErrInvalidTemperature, got: %v", err)
	}
}

func TestControlCommandEmptyDeviceCode(t *testing.T) {
	command := ControlCommand{
		DeviceCode: "",
	}

	c, _ := NewClient(
		WithUsername("test@example.com"),
		WithPassword("testpass"),
	)

	err := c.Control(nil, command)
	if err == nil {
		t.Fatal("expected error for empty device code")
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Errorf("Control with empty device code should return *ValidationError, got: %T", err)
	}
}

func TestParseModeMapping(t *testing.T) {
	tests := []struct {
		input string
		want  OperationMode
	}{
		{"0", ModeOff},
		{"1", ModeHeat},
		{"2", ModeCool},
		{"3", ModeAuto},
		{"unknown", ModeOff},
		{"", ModeOff},
	}

	for _, tt := range tests {
		got := parseMode(tt.input)
		if got != tt.want {
			t.Errorf("parseMode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseFanSpeedMapping(t *testing.T) {
	tests := []struct {
		input string
		want  FanSpeed
	}{
		{"0", FanLow},
		{"1", FanMedium},
		{"2", FanHigh},
		{"3", FanAuto},
		{"unknown", FanAuto},
		{"", FanAuto},
	}

	for _, tt := range tests {
		got := parseFanSpeed(tt.input)
		if got != tt.want {
			t.Errorf("parseFanSpeed(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestModeToAPI(t *testing.T) {
	tests := []struct {
		input OperationMode
		want  string
	}{
		{ModeOff, "0"},
		{ModeHeat, "1"},
		{ModeCool, "2"},
		{ModeAuto, "3"},
		{OperationMode("unknown"), "0"},
	}

	for _, tt := range tests {
		got := modeToAPI(tt.input)
		if got != tt.want {
			t.Errorf("modeToAPI(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFanSpeedToAPI(t *testing.T) {
	tests := []struct {
		input FanSpeed
		want  string
	}{
		{FanLow, "0"},
		{FanMedium, "1"},
		{FanHigh, "2"},
		{FanAuto, "3"},
		{FanSpeed("unknown"), "3"},
	}

	for _, tt := range tests {
		got := fanSpeedToAPI(tt.input)
		if got != tt.want {
			t.Errorf("fanSpeedToAPI(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestControlCommandNoFields(t *testing.T) {
	command := ControlCommand{
		DeviceCode: "DEVICE001",
	}

	c, _ := NewClient(
		WithUsername("test@example.com"),
		WithPassword("testpass"),
	)

	err := c.Control(nil, command)
	if err != nil {
		t.Errorf("Control with no fields should return nil, got: %v", err)
	}
}
