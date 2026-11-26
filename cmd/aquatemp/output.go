package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/emergingrobotics/goaquatemp"
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
	fmt.Printf("%-*s  %-*s  %-8s  %s\n", codeWidth, "CODE", nameWidth, "NAME", "STATUS", "PRODUCT")
	fmt.Println(strings.Repeat("-", codeWidth+nameWidth+25))

	// Rows
	for _, d := range devices {
		status := "Offline"
		if d.Online {
			status = "Online"
		}
		fmt.Printf("%-*s  %-*s  %-8s  %s\n", codeWidth, d.Code, nameWidth, d.Name, status, d.ProductCode)
	}

	return nil
}

func printDevicesCSV(devices []goaquatemp.Device) error {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	w.Write([]string{"code", "name", "online", "status", "product_code"})

	for _, d := range devices {
		online := "false"
		if d.Online {
			online = "true"
		}
		w.Write([]string{
			d.Code,
			d.Name,
			online,
			d.Status,
			d.ProductCode,
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
	fmt.Println(strings.Repeat("-", 36))
	fmt.Printf("Code:        %s\n", d.Code)
	fmt.Printf("Status:      %s\n", status)
	fmt.Printf("Product:     %s\n", d.ProductCode)

	return nil
}

func printTempTable(data *goaquatemp.DeviceData) error {
	fmt.Printf("Temperature Data (%s)\n", data.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("Target:      %s\n", bothTemps(data.TargetTemp))
	fmt.Printf("Current:     %s\n", waterTemps(data, data.CurrentTemp))
	fmt.Printf("Inlet:       %s\n", waterTemps(data, data.InletTemp))
	fmt.Printf("Outlet:      %s\n", waterTemps(data, data.OutletTemp))
	fmt.Printf("Ambient:     %s\n", bothTemps(data.AmbientTemp))
	fmt.Printf("Coil:        %s\n", bothTemps(data.CoilTemp))
	if !data.WaterTempsValid() {
		fmt.Printf("\n%s\n", noFlowWarning)
	}

	return nil
}

func printTempCSV(data *goaquatemp.DeviceData) error {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	// Water temperatures read NA with no flow; raw_* always holds the
	// reported value and status says whether it is valid.
	w.Write([]string{"metric", "celsius", "fahrenheit", "raw_celsius", "raw_fahrenheit", "status"})
	for _, m := range []struct {
		name  string
		c     float64
		water bool
	}{
		{"target", data.TargetTemp, false},
		{"current", data.CurrentTemp, true},
		{"inlet", data.InletTemp, true},
		{"outlet", data.OutletTemp, true},
		{"ambient", data.AmbientTemp, false},
		{"coil", data.CoilTemp, false},
	} {
		c, f, status := tempStr(m.c), tempFStr(m.c), ""
		if m.water {
			c, f, status = waterTempStr(data, m.c), waterTempFStr(data, m.c), string(data.WaterTempStatus)
		}
		w.Write([]string{m.name, c, f, tempStr(m.c), tempFStr(m.c), status})
	}

	return nil
}
