package main

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/emergingrobotics/goaquatemp"
)

func TestParseSince(t *testing.T) {
	tests := map[string]time.Duration{
		"24h":  24 * time.Hour,
		"90m":  90 * time.Minute,
		"7d":   7 * 24 * time.Hour,
		"1.5d": 36 * time.Hour,
	}
	for in, want := range tests {
		got, err := parseSince(in)
		if err != nil || got != want {
			t.Errorf("parseSince(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := parseSince("soon"); err == nil {
		t.Error("parseSince(soon) expected error")
	}
}

func TestParseTimeArg(t *testing.T) {
	for _, in := range []string{"2026-11-16", "2026-11-16 08:30", "2026-11-16 08:30:00", "2026-11-16T08:30", "2026-11-16T08:30:00Z"} {
		if _, err := parseTimeArg(in); err != nil {
			t.Errorf("parseTimeArg(%q) error = %v", in, err)
		}
	}
	got, _ := parseTimeArg("2026-11-16 08:30")
	if got.Hour() != 8 || got.Minute() != 30 || got.Location() != time.Local {
		t.Errorf("parseTimeArg local = %v", got)
	}
	if _, err := parseTimeArg("16/11/2026"); err == nil {
		t.Error("expected error for unsupported layout")
	}
}

func TestHistoryRange(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.Local)

	start, end, err := historyRange("6h", "", "", now)
	if err != nil || !end.Equal(now) || !start.Equal(now.Add(-6*time.Hour)) {
		t.Errorf("historyRange(6h) = %v..%v, %v", start, end, err)
	}

	start, end, err = historyRange("24h", "2026-11-01", "2026-11-15", now)
	if err != nil || start.Day() != 1 || end.Day() != 15 {
		t.Errorf("historyRange(from,to) = %v..%v, %v", start, end, err)
	}

	if _, _, err := historyRange("24h", "2026-11-15", "2026-11-01", now); err == nil {
		t.Error("expected error when from is after to")
	}
	if _, _, err := historyRange("-1h", "", "", now); err == nil {
		t.Error("expected error for negative --since")
	}
}

func TestFilterParams(t *testing.T) {
	params := []goaquatemp.Parameter{
		{Code: "T02", SemanticCode: "InletWaterTemp", Label: "Inlet water Temp."},
		{Code: "O07", SemanticCode: "Sys1CompWorkingFreq", Label: "Comp. output frequency"},
	}
	if got := filterParams(params, ""); len(got) != 2 {
		t.Errorf("empty filter = %d params", len(got))
	}
	if got := filterParams(params, "INLET"); len(got) != 1 || got[0].Code != "T02" {
		t.Errorf("filter inlet = %+v", got)
	}
	if got := filterParams(params, "o07"); len(got) != 1 || got[0].Code != "O07" {
		t.Errorf("filter o07 = %+v", got)
	}
}

func TestCompressorText(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	b := func(v bool) *bool { return &v }

	d := &goaquatemp.DeviceData{
		CompressorRunning: b(true), CompressorFrequency: f(70),
		CompressorSpeed: f(82.4), CompressorMaxFrequency: f(85), CompressorCurrent: f(6.9),
	}
	if got := compressorText(d); got != "running, 70 Hz (82.4% of 85 Hz max), 6.9 A" {
		t.Errorf("compressorText(running) = %q", got)
	}
	if got := compressorText(&goaquatemp.DeviceData{CompressorRunning: b(false), CompressorFrequency: f(0)}); got != "stopped" {
		t.Errorf("compressorText(stopped) = %q", got)
	}
	if got := compressorText(&goaquatemp.DeviceData{}); got != "unknown" {
		t.Errorf("compressorText(unknown) = %q", got)
	}
}

func TestFaultText(t *testing.T) {
	b := true
	st := &goaquatemp.DeviceStatus{Fault: true, Faults: []goaquatemp.Fault{{Description: "E01"}}}
	d := &goaquatemp.DeviceData{SensorFaults: []string{"ATSensorFault"}, ExternalStop: &b}
	got := faultText(st, d)
	for _, want := range []string{"E01", "ATSensorFault", "external stop"} {
		if !strings.Contains(got, want) {
			t.Errorf("faultText() = %q, missing %q", got, want)
		}
	}
	if got := faultText(&goaquatemp.DeviceStatus{}, &goaquatemp.DeviceData{}); got != "none" {
		t.Errorf("faultText(clean) = %q", got)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestPrintHistoryTableAlignsSeries(t *testing.T) {
	t0 := time.Date(2026, 11, 16, 13, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	histories := []*goaquatemp.History{
		{Name: "inlet", Unit: "°C", Resolution: goaquatemp.ResolutionHourly,
			Points: []goaquatemp.HistoryPoint{{Time: t0, Value: 27.5}, {Time: t1, Value: 28}}},
		{Name: "compressor", Unit: "Hz", Resolution: goaquatemp.ResolutionHourly,
			Points: []goaquatemp.HistoryPoint{{Time: t1, Value: 62}}},
	}

	out := captureStdout(t, func() { printHistoryTable(histories, true) })
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("csv lines = %q", lines)
	}
	if lines[0] != "time,inlet (°C),inlet (°F),compressor (Hz)" {
		t.Errorf("header = %q", lines[0])
	}
	if !strings.HasSuffix(lines[1], ",27.5,81.5,") || !strings.HasSuffix(lines[2], ",28,82.4,62") {
		t.Errorf("rows = %q", lines[1:])
	}
}

func TestNewCommandsRegistered(t *testing.T) {
	h := newHistoryCmd()
	for _, f := range []string{"series", "since", "from", "to", "resolution", "keep-gaps"} {
		if h.Flags().Lookup(f) == nil {
			t.Errorf("history missing --%s", f)
		}
	}
	p := newParamsCmd()
	for _, f := range []string{"kind", "filter"} {
		if p.Flags().Lookup(f) == nil {
			t.Errorf("params missing --%s", f)
		}
	}
}

func TestBothTemps(t *testing.T) {
	if got := bothTemps(28.5); got != "28.5°C / 83.3°F" {
		t.Errorf("bothTemps(28.5) = %q", got)
	}
	if got := bothTemps(0); got != "0.0°C / 32.0°F" {
		t.Errorf("bothTemps(0) = %q", got)
	}
	if got := optTemp(nil); got != "-" {
		t.Errorf("optTemp(nil) = %q", got)
	}
}

func TestStatusCSVHasFahrenheit(t *testing.T) {
	d := &goaquatemp.DeviceData{InletTemp: 100, TargetTemp: 30}
	out := captureStdout(t, func() {
		printStatusCSV(&goaquatemp.Device{Code: "D1"}, &goaquatemp.DeviceStatus{}, d)
	})
	for _, want := range []string{"inlet_c,100.0", "inlet_f,212.0", "target_f,86.0", "suction_f,\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("status CSV missing %q:\n%s", want, out)
		}
	}
}

func TestSetTempCelsius(t *testing.T) {
	if c, err := setTempCelsius(true, 32, false, 0); err != nil || c != 32 {
		t.Errorf("--temp 32 = %v, %v", c, err)
	}
	if c, err := setTempCelsius(false, 0, true, 90); err != nil || c != 32.2 {
		t.Errorf("--temp-f 90 = %v, %v; want 32.2", c, err)
	}
	if c, err := setTempCelsius(false, 0, true, 104); err != nil || c != 40 {
		t.Errorf("--temp-f 104 = %v, %v; want 40", c, err)
	}
	if _, err := setTempCelsius(false, 0, false, 0); err == nil {
		t.Error("expected error with no flag")
	}
	if _, err := setTempCelsius(true, 30, true, 90); err == nil {
		t.Error("expected error with both flags")
	}
}

func noFlowData() *goaquatemp.DeviceData {
	return &goaquatemp.DeviceData{
		InletTemp: 28.5, OutletTemp: 30.5, CurrentTemp: 28.5, AmbientTemp: 26, TargetTemp: 30,
		WaterTempStatus: goaquatemp.WaterTempNoFlow,
	}
}

func TestWaterTempsShowNAWithoutFlow(t *testing.T) {
	d := noFlowData()
	if got := waterTemps(d, d.InletTemp); !strings.HasPrefix(got, "NA ") || !strings.Contains(got, "28.5°C / 83.3°F") {
		t.Errorf("waterTemps(no flow) = %q, want NA plus the stagnant reading", got)
	}
	if got := waterTempStr(d, d.InletTemp); got != "NA" {
		t.Errorf("waterTempStr(no flow) = %q, want NA", got)
	}
	if got := waterTempJSON(d, d.InletTemp); got != nil {
		t.Errorf("waterTempJSON(no flow) = %v, want nil", got)
	}
	d.WaterTempStatus = goaquatemp.WaterTempValid
	if got := waterTemps(d, d.InletTemp); got != "28.5°C / 83.3°F" {
		t.Errorf("waterTemps(flowing) = %q", got)
	}
}

func TestStatusOutputNoFlow(t *testing.T) {
	d := noFlowData()
	out := captureStdout(t, func() {
		printStatusCSV(&goaquatemp.Device{Code: "D1"}, &goaquatemp.DeviceStatus{}, d)
	})
	for _, want := range []string{"water_temp_status,no_flow", "inlet_c,NA", "inlet_f,NA", "inlet_raw_c,28.5", "outlet_c,NA", "outlet_raw_c,30.5", "ambient_c,26.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("status CSV missing %q:\n%s", want, out)
		}
	}

	out = captureStdout(t, func() {
		printStatus(&goaquatemp.Device{Code: "D1"}, &goaquatemp.DeviceStatus{Online: true}, d)
	})
	if !strings.Contains(out, "Inlet:       NA (no flow") || !strings.Contains(out, "Warning: no water flow") {
		t.Errorf("status text should mark water temps NA:\n%s", out)
	}

	f := fahrenheitValues(d)
	if f["inletTemp"] != nil || f["ambientTemp"] == nil {
		t.Errorf("fahrenheit JSON = %v, want inletTemp null, ambientTemp set", f)
	}
}

func TestTempCSVNoFlow(t *testing.T) {
	out := captureStdout(t, func() { printTempCSV(noFlowData()) })
	for _, want := range []string{"inlet,NA,NA,28.5,83.3,no_flow", "ambient,26.0,78.8,26.0,78.8,\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("temp CSV missing %q:\n%s", want, out)
		}
	}
}
