package logger

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ConfigProvider is an interface for providing logger configuration.
type ConfigProvider interface {
	ToLoggerConfig() *Config
}

// Module provides logger for fx dependency injection.
var Module = fx.Module("logger",
	fx.Provide(ProvideLoggerFromConfig),
)

// ProvideLoggerFromConfig provides a logger instance using config from the config module.
func ProvideLoggerFromConfig(configProvider ConfigProvider, lc fx.Lifecycle) (*Logger, error) {
	// Get logger config from config module
	cfg := configProvider.ToLoggerConfig()

	logger, err := New(cfg)
	if err != nil {
		return nil, err
	}

	// Register lifecycle hooks
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Logger initialized",
				zap.String("level", string(cfg.Level)),
				zap.String("output_path", cfg.OutputPath),
			)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down logger")
			return logger.Sync()
		},
	})

	return logger, nil
}
