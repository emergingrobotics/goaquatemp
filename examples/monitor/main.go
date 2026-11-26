package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/user/goaquatemp"
)

func main() {
	client, err := goaquatemp.NewClient(
		goaquatemp.WithEnvConfig(),
	)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := client.Login(ctx); err != nil {
		log.Fatalf("login failed: %v", err)
	}
	defer client.Logout(ctx)

	devices, err := client.GetDevices(ctx)
	if err != nil {
		log.Fatalf("failed to get devices: %v", err)
	}
	if len(devices) == 0 {
		log.Fatal("no devices found")
	}

	deviceCode := devices[0].Code
	fmt.Printf("Monitoring device: %s (%s)\n", devices[0].Name, deviceCode)
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	poll := func() {
		data, err := client.GetDeviceData(ctx, deviceCode)
		if err != nil {
			log.Printf("error: %v", err)
			return
		}

		compressor := "?"
		if data.CompressorRunning != nil {
			compressor = "off"
			if *data.CompressorRunning {
				compressor = "on"
				if data.CompressorFrequency != nil {
					compressor = fmt.Sprintf("%.0fHz", *data.CompressorFrequency)
				}
			}
		}

		ct := func(c float64) string {
			return fmt.Sprintf("%.1fC/%.1fF", c, goaquatemp.CelsiusToFahrenheit(c))
		}
		// With no water flow the water sensors read stagnant water: NA.
		wt := func(c float64) string {
			if !data.WaterTempsValid() {
				return "NA(" + ct(c) + ")"
			}
			return ct(c)
		}
		fmt.Printf("[%s] %-10s mode=%-4s compressor=%-5s inlet=%s outlet=%s ambient=%s target=%s\n",
			time.Now().Format("15:04:05"),
			data.State,
			data.Mode,
			compressor,
			wt(data.InletTemp),
			wt(data.OutletTemp),
			ct(data.AmbientTemp),
			ct(data.TargetTemp),
		)
	}

	poll()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nStopped")
			return
		case <-ticker.C:
			poll()
		}
	}
}
