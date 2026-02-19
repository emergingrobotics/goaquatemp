package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
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

func requireDeviceCode(client goaquatemp.Client) (string, error) {
	code := getDeviceCode(client)
	if code == "" {
		return "", fmt.Errorf("device code required: use --device flag or set AQUATEMP_DEFAULT_DEVICE")
	}
	return code, nil
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
		Short: "Poll temperature data over time",
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
		fmt.Println("timestamp,target,current,inlet,outlet,ambient")
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
				"current":   data.CurrentTemp,
				"inlet":     data.InletTemp,
				"outlet":    data.OutletTemp,
				"ambient":   data.AmbientTemp,
			})
		} else if outputCSV {
			fmt.Printf("%s,%s,%s,%s,%s,%s\n",
				now,
				tempStr(data.TargetTemp),
				tempStr(data.CurrentTemp),
				tempStr(data.InletTemp),
				tempStr(data.OutletTemp),
				tempStr(data.AmbientTemp),
			)
		} else {
			fmt.Printf("[%s] Target: %s°C  Current: %s°C  Inlet: %s°C  Outlet: %s°C  Ambient: %s°C\n",
				now,
				tempStr(data.TargetTemp),
				tempStr(data.CurrentTemp),
				tempStr(data.InletTemp),
				tempStr(data.OutletTemp),
				tempStr(data.AmbientTemp),
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

// status command
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show full device status",
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

	data, err := client.GetDeviceData(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to get device data: %w", err)
	}

	if outputJSON {
		return printJSON(map[string]any{
			"device": device,
			"data":   data,
		})
	}

	return printStatus(device, data)
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
	var targetTemp float64
	setTempCmd := &cobra.Command{
		Use:   "set-temp",
		Short: "Set target temperature",
		RunE: func(cmd *cobra.Command, args []string) error {
			if targetTemp == 0 {
				return fmt.Errorf("specify temperature with --temp")
			}
			return runControlTemp(targetTemp)
		},
	}
	setTempCmd.Flags().Float64VarP(&targetTemp, "temp", "t", 0, "Target temperature in Celsius")

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
			case "medium":
				speed = goaquatemp.FanMedium
			case "high":
				speed = goaquatemp.FanHigh
			case "auto":
				speed = goaquatemp.FanAuto
			default:
				return fmt.Errorf("invalid fan speed %q: use low, medium, high, or auto", fanSpeed)
			}
			return runControlFanSpeed(speed)
		},
	}
	fanSpeedCmd.Flags().StringVar(&fanSpeed, "speed", "", "Fan speed: low, medium, high, auto")
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
