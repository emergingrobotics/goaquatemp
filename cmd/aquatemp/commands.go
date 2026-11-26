package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/goaquatemp"
)

func getClient() (goaquatemp.Client, error) {
	opts := []goaquatemp.Option{
		goaquatemp.WithEnvConfig(),
	}

	if debug {
		opts = append(opts, goaquatemp.WithDebug(true))
	}

	return goaquatemp.NewClient(opts...)
}

func getDeviceCode(client goaquatemp.Client) string {
	if deviceCode != "" {
		return deviceCode
	}
	if env := os.Getenv("AQUATEMP_DEFAULT_DEVICE"); env != "" {
		return env
	}
	if client.Config().DefaultDevice != "" {
		return client.Config().DefaultDevice
	}
	return ""
}

// requireDeviceCode returns the configured device code or, when none is
// configured, the account's first device.
func requireDeviceCode(client goaquatemp.Client) (string, error) {
	if code := getDeviceCode(client); code != "" {
		return code, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	devices, err := client.GetDevices(ctx)
	if err != nil {
		return "", fmt.Errorf("no --device given and listing devices failed: %w", err)
	}
	if len(devices) == 0 {
		return "", fmt.Errorf("no devices found on this account: use --device or set AQUATEMP_DEFAULT_DEVICE")
	}
	return devices[0].Code, nil
}

// login command
func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the Aqua Temp API",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getClient()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := client.Login(ctx); err != nil {
				return fmt.Errorf("login failed: %w", err)
			}

			fmt.Println("Login successful")
			return nil
		},
	}
}

// devices command
func newDevicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "List and manage devices",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all devices",
		RunE:  runDevicesList,
	}

	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Show device information",
		RunE:  runDevicesInfo,
	}

	cmd.AddCommand(listCmd, infoCmd)
	return cmd
}

func runDevicesList(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Login(ctx); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	devices, err := client.GetDevices(ctx)
	if err != nil {
		return fmt.Errorf("failed to get devices: %w", err)
	}

	if outputJSON {
		return printJSON(devices)
	}

	if outputCSV {
		return printDevicesCSV(devices)
	}

	return printDevicesTable(devices)
}

func runDevicesInfo(cmd *cobra.Command, args []string) error {
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

	if outputJSON {
		return printJSON(device)
	}

	return printDeviceInfo(device)
}

// temp command
func newTempCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "temp",
		Short: "Get temperature data",
	}

	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get current temperature data",
		RunE:  runTempGet,
	}

	var hours int
	var interval int
	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "Poll live temperature data over time (see 'aquatemp history' for recorded data)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTempHistory(hours, interval)
		},
	}
	historyCmd.Flags().IntVar(&hours, "hours", 1, "Duration to poll in hours")
	historyCmd.Flags().IntVar(&interval, "interval", 300, "Poll interval in seconds")

	cmd.AddCommand(getCmd, historyCmd)
	return cmd
}

func runTempGet(cmd *cobra.Command, args []string) error {
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

	data, err := client.GetDeviceData(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to get temperature data: %w", err)
	}

	if outputJSON {
		return printJSON(data)
	}

	if outputCSV {
		return printTempCSV(data)
	}

	return printTempTable(data)
}

