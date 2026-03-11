package gateway

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"go.uber.org/zap"

	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Controller implements commands.GatewayController for service lifecycle operations.
type Controller struct {
	config *config.Config
	loader *config.Loader
	log    *logger.Logger
}

// NewController creates a new gateway controller.
func NewController(cfg *config.Config, loader *config.Loader, log *logger.Logger) *Controller {
	return &Controller{
		config: cfg,
		loader: loader,
		log:    log,
	}
}

// Restart attempts to restart the gateway process.
func (c *Controller) Restart() error {
	c.log.Info("Gateway restart requested")

	// On Linux/macOS, re-exec the current process.
	// On Windows, use the service manager.
	if runtime.GOOS == "windows" {
		return fmt.Errorf("restart via command not supported on Windows; use the service manager")
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	// Spawn a replacement process and signal the current one to exit.
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting new process: %w", err)
	}

	c.log.Info("New gateway process started, sending SIGTERM to current process",
		zap.Int("new_pid", cmd.Process.Pid))

	// Send SIGTERM to ourselves so fx lifecycle hooks run gracefully.
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGTERM)

	return nil
}

// ReloadConfig re-reads the configuration file and applies database overrides.
func (c *Controller) ReloadConfig() error {
	c.log.Info("Configuration reload requested")

	fresh, err := c.loader.Load("")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := config.ApplyDatabaseOverrides(fresh); err != nil {
		return fmt.Errorf("applying database overrides: %w", err)
	}

	if err := config.ValidateConfig(fresh); err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	// Apply the reloaded values into the live config.
	c.config.ApplyFrom(fresh)

	c.log.Info("Configuration reloaded successfully")
	return nil
}

// Ensure Controller satisfies the interface at compile time.
var _ commands.GatewayController = (*Controller)(nil)
