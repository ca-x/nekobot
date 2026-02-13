package config

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module provides configuration for fx dependency injection.
var Module = fx.Module("config",
	fx.Provide(ProvideConfig),
	fx.Provide(ProvideLoader),
)

// ProvideLoader provides a configuration loader.
func ProvideLoader() *Loader {
	return NewLoader()
}

// ProvideConfig provides loaded configuration.
func ProvideConfig(loader *Loader, lc fx.Lifecycle) (*Config, error) {
	// Load configuration
	cfg, err := loader.Load("")
	if err != nil {
		return nil, err
	}

	// Validate configuration
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}

	// Register lifecycle hooks
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Log config info (when logger is available)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})

	return cfg, nil
}

// ProvideConfigWithPath provides configuration from a specific path.
func ProvideConfigWithPath(path string) func(*Loader, fx.Lifecycle) (*Config, error) {
	return func(loader *Loader, lc fx.Lifecycle) (*Config, error) {
		cfg, err := loader.LoadFromFile(path)
		if err != nil {
			return nil, err
		}

		if err := ValidateConfig(cfg); err != nil {
			return nil, err
		}

		return cfg, nil
	}
}

// ProvideWatcher provides a configuration watcher with hot-reload.
func ProvideWatcher(loader *Loader, cfg *Config, lc fx.Lifecycle, logger *zap.Logger) (*Watcher, error) {
	watcher := NewWatcher(loader, cfg)

	// Add handler to log config changes
	watcher.AddHandler(func(newCfg *Config) error {
		logger.Info("Configuration reloaded",
			zap.String("model", newCfg.Agents.Defaults.Model),
		)
		return nil
	})

	// Register lifecycle hooks
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Starting configuration watcher")
			return watcher.Start()
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Stopping configuration watcher")
			watcher.Stop()
			return nil
		},
	})

	return watcher, nil
}
