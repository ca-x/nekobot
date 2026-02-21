package config

import (
	"context"

	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"

	"go.uber.org/fx"
)

// Module provides configuration for fx dependency injection.
var Module = fx.Module("config",
	fx.Provide(ProvideLoader),
	fx.Provide(ProvideConfig),
	fx.Provide(ProvideRuntimeEntClient),
	fx.Provide(
		fx.Annotate(
			ProvideLoggerConfig,
			fx.As(new(logger.ConfigProvider)),
		),
	),
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

	// Runtime sections are persisted in SQLite; apply DB overrides on top of file config.
	if err := ApplyDatabaseOverrides(cfg); err != nil {
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

// ProvideRuntimeEntClient provides the shared runtime Ent client.
func ProvideRuntimeEntClient(cfg *Config, lc fx.Lifecycle) (*ent.Client, error) {
	client, err := OpenRuntimeEntClient(cfg)
	if err != nil {
		return nil, err
	}
	if err := EnsureRuntimeEntSchema(client); err != nil {
		_ = client.Close()
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return client.Close()
		},
	})
	return client, nil
}

// ProvideLoggerConfig provides logger configuration for the logger module.
func ProvideLoggerConfig(cfg *Config) *LoggerConfig {
	return &cfg.Logger
}

