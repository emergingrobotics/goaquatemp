package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var (
	// Global flags
	deviceCode string
	outputJSON bool
	outputCSV  bool
	debug      bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "aquatemp",
		Short: "CLI tool for Aqua Temp heat pump control",
		Long: `aquatemp is a command-line interface for monitoring and controlling
Aqua Temp heat pumps via the Linked-Go cloud API.

Configure credentials via environment variables:
  AQUATEMP_USERNAME - Your account email
  AQUATEMP_PASSWORD - Your account password

Or create a config file at ~/.config/aquatemp/config.yaml`,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&deviceCode, "device", "d", "", "Device code (or set AQUATEMP_DEFAULT_DEVICE)")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&outputCSV, "csv", false, "Output in CSV format")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug output")

	// Add commands
	rootCmd.AddCommand(
		newLoginCmd(),
		newDevicesCmd(),
		newTempCmd(),
		newStatusCmd(),
		newControlCmd(),
		newVersionCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("aquatemp %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built:  %s\n", date)
		},
	}
}
