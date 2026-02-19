// Package goaquatemp provides a Go SDK for interacting with the Aqua Temp / Linked-Go
// cloud API for heat pump control and monitoring.
//
// # Quick Start
//
// Create a client and authenticate:
//
//	client, err := goaquatemp.NewClient(
//	    goaquatemp.WithUsername("user@example.com"),
//	    goaquatemp.WithPassword("password"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	ctx := context.Background()
//	if err := client.Login(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Logout(ctx)
//
// # Getting Device Data
//
// List devices and retrieve temperature data:
//
//	devices, err := client.GetDevices(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	data, err := client.GetDeviceData(ctx, devices[0].Code)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Current: %.1f°C\n", data.CurrentTemp)
//
// # Controlling Devices
//
// Set temperature, mode, or power state:
//
//	err = client.SetTemperature(ctx, deviceCode, 28.0)
//	err = client.SetMode(ctx, deviceCode, goaquatemp.ModeHeat)
//	err = client.SetPower(ctx, deviceCode, true)
//
// # API Versions
//
// The Aqua Temp API has multiple versions. Use the appropriate version for your account:
//
//   - APILegacy: For accounts created before app v1.5.8
//   - APIHiTemp: For HiTemp branded devices
//   - APIModern: For accounts created with app v1.5.9+ (default)
//
// # Configuration
//
// Configuration can be provided via functional options, environment variables,
// or a config file. See [WithUsername], [WithPassword], [WithAPIVersion], and
// [WithConfigFile] for details.
package goaquatemp
