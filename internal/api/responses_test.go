package api

import (
	"testing"
)

func TestBaseResponseIsSuccess(t *testing.T) {
	tests := []struct {
		errorCode int
		want      bool
	}{
		{0, true},
		{1, false},
		{-1, false},
		{500, false},
	}

	for _, tt := range tests {
		resp := &BaseResponse{ErrorCode: tt.errorCode}
		got := resp.IsSuccess()
		if got != tt.want {
			t.Errorf("BaseResponse{ErrorCode: %d}.IsSuccess() = %v; want %v", tt.errorCode, got, tt.want)
		}
	}
}

func TestDeviceInfoIsOnline(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"1", true},
		{"0", false},
		{"", false},
		{"2", false},
	}

	for _, tt := range tests {
		info := &DeviceInfo{OnlineStatus: tt.status}
		got := info.IsOnline()
		if got != tt.want {
			t.Errorf("DeviceInfo{OnlineStatus: %q}.IsOnline() = %v; want %v", tt.status, got, tt.want)
		}
	}
}

func TestParseDeviceData(t *testing.T) {
	resp := &DeviceDataResponse{
		ObjectResult: []Parameter{
			{Code: ParamPower, Value: APIPowerOn},
			{Code: ParamMode, Value: APIModeHeat},
			{Code: ParamTargetTemp, Value: "28.5"},
			{Code: ParamCurrentTemp, Value: "26.0"},
			{Code: ParamInletTemp, Value: "24.0"},
			{Code: ParamOutletTemp, Value: "30.0"},
			{Code: ParamAmbientTemp, Value: "20.0"},
			{Code: ParamCoilTemp, Value: "35.0"},
			{Code: ParamFanSpeed, Value: APIFanAuto},
			{Code: ParamErrorCode, Value: "0"},
		},
	}

	data := resp.ParseDeviceData()

	if !data.Power {
		t.Error("expected Power = true")
	}
	if data.Mode != APIModeHeat {
		t.Errorf("Mode = %s; want %s", data.Mode, APIModeHeat)
	}
	if data.TargetTemp != 28.5 {
		t.Errorf("TargetTemp = %f; want 28.5", data.TargetTemp)
	}
	if data.CurrentTemp != 26.0 {
		t.Errorf("CurrentTemp = %f; want 26.0", data.CurrentTemp)
	}
	if data.InletTemp != 24.0 {
		t.Errorf("InletTemp = %f; want 24.0", data.InletTemp)
	}
	if data.OutletTemp != 30.0 {
		t.Errorf("OutletTemp = %f; want 30.0", data.OutletTemp)
	}
	if data.AmbientTemp != 20.0 {
		t.Errorf("AmbientTemp = %f; want 20.0", data.AmbientTemp)
	}
	if data.CoilTemp != 35.0 {
		t.Errorf("CoilTemp = %f; want 35.0", data.CoilTemp)
	}
	if data.FanSpeed != APIFanAuto {
		t.Errorf("FanSpeed = %s; want %s", data.FanSpeed, APIFanAuto)
	}
	if data.ErrorCode != 0 {
		t.Errorf("ErrorCode = %d; want 0", data.ErrorCode)
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"28.5", 28.5},
		{"0", 0},
		{"-10.5", -10.5},
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		got := parseFloat(tt.input)
		if got != tt.want {
			t.Errorf("parseFloat(%q) = %f; want %f", tt.input, got, tt.want)
		}
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"42", 42},
		{"0", 0},
		{"-10", -10},
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		got := parseInt(tt.input)
		if got != tt.want {
			t.Errorf("parseInt(%q) = %d; want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseDeviceDataMissingParameters(t *testing.T) {
	resp := &DeviceDataResponse{
		ObjectResult: []Parameter{
			{Code: ParamPower, Value: APIPowerOn},
		},
	}

	data := resp.ParseDeviceData()

	if !data.Power {
		t.Error("expected Power = true")
	}
	if data.TargetTemp != 0 {
		t.Errorf("TargetTemp = %f; want 0 (default for missing param)", data.TargetTemp)
	}
	if data.CurrentTemp != 0 {
		t.Errorf("CurrentTemp = %f; want 0 (default for missing param)", data.CurrentTemp)
	}
	if data.Mode != "" {
		t.Errorf("Mode = %q; want empty string (default for missing param)", data.Mode)
	}
}

func TestParseDeviceDataEmptyValues(t *testing.T) {
	resp := &DeviceDataResponse{
		ObjectResult: []Parameter{
			{Code: ParamPower, Value: ""},
			{Code: ParamMode, Value: ""},
			{Code: ParamTargetTemp, Value: ""},
			{Code: ParamCurrentTemp, Value: ""},
			{Code: ParamErrorCode, Value: ""},
		},
	}

	data := resp.ParseDeviceData()

	if data.Power {
		t.Error("expected Power = false for empty value")
	}
	if data.TargetTemp != 0 {
		t.Errorf("TargetTemp = %f; want 0 for empty value", data.TargetTemp)
	}
	if data.ErrorCode != 0 {
		t.Errorf("ErrorCode = %d; want 0 for empty value", data.ErrorCode)
	}
}

func TestParsedCreateTimeValidFormat(t *testing.T) {
	info := &DeviceInfo{CreateTime: "2024-01-15 10:30:00"}
	parsed := info.ParsedCreateTime()

	if parsed.Year() != 2024 {
		t.Errorf("Year = %d, want 2024", parsed.Year())
	}
	if parsed.Month() != 1 {
		t.Errorf("Month = %d, want 1", parsed.Month())
	}
	if parsed.Day() != 15 {
		t.Errorf("Day = %d, want 15", parsed.Day())
	}
	if parsed.Hour() != 10 {
		t.Errorf("Hour = %d, want 10", parsed.Hour())
	}
}

func TestParsedCreateTimeInvalidFormat(t *testing.T) {
	info := &DeviceInfo{CreateTime: "not-a-date"}
	parsed := info.ParsedCreateTime()

	if !parsed.IsZero() {
		t.Errorf("expected zero time for invalid format, got %v", parsed)
	}
}

func TestParseDeviceDataRawParams(t *testing.T) {
	resp := &DeviceDataResponse{
		ObjectResult: []Parameter{
			{Code: ParamPower, Value: "1"},
			{Code: ParamMode, Value: "2"},
			{Code: "CustomParam", Value: "custom_value"},
		},
	}

	data := resp.ParseDeviceData()

	if len(data.RawParams) != 3 {
		t.Errorf("RawParams length = %d, want 3", len(data.RawParams))
	}
	if data.RawParams["CustomParam"] != "custom_value" {
		t.Errorf("RawParams[CustomParam] = %q, want %q", data.RawParams["CustomParam"], "custom_value")
	}
}
