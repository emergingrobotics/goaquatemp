// Command history prints the last 24 hours of recorded inlet, outlet and
// ambient temperature and compressor frequency for the account's first
// device, at hourly resolution.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/user/goaquatemp"
)

func main() {
	client, err := goaquatemp.NewClient(goaquatemp.WithEnvConfig())
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	devices, err := client.GetDevices(ctx)
	if err != nil {
		log.Fatalf("failed to get devices: %v", err)
	}
	if len(devices) == 0 {
		log.Fatal("no devices found")
	}
	device := devices[0]

	end := time.Now()
	start := end.Add(-24 * time.Hour)

	for _, series := range goaquatemp.AllHistorySeries {
		h, err := client.GetHistory(ctx, device.Code, goaquatemp.HistoryQuery{
			Series:     series,
			Start:      start,
			End:        end,
			Resolution: goaquatemp.ResolutionHourly,
		})
		if errors.Is(err, goaquatemp.ErrHistoryUnavailable) {
			fmt.Printf("%s: no history recorded\n\n", series.Name())
			continue
		}
		if err != nil {
			log.Fatalf("failed to get %s history: %v", series.Name(), err)
		}

		celsius := h.Unit == "°C"
		if celsius {
			fmt.Printf("%s (°C / °F)\n", h.Name)
		} else {
			fmt.Printf("%s (%s)\n", h.Name, h.Unit)
		}
		for _, p := range h.Points {
			if celsius {
				fmt.Printf("  %s  %6.1f  %6.1f\n", p.Time.Format("01-02 15:04"), p.Value, goaquatemp.CelsiusToFahrenheit(p.Value))
			} else {
				fmt.Printf("  %s  %6.1f\n", p.Time.Format("01-02 15:04"), p.Value)
			}
		}
		fmt.Println()
	}
}
