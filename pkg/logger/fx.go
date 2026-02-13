package logger

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module provides logger for fx dependency injection.
var Module = fx.Module("logger",
	fx.Provide(ProvideLogger),
)

// ProvideLogger provides a logger instance for dependency injection.
// It reads configuration from the fx lifecycle.
func ProvideLogger(lc fx.Lifecycle) (*Logger, error) {
	// For now, use default config
	// Later this will be injected from config module
	cfg := DefaultConfig()
	cfg.Development = true // TODO: read from config

	logger, err := New(cfg)
	if err != nil {
		return nil, err
	}

	// Register lifecycle hooks
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Logger initialized")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down logger")
			return logger.Sync()
		},
	})

	return logger, nil
}

// ProvideLoggerFromConfig provides a logger with configuration.
func ProvideLoggerFromConfig(cfg *Config, lc fx.Lifecycle) (*Logger, error) {
	logger, err := New(cfg)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Logger initialized with custom config",
				zap.String("level", string(cfg.Level)),
				zap.String("output", cfg.OutputPath),
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
