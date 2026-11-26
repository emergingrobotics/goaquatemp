package goaquatemp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/emergingrobotics/goaquatemp/internal/api"
	"github.com/emergingrobotics/goaquatemp/internal/testutil"
)

var pst = time.FixedZone("PST", -8*3600)

// heatingParams mirrors a live v5 reading taken while the unit was heating.
var heatingParams = []testutil.Parameter{
	{Code: "Power", Value: "1"},
	{Code: "Mode", Value: "1"},
	{Code: "ModeState", Value: "1"},
	{Code: "Set_Temp", Value: "32.2"},
	{Code: "R02", Value: "32.2"},
	{Code: "R10", Value: "15.0"},
	{Code: "R11", Value: "40.0"},
	{Code: "T01", Value: "17.0"},
	{Code: "T02", Value: "28.5"},
	{Code: "T03", Value: "30.5"},
	{Code: "T04", Value: "12.5"},
	{Code: "T05", Value: "26.0"},
	{Code: "T06", Value: "66.0"},
	{Code: "O07", Value: "70"},
	{Code: "O08", Value: "6.9"},
	{Code: "H08", Value: "85"},
	{Code: "H09", Value: "52"},
	{Code: "H12", Value: "3"},
	{Code: "Manual-mute", Value: "0"},
	{Code: "Sys1CompState", Value: "1"},
	{Code: "WaterFlowSwitch", Value: "0"},
	{Code: "EmergencySwitch", Value: "0"},
	{Code: "InletWaterTempSensorFault", Value: "0"},
	{Code: "OutletWaterTempSensorFault", Value: "0"},
	{Code: "ATSensorFault", Value: "1"},
}

func newFeatureTest(t *testing.T) (*testutil.FakeServer, *client) {
	t.Helper()
	server, c := setupIntegrationTest(t)
	server.DeviceData["DEV001"] = heatingParams
	return server, c.(*client)
}

func TestGetDeviceDataUsesV5WithBits(t *testing.T) {
	server, c := newFeatureTest(t)

	data, err := c.GetDeviceData(context.Background(), "DEV001")
	if err != nil {
		t.Fatalf("GetDeviceData() error = %v", err)
	}

	if server.LastDataEndpoint != "/app/device/v5/getDataByCode" {
		t.Errorf("data endpoint = %q, want v5", server.LastDataEndpoint)
	}
	bits, _ := server.LastDataRequest["bitCodes"].([]any)
	if len(bits) == 0 || bits[0] != "Sys1CompState" {
		t.Errorf("bitCodes = %v, want Sys1CompState first", bits)
	}
	if _, ok := server.LastDataRequest["codes"]; !ok {
		t.Error("v5 request must send codes in the \"codes\" field")
	}

	if data.State != StateHeating {
		t.Errorf("State = %q, want heating", data.State)
	}
	if data.CompressorRunning == nil || !*data.CompressorRunning {
		t.Errorf("CompressorRunning = %v, want true", data.CompressorRunning)
	}
	if data.CompressorFrequency == nil || *data.CompressorFrequency != 70 {
		t.Errorf("CompressorFrequency = %v, want 70", data.CompressorFrequency)
	}
	if data.CompressorMaxFrequency == nil || *data.CompressorMaxFrequency != 85 {
		t.Errorf("CompressorMaxFrequency = %v, want 85 (heating max)", data.CompressorMaxFrequency)
	}
	if data.CompressorSpeed == nil || *data.CompressorSpeed != 82.4 {
		t.Errorf("CompressorSpeed = %v, want 82.4", data.CompressorSpeed)
	}
	if data.WaterFlowing == nil || !*data.WaterFlowing {
		t.Errorf("WaterFlowing = %v, want true (bit 0 = flowing)", data.WaterFlowing)
	}
	if data.WaterTempStatus != WaterTempValid {
		t.Errorf("WaterTempStatus = %q, want valid", data.WaterTempStatus)
	}
	if data.ExternalStop == nil || *data.ExternalStop {
		t.Errorf("ExternalStop = %v, want false", data.ExternalStop)
	}
	if len(data.SensorFaults) != 1 || data.SensorFaults[0] != "ATSensorFault" {
		t.Errorf("SensorFaults = %v, want [ATSensorFault]", data.SensorFaults)
	}
	if data.InletTemp != 28.5 || data.OutletTemp != 30.5 || data.AmbientTemp != 26 {
		t.Errorf("temps = %v/%v/%v", data.InletTemp, data.OutletTemp, data.AmbientTemp)
	}
	if data.ExhaustTemp == nil || *data.ExhaustTemp != 66 || data.SuctionTemp == nil || *data.SuctionTemp != 17 {
		t.Errorf("exhaust/suction = %v/%v", data.ExhaustTemp, data.SuctionTemp)
	}
	if data.TargetMin == nil || *data.TargetMin != 15 || data.TargetMax == nil || *data.TargetMax != 40 {
		t.Errorf("target range = %v-%v, want 15-40", data.TargetMin, data.TargetMax)
	}
}

