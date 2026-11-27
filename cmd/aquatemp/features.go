package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/emergingrobotics/goaquatemp"
)

// status command

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show full device status: power, mode, operating state, compressor, temperatures, faults",
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}
	code, err := requireDeviceCode(client)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Login(ctx); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	device, err := client.GetDevice(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to get device: %w", err)
	}
	status, err := client.GetDeviceStatus(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to get device status: %w", err)
	}
	data, err := client.GetDeviceData(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to get device data: %w", err)
	}

	if outputJSON {
		return printJSON(map[string]any{"device": device, "status": status, "data": data, "fahrenheit": fahrenheitValues(data)})
	}
	if outputCSV {
		return printStatusCSV(device, status, data)
	}
	printStatus(device, status, data)
	return nil
}

func onOff(b bool) string {
	if b {
		return "ON"
	}
	return "OFF"
}

func optTemp(v *float64) string {
	if v == nil {
		return "-"
	}
	return bothTemps(*v)
}

// fahrenheitValues returns the °F equivalents of the °C readings, for the
// status JSON output.
func fahrenheitValues(d *goaquatemp.DeviceData) map[string]any {
	f := goaquatemp.CelsiusToFahrenheit
	opt := func(v *float64) any {
		if v == nil {
			return nil
		}
		return f(*v)
	}
	// Water temperatures are null with no flow (see data.waterTempStatus).
	return map[string]any{
		"targetTemp":  f(d.TargetTemp),
		"currentTemp": waterTempJSON(d, f(d.CurrentTemp)),
		"inletTemp":   waterTempJSON(d, f(d.InletTemp)),
		"outletTemp":  waterTempJSON(d, f(d.OutletTemp)),
		"ambientTemp": f(d.AmbientTemp),
		"coilTemp":    f(d.CoilTemp),
		"suctionTemp": opt(d.SuctionTemp),
		"exhaustTemp": opt(d.ExhaustTemp),
		"targetMin":   opt(d.TargetMin),
		"targetMax":   opt(d.TargetMax),
	}
}

func compressorText(d *goaquatemp.DeviceData) string {
	if d.CompressorRunning == nil {
		return "unknown"
	}
	text := "stopped"
	if *d.CompressorRunning {
		text = "running"
	}
	if d.CompressorFrequency != nil && *d.CompressorFrequency > 0 {
		text += fmt.Sprintf(", %s Hz", strconv.FormatFloat(*d.CompressorFrequency, 'f', -1, 64))
		if d.CompressorSpeed != nil && d.CompressorMaxFrequency != nil {
			text += fmt.Sprintf(" (%s%% of %s Hz max)",
				strconv.FormatFloat(*d.CompressorSpeed, 'f', -1, 64),
				strconv.FormatFloat(*d.CompressorMaxFrequency, 'f', -1, 64))
		}
	}
	if d.CompressorCurrent != nil && *d.CompressorCurrent > 0 {
		text += fmt.Sprintf(", %s A", strconv.FormatFloat(*d.CompressorCurrent, 'f', -1, 64))
	}
	return text
}

// electricalText shows the inverter input voltage, the V x A compressor
// estimate and the raw power/energy counters (both unverified).
func electricalText(d *goaquatemp.DeviceData) string {
	num := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	var parts []string
	if d.InverterInputVoltage != nil {
		parts = append(parts, num(*d.InverterInputVoltage)+" V in")
		if d.CompressorCurrent != nil {
			parts = append(parts, fmt.Sprintf("~%.0f W compressor (V x A)", *d.InverterInputVoltage**d.CompressorCurrent))
		}
	}
	if d.RealPowerRaw != nil {
		parts = append(parts, "Z04 "+num(*d.RealPowerRaw)+" (unit unverified)")
	}
	if d.EnergyTotal != nil {
		parts = append(parts, "Z06 "+num(*d.EnergyTotal)+" (probably kWh)")
	}
	return strings.Join(parts, ", ")
}

func waterFlowText(d *goaquatemp.DeviceData) string {
	if d.WaterFlowing == nil {
		return "unknown"
	}
	if *d.WaterFlowing {
		return "flowing"
	}
	return "no flow"
}

func faultText(st *goaquatemp.DeviceStatus, d *goaquatemp.DeviceData) string {
	var parts []string
	for _, f := range st.Faults {
		parts = append(parts, f.Description)
	}
	if st.Fault && len(st.Faults) == 0 {
		parts = append(parts, "fault reported (no details)")
	}
	for _, s := range d.SensorFaults {
		parts = append(parts, "sensor fault: "+s)
	}
	if d.ExternalStop != nil && *d.ExternalStop {
		parts = append(parts, "held off by external stop input")
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, "; ")
}

