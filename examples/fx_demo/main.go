// Package main demonstrates fx dependency injection with logger and config.
package main

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Application demonstrates using fx for dependency injection.
func main() {
	app := fx.New(
		// Provide modules
		logger.Module,
		config.Module,

		// Provide application logic
		fx.Invoke(runApp),
	)

	app.Run()
}

// runApp is the main application logic that gets dependencies injected.
func runApp(
	lc fx.Lifecycle,
	log *logger.Logger,
	cfg *config.Config,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("Application starting",
				zap.String("component", "fx-demo"),
			)

			log.Info("Configuration loaded",
				zap.String("workspace", cfg.WorkspacePath()),
			)

			log.Info("Using model: " + cfg.Agents.Defaults.Model)
			log.Info("Workspace: " + cfg.WorkspacePath())

			// Application logic goes here...

			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("Application shutting down")
			return nil
		},
	})
}
