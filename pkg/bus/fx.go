package bus

import (
	"context"

	"go.uber.org/fx"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Module is the fx module for the message bus.
var Module = fx.Module("bus",
	fx.Provide(NewMessageBus),
)

// NewMessageBus creates a new message bus for fx.
func NewMessageBus(
	lc fx.Lifecycle,
	log *logger.Logger,
	cfg *config.Config,
) (Bus, error) {
	// Determine bus configuration
	busConfig := &Config{
		Type:       BusTypeLocal, // Default to local
		BufferSize: 100,          // Default buffer size
	}

	// Override with config if bus settings exist
	if cfg.Bus.Type != "" {
		busConfig.Type = BusType(cfg.Bus.Type)
	}
	if cfg.Bus.RedisAddr != "" {
		busConfig.RedisAddr = cfg.Bus.RedisAddr
		busConfig.RedisPassword = cfg.Bus.RedisPassword
		busConfig.RedisDB = cfg.Bus.RedisDB
		busConfig.RedisPrefix = cfg.Bus.RedisPrefix
	}

	// Default buffer size
	if busConfig.BufferSize <= 0 {
		busConfig.BufferSize = 100
	}

	bus, err := NewBus(log, busConfig)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return bus.Start()
		},
		OnStop: func(ctx context.Context) error {
			return bus.Stop()
		},
	})

	return bus, nil
}