func runTempHistory(hours, interval int) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	code, err := requireDeviceCode(client)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(hours)*time.Hour+time.Minute)
	defer cancel()

	if err := client.Login(ctx); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	endTime := time.Now().Add(time.Duration(hours) * time.Hour)
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	if outputCSV {
		fmt.Println("timestamp,target_c,target_f,current_c,current_f,inlet_c,inlet_f,outlet_c,outlet_f,ambient_c,ambient_f," +
			"water_temp_status,current_raw_c,inlet_raw_c,outlet_raw_c")
	}

	readAndPrint := func() {
		data, err := client.GetDeviceData(ctx, code)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return
		}
		now := time.Now().Format("15:04:05")
		if outputJSON {
			printJSON(map[string]any{
				"timestamp": now,
				"target":    data.TargetTemp,
				"current":   waterTempJSON(data, data.CurrentTemp),
				"inlet":     waterTempJSON(data, data.InletTemp),
				"outlet":    waterTempJSON(data, data.OutletTemp),
				"ambient":   data.AmbientTemp,
				"target_f":  goaquatemp.CelsiusToFahrenheit(data.TargetTemp),
				"current_f": waterTempJSON(data, goaquatemp.CelsiusToFahrenheit(data.CurrentTemp)),
				"inlet_f":   waterTempJSON(data, goaquatemp.CelsiusToFahrenheit(data.InletTemp)),
				"outlet_f":  waterTempJSON(data, goaquatemp.CelsiusToFahrenheit(data.OutletTemp)),
				"ambient_f": goaquatemp.CelsiusToFahrenheit(data.AmbientTemp),

				"water_temp_status": data.WaterTempStatus,
				"current_raw":       data.CurrentTemp,
				"inlet_raw":         data.InletTemp,
				"outlet_raw":        data.OutletTemp,
			})
		} else if outputCSV {
			row := []string{now, tempStr(data.TargetTemp), tempFStr(data.TargetTemp)}
			for _, c := range []float64{data.CurrentTemp, data.InletTemp, data.OutletTemp} {
				row = append(row, waterTempStr(data, c), waterTempFStr(data, c))
			}
			row = append(row, tempStr(data.AmbientTemp), tempFStr(data.AmbientTemp),
				string(data.WaterTempStatus),
				tempStr(data.CurrentTemp), tempStr(data.InletTemp), tempStr(data.OutletTemp))
			fmt.Println(strings.Join(row, ","))
		} else {
			fmt.Printf("[%s] Target: %s  Current: %s  Inlet: %s  Outlet: %s  Ambient: %s\n",
				now,
				bothTemps(data.TargetTemp),
				waterTemps(data, data.CurrentTemp),
				waterTemps(data, data.InletTemp),
				waterTemps(data, data.OutletTemp),
				bothTemps(data.AmbientTemp),
			)
		}
	}

	readAndPrint()

	for {
		select {
		case <-ctx.Done():
			return nil
		case t := <-ticker.C:
			if t.After(endTime) {
				return nil
			}
			readAndPrint()
		}
	}
}

// control command
func newControlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "control",
		Short: "Control device settings",
	}

	// power subcommand
	var powerOn, powerOff bool
	powerCmd := &cobra.Command{
		Use:   "power",
		Short: "Turn device on or off",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !powerOn && !powerOff {
				return fmt.Errorf("specify --on or --off")
			}
			return runControlPower(powerOn)
		},
	}
	powerCmd.Flags().BoolVar(&powerOn, "on", false, "Turn device on")
	powerCmd.Flags().BoolVar(&powerOff, "off", false, "Turn device off")

	// set-temp subcommand
	var targetTemp, targetTempF float64
	setTempCmd := &cobra.Command{
		Use:   "set-temp",
		Short: "Set target temperature",
		Long: `Set the target water temperature for the current mode.

Give the temperature in Celsius with --temp or in Fahrenheit with --temp-f.
Fahrenheit is converted to Celsius and rounded to 0.1°C before sending.`,
		Example: `  aquatemp control set-temp --temp 32
  aquatemp control set-temp --temp-f 90`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := setTempCelsius(cmd.Flags().Changed("temp"), targetTemp,
				cmd.Flags().Changed("temp-f"), targetTempF)
			if err != nil {
				return err
			}
			return runControlTemp(c)
		},
	}
	setTempCmd.Flags().Float64VarP(&targetTemp, "temp", "t", 0, "Target temperature in Celsius")
	setTempCmd.Flags().Float64Var(&targetTempF, "temp-f", 0, "Target temperature in Fahrenheit")
	setTempCmd.MarkFlagsMutuallyExclusive("temp", "temp-f")

	// mode subcommand
	var modeHeat, modeCool, modeAuto, modeOff bool
	modeCmd := &cobra.Command{
		Use:   "mode",
		Short: "Set operation mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			var mode goaquatemp.OperationMode
			switch {
			case modeHeat:
				mode = goaquatemp.ModeHeat
			case modeCool:
				mode = goaquatemp.ModeCool
			case modeAuto:
				mode = goaquatemp.ModeAuto
			case modeOff:
				mode = goaquatemp.ModeOff
			default:
				return fmt.Errorf("specify --heat, --cool, --auto, or --off")
			}
			return runControlMode(mode)
		},
	}
	modeCmd.Flags().BoolVar(&modeHeat, "heat", false, "Set heat mode")
	modeCmd.Flags().BoolVar(&modeCool, "cool", false, "Set cool mode")
	modeCmd.Flags().BoolVar(&modeAuto, "auto", false, "Set auto mode")
	modeCmd.Flags().BoolVar(&modeOff, "off", false, "Turn off")

	// fan-speed subcommand
	var fanSpeed string
	fanSpeedCmd := &cobra.Command{
		Use:   "fan-speed",
		Short: "Set fan speed",
		RunE: func(cmd *cobra.Command, args []string) error {
			var speed goaquatemp.FanSpeed
			switch fanSpeed {
			case "low":
				speed = goaquatemp.FanLow
			case "auto":
				speed = goaquatemp.FanAuto
			default:
				return fmt.Errorf("invalid fan speed %q: use auto or low (silent)", fanSpeed)
			}
			return runControlFanSpeed(speed)
		},
	}
	fanSpeedCmd.Flags().StringVar(&fanSpeed, "speed", "", "Fan speed: auto or low (silent)")
	fanSpeedCmd.MarkFlagRequired("speed")

	cmd.AddCommand(powerCmd, setTempCmd, modeCmd, fanSpeedCmd)
	return cmd
}