func printStatus(device *goaquatemp.Device, st *goaquatemp.DeviceStatus, d *goaquatemp.DeviceData) {
	connection := "Online"
	if !st.Online {
		connection = "Offline (" + st.Status + ")"
	}

	fmt.Printf("Device: %s (%s)\n", device.Name, device.Code)
	fmt.Println(strings.Repeat("=", 48))
	fmt.Printf("Connection:  %s\n", connection)
	fmt.Printf("Power:       %s\n", onOff(d.Power))
	fmt.Printf("Mode:        %s\n", d.Mode)
	fmt.Printf("State:       %s\n", d.State)
	fmt.Printf("Compressor:  %s\n", compressorText(d))
	if e := electricalText(d); e != "" {
		fmt.Printf("Electrical:  %s\n", e)
	}
	fmt.Printf("Water:       %s\n", waterFlowText(d))
	fmt.Printf("Fan:         %s\n", d.FanSpeed)
	fmt.Printf("Faults:      %s\n", faultText(st, d))

	target := bothTemps(d.TargetTemp)
	if d.TargetMin != nil && d.TargetMax != nil {
		target += fmt.Sprintf("  (range %s-%s°C / %s-%s°F)",
			tempStr(*d.TargetMin), tempStr(*d.TargetMax),
			tempFStr(*d.TargetMin), tempFStr(*d.TargetMax))
	}
	fmt.Printf("\nTemperatures\n")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("Target:      %s\n", target)
	fmt.Printf("Inlet:       %s\n", waterTemps(d, d.InletTemp))
	fmt.Printf("Outlet:      %s\n", waterTemps(d, d.OutletTemp))
	fmt.Printf("Ambient:     %s\n", bothTemps(d.AmbientTemp))
	fmt.Printf("Coil:        %s\n", bothTemps(d.CoilTemp))
	fmt.Printf("Suction:     %s\n", optTemp(d.SuctionTemp))
	fmt.Printf("Exhaust:     %s\n", optTemp(d.ExhaustTemp))

	if !d.WaterTempsValid() {
		fmt.Printf("\n%s\n", noFlowWarning)
	}

	fmt.Printf("\nTimestamp: %s\n", d.Timestamp.Format(time.RFC3339))
}

func printStatusCSV(device *goaquatemp.Device, st *goaquatemp.DeviceStatus, d *goaquatemp.DeviceData) error {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	opt := func(v *float64) string {
		if v == nil {
			return ""
		}
		return strconv.FormatFloat(*v, 'f', -1, 64)
	}
	optF := func(v *float64) string {
		if v == nil {
			return ""
		}
		return tempFStr(*v)
	}
	optB := func(v *bool) string {
		if v == nil {
			return ""
		}
		return strconv.FormatBool(*v)
	}

	rows := [][]string{
		{"field", "value"},
		{"device", device.Code},
		{"name", device.Name},
		{"online", strconv.FormatBool(st.Online)},
		{"fault", strconv.FormatBool(st.Fault)},
		{"power", strconv.FormatBool(d.Power)},
		{"mode", string(d.Mode)},
		{"state", string(d.State)},
		{"compressor_running", optB(d.CompressorRunning)},
		{"compressor_hz", opt(d.CompressorFrequency)},
		{"compressor_max_hz", opt(d.CompressorMaxFrequency)},
		{"compressor_pct", opt(d.CompressorSpeed)},
		{"compressor_a", opt(d.CompressorCurrent)},
		{"inverter_input_v", opt(d.InverterInputVoltage)},
		{"real_power_raw", opt(d.RealPowerRaw)},
		{"energy_total", opt(d.EnergyTotal)},
		{"water_flowing", optB(d.WaterFlowing)},
		{"water_temp_status", string(d.WaterTempStatus)},
		{"fan", string(d.FanSpeed)},
		{"target_c", tempStr(d.TargetTemp)},
		{"target_f", tempFStr(d.TargetTemp)},
		{"inlet_c", waterTempStr(d, d.InletTemp)},
		{"inlet_f", waterTempFStr(d, d.InletTemp)},
		{"inlet_raw_c", tempStr(d.InletTemp)},
		{"outlet_c", waterTempStr(d, d.OutletTemp)},
		{"outlet_f", waterTempFStr(d, d.OutletTemp)},
		{"outlet_raw_c", tempStr(d.OutletTemp)},
		{"ambient_c", tempStr(d.AmbientTemp)},
		{"ambient_f", tempFStr(d.AmbientTemp)},
		{"coil_c", tempStr(d.CoilTemp)},
		{"coil_f", tempFStr(d.CoilTemp)},
		{"suction_c", opt(d.SuctionTemp)},
		{"suction_f", optF(d.SuctionTemp)},
		{"exhaust_c", opt(d.ExhaustTemp)},
		{"exhaust_f", optF(d.ExhaustTemp)},
		{"timestamp", d.Timestamp.Format(time.RFC3339)},
	}
	return w.WriteAll(rows)
}

