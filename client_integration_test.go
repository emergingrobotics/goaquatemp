package goaquatemp

import (
	"context"
	"errors"
	"testing"

	"github.com/user/goaquatemp/internal/testutil"
)

func setupIntegrationTest(t *testing.T) (*testutil.FakeServer, Client) {
	t.Helper()
	server := testutil.NewFakeServer()
	t.Cleanup(server.Close)

	server.Devices = []testutil.DeviceInfo{
		{
			DeviceCode:   "DEV001",
			DeviceNick:   "Pool Heater",
			DeviceType:   "1",
			OnlineStatus: "1",
			ProductCode:  "PROD001",
			CreateTime:   "2024-01-15 10:30:00",
		},
		{
			DeviceCode:   "DEV002",
			DeviceNick:   "Spa Heater",
			DeviceType:   "1",
			OnlineStatus: "0",
			ProductCode:  "PROD001",
			CreateTime:   "2024-03-20 14:00:00",
		},
	}

	server.DeviceData["DEV001"] = []testutil.Parameter{
		{Code: "Power", Value: "1"},
		{Code: "Mode", Value: "1"},
		{Code: "R02", Value: "28.5"},
		{Code: "T02", Value: "26.3"},
		{Code: "T03", Value: "25.0"},
		{Code: "T04", Value: "27.1"},
		{Code: "T05", Value: "22.0"},
		{Code: "T06", Value: "30.5"},
		{Code: "Manual-mute", Value: "3"},
		{Code: "T14", Value: "0"},
	}

	client, err := NewClient(
		WithUsername(server.Username),
		WithPassword(server.Password),
		WithAPIVersion(APILegacy),
		WithBaseURL(server.URL()),
		WithRetryCount(0),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	return server, client
}

func TestIntegrationLoginWithValidCredentials(t *testing.T) {
	server, client := setupIntegrationTest(t)

	err := client.Login(context.Background())
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if !client.IsAuthenticated() {
		t.Error("expected IsAuthenticated() = true after login")
	}

	if server.LoginCount != 1 {
		t.Errorf("LoginCount = %d, want 1", server.LoginCount)
	}
}

func TestIntegrationLoginWithInvalidCredentials(t *testing.T) {
	server := testutil.NewFakeServer()
	defer server.Close()

	client, err := NewClient(
		WithUsername("wrong@example.com"),
		WithPassword("wrongpass"),
		WithAPIVersion(APILegacy),
		WithBaseURL(server.URL()),
		WithRetryCount(0),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = client.Login(context.Background())
	if err == nil {
		t.Fatal("Login() expected error for invalid credentials, got nil")
	}

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Login() error should wrap ErrInvalidCredentials, got: %v", err)
	}
}

func TestIntegrationGetDevicesAfterLogin(t *testing.T) {
	_, client := setupIntegrationTest(t)
	ctx := context.Background()

	if err := client.Login(ctx); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	devices, err := client.GetDevices(ctx)
	if err != nil {
		t.Fatalf("GetDevices() error = %v", err)
	}

	if len(devices) != 2 {
		t.Fatalf("GetDevices() returned %d devices, want 2", len(devices))
	}

	if devices[0].Code != "DEV001" {
		t.Errorf("devices[0].Code = %q, want %q", devices[0].Code, "DEV001")
	}
	if devices[0].Name != "Pool Heater" {
		t.Errorf("devices[0].Name = %q, want %q", devices[0].Name, "Pool Heater")
	}
	if !devices[0].Online {
		t.Error("devices[0].Online = false, want true")
	}

	if devices[1].Code != "DEV002" {
		t.Errorf("devices[1].Code = %q, want %q", devices[1].Code, "DEV002")
	}
	if devices[1].Online {
		t.Error("devices[1].Online = true, want false")
	}
}

func TestIntegrationGetDeviceByCode(t *testing.T) {
	_, client := setupIntegrationTest(t)
	ctx := context.Background()

	if err := client.Login(ctx); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	device, err := client.GetDevice(ctx, "DEV001")
	if err != nil {
		t.Fatalf("GetDevice() error = %v", err)
	}

	if device.Code != "DEV001" {
		t.Errorf("device.Code = %q, want %q", device.Code, "DEV001")
	}
	if device.Name != "Pool Heater" {
		t.Errorf("device.Name = %q, want %q", device.Name, "Pool Heater")
	}
}

func TestIntegrationGetDeviceNotFound(t *testing.T) {
	_, client := setupIntegrationTest(t)
	ctx := context.Background()

	if err := client.Login(ctx); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	_, err := client.GetDevice(ctx, "NONEXISTENT")
	if err == nil {
		t.Fatal("GetDevice() expected error for nonexistent device, got nil")
	}

	if !errors.Is(err, ErrDeviceNotFound) {
		t.Errorf("GetDevice() error should wrap ErrDeviceNotFound, got: %v", err)
	}
}

func TestIntegrationGetDeviceDataParsing(t *testing.T) {
	_, client := setupIntegrationTest(t)
	ctx := context.Background()

	if err := client.Login(ctx); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	data, err := client.GetDeviceData(ctx, "DEV001")
	if err != nil {
		t.Fatalf("GetDeviceData() error = %v", err)
	}

	if data.DeviceCode != "DEV001" {
		t.Errorf("DeviceCode = %q, want %q", data.DeviceCode, "DEV001")
	}
	if !data.Power {
		t.Error("Power = false, want true")
	}
	if data.Mode != ModeHeat {
		t.Errorf("Mode = %q, want %q", data.Mode, ModeHeat)
	}
	if data.TargetTemp != 28.5 {
		t.Errorf("TargetTemp = %f, want 28.5", data.TargetTemp)
	}
	if data.CurrentTemp != 26.3 {
		t.Errorf("CurrentTemp = %f, want 26.3", data.CurrentTemp)
	}
	if data.InletTemp != 25.0 {
		t.Errorf("InletTemp = %f, want 25.0", data.InletTemp)
	}
	if data.OutletTemp != 27.1 {
		t.Errorf("OutletTemp = %f, want 27.1", data.OutletTemp)
	}
	if data.AmbientTemp != 22.0 {
		t.Errorf("AmbientTemp = %f, want 22.0", data.AmbientTemp)
	}
	if data.CoilTemp != 30.5 {
		t.Errorf("CoilTemp = %f, want 30.5", data.CoilTemp)
	}
	if data.FanSpeed != FanAuto {
		t.Errorf("FanSpeed = %q, want %q", data.FanSpeed, FanAuto)
	}
	if data.ErrorCode != 0 {
		t.Errorf("ErrorCode = %d, want 0", data.ErrorCode)
	}
}

func TestIntegrationSetTemperature(t *testing.T) {
	server, client := setupIntegrationTest(t)
	ctx := context.Background()

	if err := client.Login(ctx); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if err := client.SetTemperature(ctx, "DEV001", 28.0); err != nil {
		t.Fatalf("SetTemperature() error = %v", err)
	}

	if len(server.ControlCalls) != 1 {
		t.Fatalf("ControlCalls count = %d, want 1", len(server.ControlCalls))
	}

	call := server.ControlCalls[0]
	if call.DeviceCode != "DEV001" {
		t.Errorf("ControlCall.DeviceCode = %q, want %q", call.DeviceCode, "DEV001")
	}
	if call.ProtocolCode != "R02" {
		t.Errorf("ControlCall.ProtocolCode = %q, want %q", call.ProtocolCode, "R02")
	}
	if call.Value != "28.0" {
		t.Errorf("ControlCall.Value = %q, want %q", call.Value, "28.0")
	}
}

func TestIntegrationSetTemperatureOutOfRange(t *testing.T) {
	_, client := setupIntegrationTest(t)

	err := client.SetTemperature(context.Background(), "DEV001", 100.0)
	if err == nil {
		t.Fatal("SetTemperature() expected error for out-of-range temp, got nil")
	}

	if !errors.Is(err, ErrInvalidTemperature) {
		t.Errorf("SetTemperature() error should wrap ErrInvalidTemperature, got: %v", err)
	}
}

func TestIntegrationSetMode(t *testing.T) {
	server, client := setupIntegrationTest(t)
	ctx := context.Background()

	if err := client.Login(ctx); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if err := client.SetMode(ctx, "DEV001", ModeCool); err != nil {
		t.Fatalf("SetMode() error = %v", err)
	}

	if len(server.ControlCalls) != 1 {
		t.Fatalf("ControlCalls count = %d, want 1", len(server.ControlCalls))
	}

	call := server.ControlCalls[0]
	if call.ProtocolCode != "Mode" {
		t.Errorf("ControlCall.ProtocolCode = %q, want %q", call.ProtocolCode, "Mode")
	}
	if call.Value != "2" {
		t.Errorf("ControlCall.Value = %q, want %q", call.Value, "2")
	}
}

func TestIntegrationSetPowerOnOff(t *testing.T) {
	server, client := setupIntegrationTest(t)
	ctx := context.Background()

	if err := client.Login(ctx); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if err := client.SetPower(ctx, "DEV001", true); err != nil {
		t.Fatalf("SetPower(true) error = %v", err)
	}

	if err := client.SetPower(ctx, "DEV001", false); err != nil {
		t.Fatalf("SetPower(false) error = %v", err)
	}

	if len(server.ControlCalls) != 2 {
		t.Fatalf("ControlCalls count = %d, want 2", len(server.ControlCalls))
	}

	if server.ControlCalls[0].Value != "1" {
		t.Errorf("SetPower(true) value = %q, want %q", server.ControlCalls[0].Value, "1")
	}
	if server.ControlCalls[1].Value != "0" {
		t.Errorf("SetPower(false) value = %q, want %q", server.ControlCalls[1].Value, "0")
	}
}

func TestIntegrationLogoutClearsToken(t *testing.T) {
	_, client := setupIntegrationTest(t)
	ctx := context.Background()

	if err := client.Login(ctx); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if !client.IsAuthenticated() {
		t.Error("expected IsAuthenticated() = true after login")
	}

	if err := client.Logout(ctx); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	if client.IsAuthenticated() {
		t.Error("expected IsAuthenticated() = false after logout")
	}
}

func TestIntegrationControlCommand(t *testing.T) {
	server, client := setupIntegrationTest(t)
	ctx := context.Background()

	if err := client.Login(ctx); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	power := true
	mode := ModeCool
	temp := 25.0
	command := ControlCommand{
		DeviceCode: "DEV001",
		Power:      &power,
		Mode:       &mode,
		TargetTemp: &temp,
	}

	if err := client.Control(ctx, command); err != nil {
		t.Fatalf("Control() error = %v", err)
	}

	if len(server.ControlCalls) != 3 {
		t.Fatalf("ControlCalls count = %d, want 3", len(server.ControlCalls))
	}

	if server.ControlCalls[0].ProtocolCode != "Power" || server.ControlCalls[0].Value != "1" {
		t.Errorf("ControlCall[0] = %+v, want Power=1", server.ControlCalls[0])
	}
	if server.ControlCalls[1].ProtocolCode != "Mode" || server.ControlCalls[1].Value != "2" {
		t.Errorf("ControlCall[1] = %+v, want Mode=2", server.ControlCalls[1])
	}
	if server.ControlCalls[2].ProtocolCode != "R02" || server.ControlCalls[2].Value != "25.0" {
		t.Errorf("ControlCall[2] = %+v, want R02=25.0", server.ControlCalls[2])
	}
}