func runControlPower(on bool) error {
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

	if err := client.SetPower(ctx, code, on); err != nil {
		return fmt.Errorf("failed to set power: %w", err)
	}

	state := "off"
	if on {
		state = "on"
	}
	fmt.Printf("Device power set to %s\n", state)
	return nil
}

func runControlTemp(temp float64) error {
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

	if err := client.SetTemperature(ctx, code, temp); err != nil {
		return fmt.Errorf("failed to set temperature: %w", err)
	}

	fmt.Printf("Target temperature set to %.1f°C\n", temp)
	return nil
}

func runControlMode(mode goaquatemp.OperationMode) error {
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

	if err := client.SetMode(ctx, code, mode); err != nil {
		return fmt.Errorf("failed to set mode: %w", err)
	}

	fmt.Printf("Operation mode set to %s\n", mode)
	return nil
}

func runControlFanSpeed(speed goaquatemp.FanSpeed) error {
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

	if err := client.SetFanSpeed(ctx, code, speed); err != nil {
		return fmt.Errorf("failed to set fan speed: %w", err)
	}

	fmt.Printf("Fan speed set to %s\n", speed)
	return nil
}

// Helper to convert float to string for display
func tempStr(temp float64) string {
	return strconv.FormatFloat(temp, 'f', 1, 64)
}

// setTempCelsius resolves the set-temp flags to a Celsius target, converting
// --temp-f and rounding it to 0.1°C.
func setTempCelsius(haveC bool, c float64, haveF bool, f float64) (float64, error) {
	switch {
	case haveC && haveF:
		return 0, fmt.Errorf("use either --temp or --temp-f, not both")
	case haveC:
		return c, nil
	case haveF:
		return math.Round(goaquatemp.FahrenheitToCelsius(f)*10) / 10, nil
	}
	return 0, fmt.Errorf("specify the temperature with --temp (°C) or --temp-f (°F)")
}

// tempFStr formats a °C value converted to °F, to one decimal place.
func tempFStr(c float64) string {
	return tempStr(goaquatemp.CelsiusToFahrenheit(c))
}

// bothTemps formats a °C value as "28.5°C / 83.3°F".
func bothTemps(c float64) string {
	return tempStr(c) + "°C / " + tempFStr(c) + "°F"
}

// naValue is printed in place of a water temperature that is not valid
// because there is no water flow.
const naValue = "NA"

// waterTemps formats a water temperature (current, inlet, outlet). With no
// flow it prints NA and shows the reading only as a stagnant value.
func waterTemps(d *goaquatemp.DeviceData, c float64) string {
	if d.WaterTempsValid() {
		return bothTemps(c)
	}
	return naValue + " (no flow; stagnant reading " + bothTemps(c) + ", not valid)"
}

// waterTempStr is tempStr for a water temperature: NA when there is no flow.
func waterTempStr(d *goaquatemp.DeviceData, c float64) string {
	if !d.WaterTempsValid() {
		return naValue
	}
	return tempStr(c)
}

// waterTempFStr is tempFStr for a water temperature: NA when there is no flow.
func waterTempFStr(d *goaquatemp.DeviceData, c float64) string {
	if !d.WaterTempsValid() {
		return naValue
	}
	return tempFStr(c)
}

// waterTempJSON is a water temperature for JSON output: null when there is
// no flow.
func waterTempJSON(d *goaquatemp.DeviceData, c float64) any {
	if !d.WaterTempsValid() {
		return nil
	}
	return c
}

// noFlowWarning is printed under human-readable output when the water
// temperatures are stagnant.
const noFlowWarning = "Warning: no water flow. Water temperatures are of stagnant water in the unit, not the pool; do not use them."
