package channels

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/channels/telegram"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Module is the fx module for channels.
var Module = fx.Module("channels",
	fx.Provide(NewChannelManager),
	fx.Invoke(RegisterChannels),
)

// NewChannelManager creates a new channel manager for fx.
func NewChannelManager(
	lc fx.Lifecycle,
	log *logger.Logger,
	messageBus bus.Bus, // Use interface, not pointer to interface
) *Manager {
	manager := NewManager(log, messageBus)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return manager.Start()
		},
		OnStop: func(ctx context.Context) error {
			return manager.Stop()
		},
	})

	return manager
}

// RegisterChannels registers all available channels with the manager.
func RegisterChannels(
	manager *Manager,
	log *logger.Logger,
	messageBus bus.Bus, // Use interface, not pointer to interface
	ag *agent.Agent,
	cfg *config.Config,
) error {
	// Register Telegram channel
	if cfg.Channels.Telegram.Enabled {
		tgChannel, err := telegram.New(log, messageBus, ag, &cfg.Channels.Telegram)
		if err != nil {
			log.Warn("Failed to create Telegram channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(tgChannel); err != nil {
				return err
			}
		}
	}

	// TODO: Register other channels (Discord, WhatsApp, etc.)
	// if cfg.Channels.Discord.Enabled {
	//     discordChannel, err := discord.New(log, messageBus, ag, &cfg.Channels.Discord)
	//     if err == nil {
	//         manager.Register(discordChannel)
	//     }
	// }

	return nil
}
