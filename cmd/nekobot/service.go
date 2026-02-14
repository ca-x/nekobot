package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kardianos/service"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/approval"
	"nekobot/pkg/bus"
	"nekobot/pkg/channels"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/cron"
	"nekobot/pkg/gateway"
	"nekobot/pkg/heartbeat"
	"nekobot/pkg/logger"
	"nekobot/pkg/process"
	"nekobot/pkg/session"
	"nekobot/pkg/skills"
	"nekobot/pkg/state"
	"nekobot/pkg/webui"
	"nekobot/pkg/workspace"
)

// GatewayService implements the service.Interface for the gateway.
type GatewayService struct {
	app    *fx.App
	logger service.Logger
}

// NewGatewayService creates a new gateway service.
func NewGatewayService() *GatewayService {
	return &GatewayService{}
}

// Start implements service.Interface.Start
func (s *GatewayService) Start(svc service.Service) error {
	if s.logger != nil {
		s.logger.Info("Starting nekobot gateway service")
	}

	// Start in a goroutine to not block
	go s.run()

	return nil
}

// Stop implements service.Interface.Stop
func (s *GatewayService) Stop(svc service.Service) error {
	if s.logger != nil {
		s.logger.Info("Stopping nekobot gateway service")
	}

	if s.app != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.app.Stop(ctx); err != nil {
			if s.logger != nil {
				s.logger.Errorf("Error stopping service: %v", err)
			}
			return err
		}
	}

	return nil
}

// run starts the gateway application.
func (s *GatewayService) run() {
	s.app = fx.New(
		// Core modules
		config.Module,
		logger.Module,
		commands.Module,
		workspace.Module,
		state.Module,
		session.Module,
		approval.Module,
		skills.Module,
		process.Module,
		agent.Module,

		// Gateway modules
		bus.Module,
		channels.Module,
		heartbeat.Module,
		cron.Module,
		gateway.Module,
		webui.Module,

		fx.Invoke(func(lc fx.Lifecycle, log *logger.Logger, b bus.Bus, cm *channels.Manager) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					log.Info("Gateway service started",
						zap.String("mode", "daemon"))
					return nil
				},
				OnStop: func(ctx context.Context) error {
					log.Info("Gateway service stopped")
					return nil
				},
			})
		}),

		fx.NopLogger, // Suppress fx logs when running as service
	)

	// Run the app
	s.app.Run()
}

// ServiceConfig returns the service configuration.
func ServiceConfig() *service.Config {
	return &service.Config{
		Name:        "nekobot-gateway",
		DisplayName: "Nekobot Gateway",
		Description: "Nekobot AI assistant gateway for multi-channel support",
		Arguments:   []string{"gateway", "run"}, // Will call "nekobot gateway run" when service starts
	}
}

// InstallService installs the gateway as a system service.
func InstallService() error {
	svcConfig := ServiceConfig()
	prg := NewGatewayService()

	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	// Set logger
	logger, err := s.Logger(nil)
	if err != nil {
		return fmt.Errorf("creating service logger: %w", err)
	}
	prg.logger = logger

	// Install
	if err := s.Install(); err != nil {
		return fmt.Errorf("installing service: %w", err)
	}

	fmt.Println("Service installed successfully!")
	fmt.Println("Use 'nekobot gateway start' to start the service")
	return nil
}

// UninstallService uninstalls the gateway service.
func UninstallService() error {
	svcConfig := ServiceConfig()
	prg := NewGatewayService()

	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	// Uninstall
	if err := s.Uninstall(); err != nil {
		return fmt.Errorf("uninstalling service: %w", err)
	}

	fmt.Println("Service uninstalled successfully!")
	return nil
}

// StartService starts the gateway service.
func StartService() error {
	svcConfig := ServiceConfig()
	prg := NewGatewayService()

	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	// Start
	if err := s.Start(); err != nil {
		return fmt.Errorf("starting service: %w", err)
	}

	fmt.Println("Service started successfully!")
	return nil
}

// StopService stops the gateway service.
func StopService() error {
	svcConfig := ServiceConfig()
	prg := NewGatewayService()

	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	// Stop
	if err := s.Stop(); err != nil {
		return fmt.Errorf("stopping service: %w", err)
	}

	fmt.Println("Service stopped successfully!")
	return nil
}

// RestartService restarts the gateway service.
func RestartService() error {
	svcConfig := ServiceConfig()
	prg := NewGatewayService()

	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	// Restart
	if err := s.Restart(); err != nil {
		return fmt.Errorf("restarting service: %w", err)
	}

	fmt.Println("Service restarted successfully!")
	return nil
}

// StatusService checks the status of the gateway service.
func StatusService() error {
	svcConfig := ServiceConfig()
	prg := NewGatewayService()

	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	// Get status
	status, err := s.Status()
	if err != nil {
		return fmt.Errorf("getting service status: %w", err)
	}

	// Print status
	statusStr := "Unknown"
	switch status {
	case service.StatusRunning:
		statusStr = "Running"
	case service.StatusStopped:
		statusStr = "Stopped"
	case service.StatusUnknown:
		statusStr = "Unknown"
	}

	fmt.Printf("Service Status: %s\n", statusStr)
	return nil
}

// RunService runs the gateway service (called by service manager).
func RunService() error {
	svcConfig := ServiceConfig()
	prg := NewGatewayService()

	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	// Set logger
	logger, err := s.Logger(nil)
	if err != nil {
		return fmt.Errorf("creating service logger: %w", err)
	}
	prg.logger = logger

	// Run the service
	if err := s.Run(); err != nil {
		logger.Error(err)
		return err
	}

	return nil
}

// runGatewayForeground runs the gateway in foreground mode (not as a service).
func runGatewayForeground() {
	app := fx.New(
		// Core modules
		config.Module,
		logger.Module,
		commands.Module,
		workspace.Module,
		state.Module,
		session.Module,
		approval.Module,
		skills.Module,
		process.Module,
		agent.Module,

		// Gateway modules
		bus.Module,
		channels.Module,
		heartbeat.Module,
		cron.Module,
		gateway.Module,
		webui.Module,

		fx.Invoke(func(lc fx.Lifecycle, log *logger.Logger, b bus.Bus, cm *channels.Manager, cfg *config.Config) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					log.Info("Gateway started",
						zap.String("mode", "foreground"),
						zap.String("host", cfg.Gateway.Host),
						zap.Int("port", cfg.Gateway.Port))

					// Log enabled channels
					enabledChannels := cm.GetEnabledChannels()
					if len(enabledChannels) > 0 {
						channelNames := make([]string, len(enabledChannels))
						for i, ch := range enabledChannels {
							channelNames[i] = ch.Name()
						}
						log.Info("Active channels", zap.Strings("channels", channelNames))
					} else {
						log.Warn("No channels enabled")
					}

					log.Info("Press Ctrl+C to stop")
					return nil
				},
			})
		}),
	)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down gateway...")
		cancel()
	}()

	// Run the app
	app.Run()

	// Wait for shutdown
	<-ctx.Done()
}