func TestOperatingState(t *testing.T) {
	b := func(v bool) *bool { return &v }
	f := func(v float64) *float64 { return &v }

	tests := []struct {
		name    string
		parsed  api.ParsedDeviceData
		running *bool
		want    OperatingState
	}{
		{"off", api.ParsedDeviceData{Power: false, ModeState: "1"}, b(true), StateOff},
		{"defrosting", api.ParsedDeviceData{Power: true, ModeState: "2"}, b(true), StateDefrosting},
		{"idle", api.ParsedDeviceData{Power: true, ModeState: "1"}, b(false), StateIdle},
		{"heating", api.ParsedDeviceData{Power: true, ModeState: "1"}, b(true), StateHeating},
		{"cooling", api.ParsedDeviceData{Power: true, ModeState: "0"}, b(true), StateCooling},
		{"no state, heat mode", api.ParsedDeviceData{Power: true, Mode: "1"}, b(true), StateHeating},
		{"auto heats below target", api.ParsedDeviceData{Power: true, Mode: "2", TargetTemp: 30, InletTemp: 25}, b(true), StateHeating},
		{"auto cools at target", api.ParsedDeviceData{Power: true, Mode: "2", TargetTemp: 25, InletTemp: 25}, b(true), StateCooling},
		{"nothing known", api.ParsedDeviceData{Power: true, Mode: "1"}, nil, StateUnknown},
	}
	for _, tt := range tests {
		if got := operatingState(&tt.parsed, tt.running); got != tt.want {
			t.Errorf("%s: operatingState() = %q, want %q", tt.name, got, tt.want)
		}
	}

	// Fixed-speed compressors get no percentage.
	if got := compressorSpeed(&api.ParsedDeviceData{CompressorFreq: f(50), CompressorMaxFreq: f(100), CompressorType: f(0)}); got != nil {
		t.Errorf("compressorSpeed(type 0) = %v, want nil", *got)
	}
	if got := compressorSpeed(&api.ParsedDeviceData{CompressorFreq: f(0), CompressorType: f(3)}); got == nil || *got != 0 {
		t.Errorf("compressorSpeed(stopped) = %v, want 0", got)
	}
}

func TestCompressorRunningFallsBackToFrequency(t *testing.T) {
	server, c := newFeatureTest(t)
	server.DeviceData["DEV001"] = []testutil.Parameter{
		{Code: "Power", Value: "1"}, {Code: "Mode", Value: "1"}, {Code: "O07", Value: "0"},
	}

	data, err := c.GetDeviceData(context.Background(), "DEV001")
	if err != nil {
		t.Fatal(err)
	}
	if data.CompressorRunning == nil || *data.CompressorRunning {
		t.Errorf("CompressorRunning = %v, want false from O07=0", data.CompressorRunning)
	}
	if data.State != StateIdle {
		t.Errorf("State = %q, want idle", data.State)
	}
	if data.WaterFlowing != nil {
		t.Errorf("WaterFlowing = %v, want nil when the bit is unknown", *data.WaterFlowing)
	}
	if data.WaterTempStatus != WaterTempUnknown {
		t.Errorf("WaterTempStatus = %q, want unknown", data.WaterTempStatus)
	}
}

func TestGetDeviceStatus(t *testing.T) {
	server, c := newFeatureTest(t)
	ctx := context.Background()

	st, err := c.GetDeviceStatus(ctx, "DEV001")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Online || st.Fault || len(st.Faults) != 0 {
		t.Errorf("status = %+v, want online without faults", st)
	}

	server.DeviceStatus["DEV001"] = "OFFLINE"
	server.DeviceFaults["DEV001"] = []map[string]any{{"description": "P01 inlet sensor"}}
	st, err = c.GetDeviceStatus(ctx, "DEV001")
	if err != nil {
		t.Fatal(err)
	}
	if st.Online || !st.Fault || len(st.Faults) != 1 || st.Faults[0].Description != "P01 inlet sensor" {
		t.Errorf("status = %+v, want offline with one fault", st)
	}
}