// history command

func newHistoryCmd() *cobra.Command {
	var (
		seriesFlag     string
		sinceFlag      string
		fromFlag       string
		toFlag         string
		resolutionFlag string
		keepGaps       bool
	)

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show recorded history (the data behind the app's charts)",
		Long: `Show the history the cloud records for the device: inlet, outlet and
ambient temperature and compressor frequency.

Resolution is picked from the span unless --resolution is given:
up to 2 days: 5 minutes; up to 31 days: hourly; up to 400 days: daily; else monthly.

Examples:
  aquatemp history                         # last 24h, all series
  aquatemp history --since 7d --series inlet,outlet
  aquatemp history --from "2026-11-01" --to "2026-11-15" --resolution day --csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			start, end, err := historyRange(sinceFlag, fromFlag, toFlag, time.Now())
			if err != nil {
				return err
			}
			res, err := goaquatemp.ParseHistoryResolution(resolutionFlag)
			if err != nil {
				return err
			}
			var series []goaquatemp.HistorySeries
			for _, name := range strings.Split(seriesFlag, ",") {
				s, err := goaquatemp.ParseHistorySeries(name)
				if err != nil {
					return err
				}
				series = append(series, s)
			}
			return runHistory(series, start, end, res, keepGaps)
		},
	}

	cmd.Flags().StringVar(&seriesFlag, "series", "inlet,outlet,ambient,compressor",
		"Comma-separated series: inlet, outlet, ambient, compressor, or register addresses")
	cmd.Flags().StringVar(&sinceFlag, "since", "24h", "How far back to go (e.g. 6h, 7d); ignored with --from")
	cmd.Flags().StringVar(&fromFlag, "from", "", `Start time ("2006-01-02", "2006-01-02 15:04" or RFC 3339, local time)`)
	cmd.Flags().StringVar(&toFlag, "to", "", "End time (default now)")
	cmd.Flags().StringVar(&resolutionFlag, "resolution", "auto", "auto, 5m, hour, day or month")
	cmd.Flags().BoolVar(&keepGaps, "keep-gaps", false, "Keep the 0 values the cloud returns for missing samples")
	return cmd
}

// parseSince parses a duration, also accepting a day suffix ("7d").
func parseSince(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		days, err := strconv.ParseFloat(strings.TrimSuffix(s, "d"), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	return d, nil
}

// parseTimeArg parses a local time in one of a few layouts.
func parseTimeArg(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Local(), nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time %q: use 2006-01-02, 2006-01-02 15:04 or RFC 3339", s)
}

func historyRange(since, from, to string, now time.Time) (time.Time, time.Time, error) {
	end := now
	if to != "" {
		t, err := parseTimeArg(to)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		end = t
	}
	var start time.Time
	if from != "" {
		t, err := parseTimeArg(from)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		start = t
	} else {
		d, err := parseSince(since)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		if d <= 0 {
			return time.Time{}, time.Time{}, fmt.Errorf("--since must be positive")
		}
		start = end.Add(-d)
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, fmt.Errorf("start (%s) must be before end (%s)",
			start.Format(time.RFC3339), end.Format(time.RFC3339))
	}
	return start, end, nil
}

func runHistory(series []goaquatemp.HistorySeries, start, end time.Time, res goaquatemp.HistoryResolution, keepGaps bool) error {
	client, err := getClient()
	if err != nil {
		return err
	}
	code, err := requireDeviceCode(client)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := client.Login(ctx); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	var histories []*goaquatemp.History
	for _, s := range series {
		h, err := client.GetHistory(ctx, code, goaquatemp.HistoryQuery{
			Series: s, Start: start, End: end, Resolution: res, KeepGaps: keepGaps,
		})
		if errors.Is(err, goaquatemp.ErrHistoryUnavailable) {
			fmt.Fprintf(os.Stderr, "warning: no history recorded for series %s\n", s.Name())
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to get %s history: %w", s.Name(), err)
		}
		histories = append(histories, h)
	}

	if outputJSON {
		return printJSON(histories)
	}
	return printHistoryTable(histories, outputCSV)
}

// printHistoryTable prints the series side by side, one row per time.
func printHistoryTable(histories []*goaquatemp.History, asCSV bool) error {
	if len(histories) == 0 {
		fmt.Println("No history")
		return nil
	}

	values := make([]map[int64]float64, len(histories))
	var times []time.Time
	seen := make(map[int64]bool)
	for i, h := range histories {
		values[i] = make(map[int64]float64)
		for _, p := range h.Points {
			k := p.Time.Unix()
			values[i][k] = p.Value
			if !seen[k] {
				seen[k] = true
				times = append(times, p.Time)
			}
		}
	}
	sort.Slice(times, func(a, b int) bool { return times[a].Before(times[b]) })

	timeLayout := "2006-01-02 15:04"
	switch histories[0].Resolution {
	case goaquatemp.ResolutionDaily:
		timeLayout = "2006-01-02"
	case goaquatemp.ResolutionMonthly:
		timeLayout = "2006-01"
	}

	// Temperature series (°C) get a second °F column.
	isCelsius := func(h *goaquatemp.History) bool { return h.Unit == "°C" }

	header := []string{"time"}
	for _, h := range histories {
		col := h.Name
		if h.Unit != "" {
			col += " (" + h.Unit + ")"
		}
		header = append(header, col)
		if isCelsius(h) {
			header = append(header, h.Name+" (°F)")
		}
	}

	row := func(t time.Time) []string {
		r := []string{t.Format(timeLayout)}
		if asCSV {
			r[0] = t.Format(time.RFC3339)
		}
		for i, h := range histories {
			v, ok := values[i][t.Unix()]
			switch {
			case !ok:
				r = append(r, "")
				if isCelsius(h) {
					r = append(r, "")
				}
			case isCelsius(h):
				r = append(r, strconv.FormatFloat(v, 'f', -1, 64), tempFStr(v))
			default:
				r = append(r, strconv.FormatFloat(v, 'f', -1, 64))
			}
		}
		return r
	}

	if asCSV {
		w := csv.NewWriter(os.Stdout)
		defer w.Flush()
		w.Write(header)
		for _, t := range times {
			w.Write(row(t))
		}
		return nil
	}

	widths := make([]int, len(header))
	for i, h := range header {
		widths[i] = len([]rune(h))
	}
	if w := len(timeLayout); w > widths[0] {
		widths[0] = w
	}
	printRow := func(cells []string) {
		for i, c := range cells {
			if i == 0 {
				fmt.Printf("%-*s", widths[i], c)
			} else {
				fmt.Printf("  %*s", widths[i], c)
			}
		}
		fmt.Println()
	}
	printRow(header)
	total := 0
	for _, w := range widths {
		total += w + 2
	}
	fmt.Println(strings.Repeat("-", total-2))
	for _, t := range times {
		printRow(row(t))
	}
	return nil
}

// params command

func newParamsCmd() *cobra.Command {
	var kind, filter string
	cmd := &cobra.Command{
		Use:   "params",
		Short: "List the device's parameter catalog (codes, values, labels, history addresses)",
		RunE: func(cmd *cobra.Command, args []string) error {
			var k goaquatemp.ParameterKind
			switch kind {
			case "status":
				k = goaquatemp.ParametersStatus
			case "settings":
				k = goaquatemp.ParametersSettings
			default:
				return fmt.Errorf("invalid --kind %q: use status or settings", kind)
			}
			return runParams(k, filter)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "status", "status (read-only values) or settings (writable)")
	cmd.Flags().StringVar(&filter, "filter", "", "Only show parameters whose code or label contains this text")
	return cmd
}

func runParams(kind goaquatemp.ParameterKind, filter string) error {
	client, err := getClient()
	if err != nil {
		return err
	}
	code, err := requireDeviceCode(client)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Login(ctx); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	params, err := client.GetParameters(ctx, code, kind)
	if err != nil {
		return fmt.Errorf("failed to get parameters: %w", err)
	}
	params = filterParams(params, filter)

	if outputJSON {
		return printJSON(params)
	}

	if outputCSV {
		w := csv.NewWriter(os.Stdout)
		defer w.Flush()
		w.Write([]string{"code", "semantic_code", "value", "unit", "type", "address", "label", "description"})
		for _, p := range params {
			w.Write([]string{p.Code, p.SemanticCode, p.Value, p.Unit, p.Type, p.Address, p.Label, oneLine(p.Description)})
		}
		return nil
	}

	fmt.Printf("%-14s %-24s %-10s %-5s %-10s %s\n", "CODE", "SEMANTIC", "VALUE", "UNIT", "ADDRESS", "LABEL")
	fmt.Println(strings.Repeat("-", 100))
	for _, p := range params {
		label := p.Label
		if d := oneLine(p.Description); d != "" && d != p.Label {
			label += " [" + d + "]"
		}
		fmt.Printf("%-14s %-24s %-10s %-5s %-10s %s\n", p.Code, p.SemanticCode, p.Value, p.Unit, p.Address, label)
	}
	return nil
}

func filterParams(params []goaquatemp.Parameter, filter string) []goaquatemp.Parameter {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return params
	}
	var out []goaquatemp.Parameter
	for _, p := range params {
		hay := strings.ToLower(p.Code + " " + p.SemanticCode + " " + p.Label + " " + p.Description)
		if strings.Contains(hay, filter) {
			out = append(out, p)
		}
	}
	return out
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
