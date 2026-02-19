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

		power := "OFF"
		if data.Power {
			power = "ON"
		}

		fmt.Printf("[%s] Power: %s  Mode: %s  Current: %.1fC  Target: %.1fC  Ambient: %.1fC\n",
			time.Now().Format("15:04:05"),
			power,
			data.Mode,
			data.CurrentTemp,
			data.TargetTemp,
			data.AmbientTemp,
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