func TestGetHistoryHourly(t *testing.T) {
	server, c := newFeatureTest(t)
	c.now = func() time.Time { return time.Date(2026, 11, 16, 2, 30, 0, 0, pst) }
	server.History["2046"] = map[string][]testutil.HistoryValue{
		"day": {
			{DateTime: "2026-11-16 00", AddressValue: "27.00"},
			{DateTime: "2026-11-16 01", AddressValue: "27.50"},
			{DateTime: "2026-11-16 02", AddressValue: "28.00"},
			{DateTime: "2026-11-16 03", AddressValue: "0.00"}, // future
		},
	}

	h, err := c.GetHistory(context.Background(), "DEV001", HistoryQuery{
		Series:     SeriesInletTemp,
		Start:      time.Date(2026, 11, 16, 0, 0, 0, 0, pst),
		End:        time.Date(2026, 11, 16, 23, 59, 59, 0, pst),
		Resolution: ResolutionHourly,
	})
	if err != nil {
		t.Fatalf("GetHistory() error = %v", err)
	}

	req := server.LastHistoryRequest
	if req["startTime"] != "2026-11-16 00:00:00" || req["endTime"] != "2026-11-16 23:59:59" ||
		req["frequency"] != "day" || req["timeZone"] != "-8" || req["address"] != "2046" {
		t.Errorf("history request = %v", req)
	}
	if h.Name != "inlet" || h.Unit != "°C" || h.Resolution != ResolutionHourly {
		t.Errorf("history meta = %+v", h)
	}
	if len(h.Points) != 3 {
		t.Fatalf("points = %+v, want 3 (future bucket dropped)", h.Points)
	}
	want := time.Date(2026, 11, 16, 1, 0, 0, 0, pst)
	if !h.Points[1].Time.Equal(want) || h.Points[1].Value != 27.5 {
		t.Errorf("points[1] = %+v, want 27.5 at %v", h.Points[1], want)
	}
}

