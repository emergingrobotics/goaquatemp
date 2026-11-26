// Example program that connects to the Aqua Temp API and retrieves
// temperature data for the current day.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/user/goaquatemp"
)

func main() {
	// Create client with configuration from environment variables
	client, err := goaquatemp.NewClient(
		goaquatemp.WithUsername(os.Getenv("AQUATEMP_USERNAME")),
		goaquatemp.WithPassword(os.Getenv("AQUATEMP_PASSWORD")),
		goaquatemp.WithAPIVersion(goaquatemp.APIModern),
		goaquatemp.WithTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// Authenticate
	fmt.Println("Authenticating...")
	if err := client.Login(ctx); err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	defer client.Logout(ctx)
	fmt.Println("Authentication successful!")

	// Get list of devices
	fmt.Println("\nFetching devices...")
	devices, err := client.GetDevices(ctx)
	if err != nil {
		log.Fatalf("Failed to get devices: %v", err)
	}

	if len(devices) == 0 {
		log.Fatal("No devices found")
	}

	fmt.Printf("Found %d device(s)\n", len(devices))

	// Get temperature data for first device (or specified device)
	deviceCode := os.Getenv("AQUATEMP_DEFAULT_DEVICE")
	var device goaquatemp.Device

	if deviceCode != "" {
		// Find the specified device
		for _, d := range devices {
			if d.Code == deviceCode {
				device = d
				break
			}
		}
		if device.Code == "" {
			log.Fatalf("Device %s not found", deviceCode)
		}
	} else {
		// Use first device
		device = devices[0]
	}

	fmt.Printf("\nGetting temperature data for: %s (%s)\n", device.Name, device.Code)

	data, err := client.GetDeviceData(ctx, device.Code)
	if err != nil {
		log.Fatalf("Failed to get device data: %v", err)
	}

	// Display temperature data
	fmt.Println()
	fmt.Println("=" + repeatStr("=", 39))
	fmt.Printf(" Temperature Data - %s\n", time.Now().Format("2006-01-02"))
	fmt.Println("=" + repeatStr("=", 39))
	fmt.Println()

	fmt.Printf(" Device:       %s\n", device.Name)
	fmt.Printf(" Device Code:  %s\n", device.Code)
	fmt.Printf(" Timestamp:    %s\n", data.Timestamp.Format(time.RFC3339))
	fmt.Println()

	// Status
	fmt.Println(" Status")
	fmt.Println(" " + repeatStr("-", 25))
	fmt.Printf(" Power:        %s\n", formatPower(data.Power))
	fmt.Printf(" Mode:         %s\n", data.Mode)
	fmt.Printf(" Fan Speed:    %s\n", data.FanSpeed)
	fmt.Println()

	// Temperatures
	fmt.Println(" Temperatures")
	fmt.Println(" " + repeatStr("-", 25))
	fmt.Printf(" Target:       %.1f°C / %.1f°F\n", data.TargetTemp, goaquatemp.CelsiusToFahrenheit(data.TargetTemp))
	// With no water flow the water sensors read stagnant water: show NA.
	waterTemp := func(c float64) string {
		s := fmt.Sprintf("%.1f°C / %.1f°F", c, goaquatemp.CelsiusToFahrenheit(c))
		if !data.WaterTempsValid() {
			return "NA (no flow; stagnant " + s + ")"
		}
		return s
	}
	fmt.Printf(" Current:      %s\n", waterTemp(data.CurrentTemp))
	fmt.Printf(" Inlet:        %s\n", waterTemp(data.InletTemp))
	fmt.Printf(" Outlet:       %s\n", waterTemp(data.OutletTemp))
	fmt.Printf(" Ambient:      %.1f°C / %.1f°F\n", data.AmbientTemp, goaquatemp.CelsiusToFahrenheit(data.AmbientTemp))
	fmt.Printf(" Coil:         %.1f°C / %.1f°F\n", data.CoilTemp, goaquatemp.CelsiusToFahrenheit(data.CoilTemp))
	fmt.Println()

	// Delta calculation
	if data.WaterTempsValid() && data.InletTemp > 0 && data.OutletTemp > 0 {
		delta := data.OutletTemp - data.InletTemp
		fmt.Printf(" Delta (Out-In): %.1f°C / %.1f°F\n", delta, goaquatemp.CelsiusDeltaToFahrenheit(delta))
		fmt.Println()
	}

	fmt.Println("=" + repeatStr("=", 39))
}

func formatPower(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}

func repeatStr(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
