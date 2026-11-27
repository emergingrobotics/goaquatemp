package api

import (
	"encoding/json"
	"testing"
)

func TestParseResponseStringErrorCode(t *testing.T) {
	// Captured from the real API (login with the wrong request format).
	body := []byte(`{"error_msg":"请重新登录","error_msg_code":"","isReusltSuc":false,"objectResult":null,"error_code":"-100"}`)

	resp, err := Modern.ParseResponse(body)
	if err != nil {
		t.Fatalf("ParseResponse() error = %v", err)
	}
	if resp.ErrorCode != ErrCodeTokenInvalid {
		t.Errorf("ErrorCode = %d, want %d", resp.ErrorCode, ErrCodeTokenInvalid)
	}
	if resp.IsSuccess() {
		t.Error("IsSuccess() = true, want false")
	}
	if resp.ErrorMessage != "请重新登录" {
		t.Errorf("ErrorMessage = %q", resp.ErrorMessage)
	}
}

func TestParseResponseNumericErrorCode(t *testing.T) {
	resp, err := Modern.ParseResponse([]byte(`{"error_code":0,"error_msg":"Success"}`))
	if err != nil {
		t.Fatalf("ParseResponse() error = %v", err)
	}
	if !resp.IsSuccess() {
		t.Error("IsSuccess() = false, want true")
	}
}

func TestParseResponseUsesDialectResultKey(t *testing.T) {
	body := []byte(`{"error_code":"0","objectResult":{"a":1},"object_result":{"b":2}}`)

	m, _ := Modern.ParseResponse(body)
	if string(m.Result) != `{"a":1}` {
		t.Errorf("Modern result = %s", m.Result)
	}
	l, _ := Legacy.ParseResponse(body)
	if string(l.Result) != `{"b":2}` {
		t.Errorf("Legacy result = %s", l.Result)
	}
}