func TestGetHistory5MinConvertsChinaTimeAndDropsGaps(t *testing.T) {
	server, c := newFeatureTest(t)
	c.now = func() time.Time { return time.Date(2026, 11, 17, 7, 0, 0, 0, pst) }
	// 5-minute labels are in UTC+8: 2026-11-16 16:00 +8 == 2026-11-16 00:00 PST.
	server.History["2046"] = map[string][]testutil.HistoryValue{
		"week": {
			{DateTime: "2026-11-16 16:00:00", AddressValue: "30.00"},
			{DateTime: "2026-11-16 16:05:00", AddressValue: "0.00"},
			{DateTime: "2026-11-16 16:10:00", AddressValue: "30.50"},
		},
	}
	server.History["2021"] = map[string][]testutil.HistoryValue{
		"week": {
			{DateTime: "2026-11-16 16:00:00", AddressValue: "62.00"},
			{DateTime: "2026-11-16 16:05:00", AddressValue: "0.00"}, // gap between runs
			{DateTime: "2026-11-16 16:10:00", AddressValue: "62.00"},
			{DateTime: "2026-11-16 16:15:00", AddressValue: "0.00"}, // stopped
			{DateTime: "2026-11-16 16:20:00", AddressValue: "0.00"}, // stopped
		},
	}
	start := time.Date(2026, 11, 16, 0, 0, 0, 0, pst)
	end := time.Date(2026, 11, 16, 6, 0, 0, 0, pst)

	h, err := c.GetHistory(context.Background(), "DEV001", HistoryQuery{Series: SeriesInletTemp, Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	if h.Resolution != Resolution5Min {
		t.Errorf("auto resolution = %q, want 5m for a 6h span", h.Resolution)
	}
	req := server.LastHistoryRequest
	// 5-minute windows go out in UTC+8 with timeZone 8: 00:00 PST = 16:00 +8.
	if req["frequency"] != "week" || req["timeZone"] != "8" ||
		req["startTime"] != "2026-11-16 16:00:00" || req["endTime"] != "2026-11-16 22:00:00" {
		t.Errorf("5-minute history request = %v", req)
	}
	if len(h.Points) != 2 || !h.Points[0].Time.Equal(start) || h.Points[1].Value != 30.5 {
		t.Errorf("inlet points = %+v, want 2 points starting at %v", h.Points, start)
	}
	if h.Points[0].Time.Location() != pst {
		t.Errorf("point location = %v, want query location", h.Points[0].Time.Location())
	}

	h, err = c.GetHistory(context.Background(), "DEV001", HistoryQuery{Series: SeriesCompressorFrequency, Start: start, End: end})
	if err != nil {
		t.Fatal(err)
	}
	var got []float64
	for _, p := range h.Points {
		got = append(got, p.Value)
	}
	if len(got) != 4 || got[0] != 62 || got[1] != 62 || got[2] != 0 || got[3] != 0 {
		t.Errorf("compressor values = %v, want [62 62 0 0]", got)
	}

	h, err = c.GetHistory(context.Background(), "DEV001", HistoryQuery{Series: SeriesInletTemp, Start: start, End: end, KeepGaps: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Points) != 3 {
		t.Errorf("KeepGaps points = %d, want 3", len(h.Points))
	}
}

func TestGetHistoryAlignsStart(t *testing.T) {
	server, c := newFeatureTest(t)
	server.History["2046"] = map[string][]testutil.HistoryValue{"day": {}, "week": {}}

	start := time.Date(2026, 11, 16, 4, 22, 18, 0, pst)
	end := start.Add(3 * time.Hour)
	if _, err := c.GetHistory(context.Background(), "DEV001", HistoryQuery{Series: SeriesInletTemp, Start: start, End: end}); err != nil {
		t.Fatal(err)
	}
	if got := server.LastHistoryRequest["startTime"]; got != "2026-11-16 20:20:00" {
		t.Errorf("5-minute startTime = %v, want aligned 2026-11-16 20:20:00 (+8)", got)
	}

	if _, err := c.GetHistory(context.Background(), "DEV001", HistoryQuery{Series: SeriesInletTemp, Start: start, End: end, Resolution: ResolutionHourly}); err != nil {
		t.Fatal(err)
	}
	if got := server.LastHistoryRequest["startTime"]; got != "2026-11-16 04:00:00" {
		t.Errorf("hourly startTime = %v, want 2026-11-16 04:00:00", got)
	}
}

func TestGetHistoryUnavailable(t *testing.T) {
	_, c := newFeatureTest(t)
	_, err := c.GetHistory(context.Background(), "DEV001", HistoryQuery{
		Series: "2045", Start: time.Now().Add(-time.Hour),
	})
	if !errors.Is(err, ErrHistoryUnavailable) {
		t.Errorf("GetHistory(no config) error = %v, want ErrHistoryUnavailable", err)
	}
}

func TestGetHistoryValidation(t *testing.T) {
	_, c := newFeatureTest(t)
	if _, err := c.GetHistory(context.Background(), "DEV001", HistoryQuery{Start: time.Now().Add(-time.Hour)}); err == nil {
		t.Error("expected error for missing series")
	}
	if _, err := c.GetHistory(context.Background(), "DEV001", HistoryQuery{Series: SeriesInletTemp, Start: time.Now().Add(time.Hour)}); err == nil {
		t.Error("expected error for start after end")
	}
}

func TestAutoResolution(t *testing.T) {
	tests := []struct {
		span time.Duration
		want HistoryResolution
	}{
		{24 * time.Hour, Resolution5Min},
		{7 * 24 * time.Hour, ResolutionHourly},
		{90 * 24 * time.Hour, ResolutionDaily},
		{3 * 365 * 24 * time.Hour, ResolutionMonthly},
	}
	for _, tt := range tests {
		if got := autoResolution(tt.span); got != tt.want {
			t.Errorf("autoResolution(%v) = %q, want %q", tt.span, got, tt.want)
		}
	}
}

func TestParseHistorySeriesAndResolution(t *testing.T) {
	if s, err := ParseHistorySeries("Ambient"); err != nil || s != SeriesAmbientTemp {
		t.Errorf("ParseHistorySeries(Ambient) = %q, %v", s, err)
	}
	if s, err := ParseHistorySeries("2055"); err != nil || s != "2055" {
		t.Errorf("ParseHistorySeries(2055) = %q, %v", s, err)
	}
	if _, err := ParseHistorySeries("bogus"); err == nil {
		t.Error("ParseHistorySeries(bogus) expected error")
	}
	if r, err := ParseHistoryResolution("hourly"); err != nil || r != ResolutionHourly {
		t.Errorf("ParseHistoryResolution(hourly) = %q, %v", r, err)
	}
	if _, err := ParseHistoryResolution("fortnight"); err == nil {
		t.Error("ParseHistoryResolution(fortnight) expected error")
	}
}

func TestGetParameters(t *testing.T) {
	server, c := newFeatureTest(t)
	server.StatusParams = []map[string]any{
		{"num": "T02", "standardNum": "InletWaterTemp", "content": "Inlet water Temp.", "dataValue": "28.5",
			"showUnit": "℃", "tranferUnit": "TEMP", "address": "2046", "description": "-30-100℃"},
		{"num": "2059", "standardNum": "undefined", "content": "undefined", "dataValue": "0"},
	}
	server.SettingsParams = []map[string]any{
		{"num": "R02", "content": "Heat target Temp.", "value": "32.20", "tranferUnit": "TEMP"},
	}
	ctx := context.Background()

	params, err := c.GetParameters(ctx, "DEV001", ParametersStatus)
	if err != nil {
		t.Fatalf("GetParameters(status) error = %v", err)
	}
	if len(params) != 2 {
		t.Fatalf("status params = %+v", params)
	}
	p := params[0]
	if p.Code != "T02" || p.SemanticCode != "InletWaterTemp" || p.Value != "28.5" || p.Address != "2046" || p.Unit != "℃" {
		t.Errorf("params[0] = %+v", p)
	}
	if params[1].SemanticCode != "" {
		t.Errorf("\"undefined\" semantic code should be blanked, got %q", params[1].SemanticCode)
	}

	if _, err := c.GetParameters(ctx, "DEV001", ParametersStatus); err != nil {
		t.Fatal(err)
	}
	if server.ProductInfoCalls != 1 {
		t.Errorf("ProductInfoCalls = %d, want 1 (protocol id cached)", server.ProductInfoCalls)
	}

	settings, err := c.GetParameters(ctx, "DEV001", ParametersSettings)
	if err != nil {
		t.Fatal(err)
	}
	if len(settings) != 1 || settings[0].Code != "R02" || settings[0].Value != "32.20" {
		t.Errorf("settings = %+v", settings)
	}

	if _, err := c.GetParameters(ctx, "DEV001", ParameterKind("bogus")); err == nil {
		t.Error("expected error for unknown kind")
	}
}

func TestLoginIsNoOpWhenAuthenticated(t *testing.T) {
	server, c := newFeatureTest(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := c.Login(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if server.LoginCount != 1 {
		t.Errorf("LoginCount = %d, want 1", server.LoginCount)
	}
}

func TestWaterTempsInvalidWithoutFlow(t *testing.T) {
	server, c := newFeatureTest(t)
	params := append([]testutil.Parameter(nil), heatingParams...)
	for i := range params {
		if params[i].Code == "WaterFlowSwitch" {
			params[i].Value = "1" // no flow
		}
	}
	server.DeviceData["DEV001"] = params

	data, err := c.GetDeviceData(context.Background(), "DEV001")
	if err != nil {
		t.Fatal(err)
	}
	if data.WaterTempStatus != WaterTempNoFlow || data.WaterTempsValid() {
		t.Errorf("status = %q, valid = %v; want no_flow, false", data.WaterTempStatus, data.WaterTempsValid())
	}
	// The reported numbers are kept for reference...
	if data.InletTemp != 28.5 || data.OutletTemp != 30.5 || data.CurrentTemp != 28.5 {
		t.Errorf("raw temps = %v/%v/%v, want 28.5/30.5/28.5", data.InletTemp, data.OutletTemp, data.CurrentTemp)
	}
	// ...but the checked accessors refuse them.
	if data.ValidInletTemp() != nil || data.ValidOutletTemp() != nil || data.ValidCurrentTemp() != nil {
		t.Error("Valid*Temp() should be nil with no flow")
	}
	// Non-water readings are unaffected.
	if data.AmbientTemp != 26 {
		t.Errorf("AmbientTemp = %v, want 26", data.AmbientTemp)
	}
}

func TestWaterTempStatusFromFlow(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		flow  *bool
		want  WaterTempStatus
		valid bool
	}{
		{&yes, WaterTempValid, true},
		{&no, WaterTempNoFlow, false},
		{nil, WaterTempUnknown, true},
	}
	for _, tt := range tests {
		d := DeviceData{InletTemp: 20, WaterTempStatus: WaterTempStatusFromFlow(tt.flow)}
		if d.WaterTempStatus != tt.want || d.WaterTempsValid() != tt.valid {
			t.Errorf("flow %v: status %q valid %v, want %q %v", tt.flow, d.WaterTempStatus, d.WaterTempsValid(), tt.want, tt.valid)
		}
		if got := d.ValidInletTemp(); (got != nil) != tt.valid || (got != nil && *got != 20) {
			t.Errorf("flow %v: ValidInletTemp() = %v", tt.flow, got)
		}
	}
}
