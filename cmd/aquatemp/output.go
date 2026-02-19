package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/user/goaquatemp"
)

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printDevicesTable(devices []goaquatemp.Device) error {
	if len(devices) == 0 {
		fmt.Println("No devices found")
		return nil
	}

	// Calculate column widths
	codeWidth := 12
	nameWidth := 20
	for _, d := range devices {
		if len(d.Code) > codeWidth {
			codeWidth = len(d.Code)
		}
		if len(d.Name) > nameWidth {
			nameWidth = len(d.Name)
		}
	}

	// Header
	fmt.Printf("%-*s  %-*s  %-8s  %s\n", codeWidth, "CODE", nameWidth, "NAME", "STATUS", "TYPE")
	fmt.Println(strings.Repeat("-", codeWidth+nameWidth+25))

	// Rows
	for _, d := range devices {
		status := "Offline"
		if d.Online {
			status = "Online"
		}
		fmt.Printf("%-*s  %-*s  %-8s  %s\n", codeWidth, d.Code, nameWidth, d.Name, status, d.Type)
	}

	return nil
}

func printDevicesCSV(devices []goaquatemp.Device) error {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	w.Write([]string{"code", "name", "online", "type", "product_code", "created_at"})

	for _, d := range devices {
		online := "false"
		if d.Online {
			online = "true"
		}
		w.Write([]string{
			d.Code,
			d.Name,
			online,
			d.Type,
			d.ProductCode,
			d.CreatedAt.Format(time.RFC3339),
		})
	}

	return nil
}

func printDeviceInfo(d *goaquatemp.Device) error {
	status := "Offline"
	if d.Online {
		status = "Online"
	}

	fmt.Printf("Device: %s\n", d.Name)
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("Code:        %s\n", d.Code)
	fmt.Printf("Status:      %s\n", status)
	fmt.Printf("Type:        %s\n", d.Type)
	fmt.Printf("Product:     %s\n", d.ProductCode)
	fmt.Printf("Created:     %s\n", d.CreatedAt.Format("2006-01-02 15:04:05"))

	return nil
}

func printTempTable(data *goaquatemp.DeviceData) error {
	fmt.Printf("Temperature Data (%s)\n", data.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("Target:      %s°C\n", tempStr(data.TargetTemp))
	fmt.Printf("Current:     %s°C\n", tempStr(data.CurrentTemp))
	fmt.Printf("Inlet:       %s°C\n", tempStr(data.InletTemp))
	fmt.Printf("Outlet:      %s°C\n", tempStr(data.OutletTemp))
	fmt.Printf("Ambient:     %s°C\n", tempStr(data.AmbientTemp))
	fmt.Printf("Coil:        %s°C\n", tempStr(data.CoilTemp))

	return nil
}

func printTempCSV(data *goaquatemp.DeviceData) error {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	w.Write([]string{"metric", "value", "unit"})
	w.Write([]string{"target", tempStr(data.TargetTemp), "C"})
	w.Write([]string{"current", tempStr(data.CurrentTemp), "C"})
	w.Write([]string{"inlet", tempStr(data.InletTemp), "C"})
	w.Write([]string{"outlet", tempStr(data.OutletTemp), "C"})
	w.Write([]string{"ambient", tempStr(data.AmbientTemp), "C"})
	w.Write([]string{"coil", tempStr(data.CoilTemp), "C"})

	return nil
}

func printStatus(device *goaquatemp.Device, data *goaquatemp.DeviceData) error {
	status := "Offline"
	if device.Online {
		status = "Online"
	}

	power := "OFF"
	if data.Power {
		power = "ON"
	}

	fmt.Printf("Device: %s (%s)\n", device.Name, device.Code)
	fmt.Println(strings.Repeat("=", 40))

	fmt.Printf("\nStatus\n")
	fmt.Println(strings.Repeat("-", 20))
	fmt.Printf("Connection:  %s\n", status)
	fmt.Printf("Power:       %s\n", power)
	fmt.Printf("Mode:        %s\n", data.Mode)
	fmt.Printf("Fan Speed:   %s\n", data.FanSpeed)

	if data.ErrorCode != 0 {
		fmt.Printf("Error Code:  %d\n", data.ErrorCode)
	}

	fmt.Printf("\nTemperatures\n")
	fmt.Println(strings.Repeat("-", 20))
	fmt.Printf("Target:      %s°C\n", tempStr(data.TargetTemp))
	fmt.Printf("Current:     %s°C\n", tempStr(data.CurrentTemp))
	fmt.Printf("Inlet:       %s°C\n", tempStr(data.InletTemp))
	fmt.Printf("Outlet:      %s°C\n", tempStr(data.OutletTemp))
	fmt.Printf("Ambient:     %s°C\n", tempStr(data.AmbientTemp))
	fmt.Printf("Coil:        %s°C\n", tempStr(data.CoilTemp))

	fmt.Printf("\nTimestamp: %s\n", data.Timestamp.Format(time.RFC3339))

	return nil
}