func TestParseLogin(t *testing.T) {
	res, err := Modern.ParseLogin(json.RawMessage(`{"x-token":"tok","userId":"u1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.Token != "tok" || res.UserID != "u1" {
		t.Errorf("ParseLogin() = %+v", res)
	}
}

func TestParseDevices(t *testing.T) {
	result := json.RawMessage(`[{"deviceCode":"D1","deviceNickName":"Pool","productId":"P1","deviceStatus":"ONLINE"},
		{"deviceCode":"D2","deviceNickName":"Spa","deviceStatus":"OFFLINE"}]`)

	devices, err := Modern.ParseDevices(result)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 {
		t.Fatalf("len = %d, want 2", len(devices))
	}
	if devices[0].DeviceCode != "D1" || devices[0].Nickname != "Pool" || devices[0].ProductID != "P1" || !devices[0].IsOnline() {
		t.Errorf("devices[0] = %+v", devices[0])
	}
	if devices[1].IsOnline() {
		t.Error("devices[1].IsOnline() = true, want false")
	}
}

func TestParseDataParams(t *testing.T) {
	params, err := ParseDataParams(json.RawMessage(`[{"code":"T02","value":"26.5"},{"code":"Power","value":1}]`))
	if err != nil {
		t.Fatal(err)
	}
	if params["T02"] != "26.5" || params["Power"] != "1" {
		t.Errorf("params = %v", params)
	}
}

func TestParseDeviceData(t *testing.T) {
	data := ParseDeviceData(map[string]string{
		ParamPower:     APIPowerOn,
		ParamMode:      APIModeHeat,
		ParamCoolSet:   "20",
		ParamHeatSet:   "28.5",
		ParamSetTemp:   "27",
		ParamInletTemp: "24.0",
		ParamOutlet:    "30.0",
		ParamAmbient:   "20.0",
		ParamCoilTemp:  "35.0",
		ParamFanSpeed:  APIFanAuto,
	})

	if !data.Power {
		t.Error("expected Power = true")
	}
	if data.TargetTemp != 28.5 {
		t.Errorf("TargetTemp = %v; want 28.5 (R02 for heat)", data.TargetTemp)
	}
	if data.InletTemp != 24 || data.OutletTemp != 30 || data.AmbientTemp != 20 || data.CoilTemp != 35 {
		t.Errorf("temps = %+v", data)
	}
}

func TestParseDeviceDataTargetFallsBackToSetTemp(t *testing.T) {
	data := ParseDeviceData(map[string]string{ParamSetTemp: "27"})
	if data.TargetTemp != 27 {
		t.Errorf("TargetTemp = %v; want 27", data.TargetTemp)
	}
}

func TestControlRequestShape(t *testing.T) {
	req := Modern.ControlRequest("D1", []ControlValue{{ProtocolCode: "R02", Value: 28.0}})
	b, _ := json.Marshal(req)
	want := `{"param":[{"deviceCode":"D1","protocolCode":"R02","value":28}]}`
	if string(b) != want {
		t.Errorf("ControlRequest JSON = %s, want %s", b, want)
	}
}

func TestLoginRequestShape(t *testing.T) {
	b, _ := json.Marshal(Modern.LoginRequest("u@example.com", "hash"))
	want := `{"password":"hash","userName":"u@example.com"}`
	if string(b) != want {
		t.Errorf("LoginRequest JSON = %s, want %s", b, want)
	}
	if Modern.Path(EndpointLogin) != "/app/user/login" || Legacy.Path(EndpointLogin) != "/app/user/login.json" {
		t.Error("unexpected Path() suffix handling")
	}
}

func TestDeviceDataRequestByDialect(t *testing.T) {
	b, _ := json.Marshal(Modern.DeviceDataRequest("D1", []string{"T02"}, []string{"Sys1CompState"}))
	if string(b) != `{"bitCodes":["Sys1CompState"],"codes":["T02"],"deviceCode":"D1"}` {
		t.Errorf("modern data request = %s", b)
	}
	b, _ = json.Marshal(Legacy.DeviceDataRequest("D1", []string{"T02"}, []string{"Sys1CompState"}))
	if string(b) != `{"device_code":"D1","protocal_codes":["T02"]}` {
		t.Errorf("legacy data request = %s (legacy cannot read bits)", b)
	}
	if Modern.DataEndpoint != EndpointDeviceDataV5 || Legacy.DataEndpoint != EndpointDeviceData {
		t.Error("unexpected data endpoints")
	}
}

func TestPathKeepsQueryAfterSuffix(t *testing.T) {
	if got := Legacy.Path("/app/x?lang=en"); got != "/app/x.json?lang=en" {
		t.Errorf("Legacy.Path = %q", got)
	}
	if got := Modern.Path("/app/x?lang=en"); got != "/app/x?lang=en" {
		t.Errorf("Modern.Path = %q", got)
	}
}

func TestHistoryRequestShape(t *testing.T) {
	b, _ := json.Marshal(Modern.HistoryRequest("D1", "2046", "2026-11-16 00:00:00", "2026-11-16 23:59:59", "day", -8))
	want := `{"address":"2046","deviceCode":"D1","endTime":"2026-11-16 23:59:59","frequency":"day","startTime":"2026-11-16 00:00:00","timeZone":"-8"}`
	if string(b) != want {
		t.Errorf("history request = %s, want %s", b, want)
	}
}

func TestParseHistory(t *testing.T) {
	h, err := ParseHistory(json.RawMessage(`{"title":"2046 进水温度","total":"2","configId":"C1",
		"valueList":[{"dateTime":"2026-11-16 00","addressValue":"30.00"},{"dateTime":"2026-11-16 01","addressValue":"29.50"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if h.ConfigID != "C1" || len(h.Points) != 2 || h.Points[1].Value != "29.50" {
		t.Errorf("ParseHistory() = %+v", h)
	}
	h, _ = ParseHistory(json.RawMessage(`{"title":"","configId":"","valueList":[]}`))
	if h.ConfigID != "" || len(h.Points) != 0 {
		t.Errorf("empty history = %+v", h)
	}
}

func TestParseDeviceStatusAndFaults(t *testing.T) {
	st, err := Modern.ParseDeviceStatus(json.RawMessage(`{"status":"ONLINE","isFault":false,"is_fault":false}`))
	if err != nil || st.Status != "ONLINE" || st.IsFault {
		t.Errorf("ParseDeviceStatus() = %+v, %v", st, err)
	}
	st, _ = Modern.ParseDeviceStatus(json.RawMessage(`{"status":"OFFLINE","isFault":true}`))
	if st.Status != "OFFLINE" || !st.IsFault {
		t.Errorf("ParseDeviceStatus(fault) = %+v", st)
	}
	faults, err := ParseFaults(json.RawMessage(`[{"description":"E01"}]`))
	if err != nil || len(faults) != 1 || faults[0].Description != "E01" {
		t.Errorf("ParseFaults() = %+v, %v", faults, err)
	}
}

func TestParseParametersUsesDataValueOrValue(t *testing.T) {
	params, err := ParseParameters(json.RawMessage(`{"total":2,"dataList":[
		{"num":"T02","dataValue":"28.5","address":"2046","standardNum":"InletWaterTemp"},
		{"num":"R02 ","value":"32.20","content":" Heat target "}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if params[0].Value != "28.5" || params[0].Address != "2046" {
		t.Errorf("params[0] = %+v", params[0])
	}
	if params[1].Code != "R02" || params[1].Value != "32.20" || params[1].Label != "Heat target" {
		t.Errorf("params[1] = %+v", params[1])
	}
}

func TestParseDeviceDataBitsAndOptionals(t *testing.T) {
	d := ParseDeviceData(map[string]string{
		ParamPower: "1", ParamMode: APIModeCool, ParamModeState: APIStateCooling,
		ParamCompFreq: "40", ParamHeatMaxFreq: "85", ParamCoolMaxFreq: "52",
		BitCompressor: "1", BitWaterFlow: "1", ParamExhaust: "",
	})
	if d.CompressorRunning == nil || !*d.CompressorRunning {
		t.Error("CompressorRunning should be true")
	}
	if d.WaterFlowing == nil || *d.WaterFlowing {
		t.Error("WaterFlowing should be false (bit 1 = no flow)")
	}
	if d.CompressorMaxFreq == nil || *d.CompressorMaxFreq != 52 {
		t.Errorf("CompressorMaxFreq = %v, want cooling max 52", d.CompressorMaxFreq)
	}
	if d.ExhaustTemp != nil {
		t.Error("empty value should give nil ExhaustTemp")
	}
	if d.ExternalStop != nil {
		t.Error("absent bit should give nil ExternalStop")
	}
	if d.InverterInputVoltage != nil || d.RealPowerRaw != nil || d.EnergyTotal != nil {
		t.Error("absent electrical codes should give nil fields, not 0")
	}
}
