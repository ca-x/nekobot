package channels

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/channels/discord"
	"nekobot/pkg/channels/slack"
	"nekobot/pkg/channels/telegram"
	"nekobot/pkg/commands"
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
	cmdRegistry *commands.Registry,
	cfg *config.Config,
) error {
	// Register Telegram channel
	if cfg.Channels.Telegram.Enabled {
		tgChannel, err := telegram.New(log, messageBus, ag, cmdRegistry, &cfg.Channels.Telegram)
		if err != nil {
			log.Warn("Failed to create Telegram channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(tgChannel); err != nil {
				return err
			}
		}
	}

	// Register Discord channel
	if cfg.Channels.Discord.Enabled {
		discordChannel, err := discord.NewChannel(log, cfg.Channels.Discord, messageBus)
		if err != nil {
			log.Warn("Failed to create Discord channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(discordChannel); err != nil {
				return err
			}
		}
	}

	// Register Slack channel
	if cfg.Channels.Slack.Enabled {
		slackChannel, err := slack.NewChannel(log, cfg.Channels.Slack, messageBus, cmdRegistry)
		if err != nil {
			log.Warn("Failed to create Slack channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(slackChannel); err != nil {
				return err
			}
		}
	}

	return nil
}
