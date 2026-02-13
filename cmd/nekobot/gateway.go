package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start gateway server for multi-channel support",
	Long: `Start the nekobot gateway server for multi-channel support.

The gateway supports multiple communication channels like Telegram, Discord, WhatsApp, etc.
It can run in foreground mode or be installed as a system service.

Examples:
  # Run in foreground (default)
  nekobot gateway

  # Install as system service (requires sudo/admin privileges)
  sudo nekobot gateway install

  # Control the service
  sudo nekobot gateway start
  sudo nekobot gateway stop
  sudo nekobot gateway restart
  sudo nekobot gateway status

  # Uninstall the service
  sudo nekobot gateway uninstall`,
	Run: runGatewayDefault,
}

var gatewayRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run gateway in foreground or as service",
	Long:  `Run the gateway. When installed as a service, this is called automatically.`,
	Run:   runGatewayRun,
}

var gatewayInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install gateway as system service",
	Long: `Install the nekobot gateway as a system service.

This will register the gateway with the system service manager:
- Linux: systemd
- macOS: launchd
- Windows: Windows Service Manager

The service will be configured to start automatically on system boot.
Requires administrator/root privileges.`,
	Run: runGatewayInstall,
}

var gatewayUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall gateway service",
	Long: `Uninstall the nekobot gateway system service.

This will remove the service from the system service manager.
The service will be stopped before uninstallation.
Requires administrator/root privileges.`,
	Run: runGatewayUninstall,
}

var gatewayStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start gateway service",
	Long: `Start the nekobot gateway service.

The service must be installed first using 'nekobot gateway install'.
Requires administrator/root privileges.`,
	Run: runGatewayStart,
}

var gatewayStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop gateway service",
	Long: `Stop the running nekobot gateway service.

Requires administrator/root privileges.`,
	Run: runGatewayStop,
}

var gatewayRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart gateway service",
	Long: `Restart the nekobot gateway service.

This will stop and then start the service.
Requires administrator/root privileges.`,
	Run: runGatewayRestart,
}

var gatewayStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check gateway service status",
	Long:  `Check the status of the nekobot gateway service.`,
	Run:   runGatewayStatus,
}

func init() {
	// Add gateway subcommands
	gatewayCmd.AddCommand(gatewayRunCmd)
	gatewayCmd.AddCommand(gatewayInstallCmd)
	gatewayCmd.AddCommand(gatewayUninstallCmd)
	gatewayCmd.AddCommand(gatewayStartCmd)
	gatewayCmd.AddCommand(gatewayStopCmd)
	gatewayCmd.AddCommand(gatewayRestartCmd)
	gatewayCmd.AddCommand(gatewayStatusCmd)
}

// runGatewayDefault runs the gateway in foreground mode (default behavior).
func runGatewayDefault(cmd *cobra.Command, args []string) {
	fmt.Println("Starting nekobot gateway in foreground mode...")
	fmt.Println("To install as a system service, use: nekobot gateway install")
	fmt.Println()

	runGatewayForeground()
}

// runGatewayRun runs the gateway (called by service or manually).
func runGatewayRun(cmd *cobra.Command, args []string) {
	// Check if running as a service
	isService := os.Getenv("INVOCATION_ID") != "" || // systemd
		os.Getenv("_") == "/bin/launchd" || // launchd
		os.Getenv("SERVICE_NAME") != "" // Windows service

	if isService {
		// Running as service - use service runner
		if err := RunService(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running service: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Running manually - foreground mode
		runGatewayForeground()
	}
}

// runGatewayInstall installs the gateway as a system service.
func runGatewayInstall(cmd *cobra.Command, args []string) {
	if err := InstallService(); err != nil {
		fmt.Fprintf(os.Stderr, "Error installing service: %v\n", err)
		fmt.Fprintln(os.Stderr, "\nNote: Installing system services requires administrator privileges.")
		fmt.Fprintln(os.Stderr, "Please run with sudo (Linux/macOS) or as Administrator (Windows).")
		os.Exit(1)
	}
}

// runGatewayUninstall uninstalls the gateway service.
func runGatewayUninstall(cmd *cobra.Command, args []string) {
	if err := UninstallService(); err != nil {
		fmt.Fprintf(os.Stderr, "Error uninstalling service: %v\n", err)
		fmt.Fprintln(os.Stderr, "\nNote: Uninstalling system services requires administrator privileges.")
		fmt.Fprintln(os.Stderr, "Please run with sudo (Linux/macOS) or as Administrator (Windows).")
		os.Exit(1)
	}
}

// runGatewayStart starts the gateway service.
func runGatewayStart(cmd *cobra.Command, args []string) {
	if err := StartService(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting service: %v\n", err)
		fmt.Fprintln(os.Stderr, "\nNote: Starting system services requires administrator privileges.")
		fmt.Fprintln(os.Stderr, "Please run with sudo (Linux/macOS) or as Administrator (Windows).")
		os.Exit(1)
	}
}

// runGatewayStop stops the gateway service.
func runGatewayStop(cmd *cobra.Command, args []string) {
	if err := StopService(); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping service: %v\n", err)
		fmt.Fprintln(os.Stderr, "\nNote: Stopping system services requires administrator privileges.")
		fmt.Fprintln(os.Stderr, "Please run with sudo (Linux/macOS) or as Administrator (Windows).")
		os.Exit(1)
	}
}

// runGatewayRestart restarts the gateway service.
func runGatewayRestart(cmd *cobra.Command, args []string) {
	if err := RestartService(); err != nil {
		fmt.Fprintf(os.Stderr, "Error restarting service: %v\n", err)
		fmt.Fprintln(os.Stderr, "\nNote: Restarting system services requires administrator privileges.")
		fmt.Fprintln(os.Stderr, "Please run with sudo (Linux/macOS) or as Administrator (Windows).")
		os.Exit(1)
	}
}

// runGatewayStatus checks the gateway service status.
func runGatewayStatus(cmd *cobra.Command, args []string) {
	if err := StatusService(); err != nil {
		fmt.Fprintf(os.Stderr, "Error checking service status: %v\n", err)
		os.Exit(1)
	}
}
