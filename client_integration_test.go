package goaquatemp

import (
	"context"
	"errors"
	"testing"

	"github.com/emergingrobotics/goaquatemp/internal/testutil"
)

func setupIntegrationTest(t *testing.T) (*testutil.FakeServer, Client) {
	t.Helper()
	server := testutil.NewFakeServer()
	t.Cleanup(server.Close)

	server.Devices = []testutil.DeviceInfo{
		{DeviceCode: "DEV001", DeviceNickName: "Pool Heater", DeviceStatus: "ONLINE", ProductID: "PROD001"},
		{DeviceCode: "DEV002", DeviceNickName: "Spa Heater", DeviceStatus: "OFFLINE", ProductID: "PROD001"},
	}

	server.DeviceData["DEV001"] = []testutil.Parameter{
		{Code: "Power", Value: "1"},
		{Code: "Mode", Value: "1"},
		{Code: "R01", Value: "20"},
		{Code: "R02", Value: "28.5"},
		{Code: "Set_Temp", Value: "28.5"},
		{Code: "T02", Value: "26.3"},
		{Code: "T03", Value: "27.1"},
		{Code: "T04", Value: "30.5"},
		{Code: "T05", Value: "22.0"},
		{Code: "T06", Value: "45.0"},
		{Code: "Manual-mute", Value: "0"},
	}

	client, err := NewClient(
		WithUsername(server.Username),
		WithPassword(server.Password),
		WithAPIVersion(APIModern),
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
		WithAPIVersion(APIModern),
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
	if data.InletTemp != 26.3 {
		t.Errorf("InletTemp = %f, want 26.3", data.InletTemp)
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

	assertControl(t, server, 1, []testutil.ControlCall{
		{DeviceCode: "DEV001", ProtocolCode: "R02", Value: 28.0},
		{DeviceCode: "DEV001", ProtocolCode: "Set_Temp", Value: 28.0},
	})
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

	assertControl(t, server, 1, []testutil.ControlCall{
		{DeviceCode: "DEV001", ProtocolCode: "power", Value: "1"},
		{DeviceCode: "DEV001", ProtocolCode: "Mode", Value: "0"},
	})
}

func TestIntegrationSetModeOffTurnsPowerOff(t *testing.T) {
	server, client := setupIntegrationTest(t)

	if err := client.SetMode(context.Background(), "DEV001", ModeOff); err != nil {
		t.Fatalf("SetMode() error = %v", err)
	}

	assertControl(t, server, 1, []testutil.ControlCall{
		{DeviceCode: "DEV001", ProtocolCode: "power", Value: "0"},
	})
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

	assertControl(t, server, 2, []testutil.ControlCall{
		{DeviceCode: "DEV001", ProtocolCode: "power", Value: "1"},
		{DeviceCode: "DEV001", ProtocolCode: "power", Value: "0"},
	})
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

	assertControl(t, server, 1, []testutil.ControlCall{
		{DeviceCode: "DEV001", ProtocolCode: "power", Value: "1"},
		{DeviceCode: "DEV001", ProtocolCode: "Mode", Value: "0"},
		{DeviceCode: "DEV001", ProtocolCode: "R01", Value: 25.0},
		{DeviceCode: "DEV001", ProtocolCode: "Set_Temp", Value: 25.0},
	})
}

func TestIntegrationSharedDevicesListed(t *testing.T) {
	server, client := setupIntegrationTest(t)
	server.SharedDevices = []testutil.DeviceInfo{
		{DeviceCode: "DEV003", DeviceNickName: "Shared Heater", DeviceStatus: "ONLINE"},
		{DeviceCode: "DEV001", DeviceNickName: "Pool Heater", DeviceStatus: "ONLINE"}, // duplicate
	}

	devices, err := client.GetDevices(context.Background())
	if err != nil {
		t.Fatalf("GetDevices() error = %v", err)
	}
	if len(devices) != 3 || devices[2].Code != "DEV003" {
		t.Errorf("GetDevices() = %+v, want DEV001, DEV002, DEV003", devices)
	}
}

func TestIntegrationReloginOnInvalidToken(t *testing.T) {
	server, client := setupIntegrationTest(t)
	ctx := context.Background()

	if err := client.Login(ctx); err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	// Token revoked server-side (e.g. the mobile app logged in).
	server.InvalidateToken("new-token")

	if _, err := client.GetDevices(ctx); err != nil {
		t.Fatalf("GetDevices() after token revocation error = %v", err)
	}
	if server.LoginCount != 2 {
		t.Errorf("LoginCount = %d, want 2 (re-login after -100)", server.LoginCount)
	}
}

func TestIntegrationSetFanSpeedRejectsUnsupported(t *testing.T) {
	_, client := setupIntegrationTest(t)
	if err := client.SetFanSpeed(context.Background(), "DEV001", FanSpeed("high")); err == nil {
		t.Fatal("SetFanSpeed(high) expected validation error")
	}
}

func assertControl(t *testing.T, server *testutil.FakeServer, wantReqs int, want []testutil.ControlCall) {
	t.Helper()
	if server.ControlReqs != wantReqs {
		t.Errorf("control requests = %d, want %d", server.ControlReqs, wantReqs)
	}
	if len(server.ControlCalls) != len(want) {
		t.Fatalf("ControlCalls = %+v, want %+v", server.ControlCalls, want)
	}
	for i := range want {
		if server.ControlCalls[i] != want[i] {
			t.Errorf("ControlCalls[%d] = %+v, want %+v", i, server.ControlCalls[i], want[i])
		}
	}
}
