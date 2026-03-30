package channels

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/channelaccounts"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/process"
	"nekobot/pkg/toolsessions"
	"nekobot/pkg/userprefs"
)

// Module is the fx module for channels.
var Module = fx.Module("channels",
	fx.Provide(NewChannelManager),
	fx.Provide(
		fx.Annotate(
			newCommandChannelAdapter,
			fx.As(new(commands.ChannelManager)),
		),
	),
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
	accountMgr *channelaccounts.Manager,
	prefsMgr *userprefs.Manager,
	toolSessionMgr *toolsessions.Manager,
	processMgr *process.Manager,
) error {
	accountedTypes := map[string]bool{}
	if accountMgr != nil {
		accounts, err := accountMgr.List(context.Background())
		if err != nil {
			return err
		}
		for _, account := range accounts {
			if !account.Enabled {
				continue
			}

			channel, err := BuildChannelFromAccount(
				account,
				log,
				messageBus,
				ag,
				cmdRegistry,
				prefsMgr,
				toolSessionMgr,
				processMgr,
				cfg,
			)
			if err != nil {
				log.Warn("Failed to create account-scoped channel, skipping",
					zap.String("channel_type", account.ChannelType),
					zap.String("account_key", account.AccountKey),
					zap.Error(err))
				continue
			}
			if err := manager.Register(channel); err != nil {
				return err
			}
			accountedTypes[account.ChannelType] = true
		}
	}

	for _, name := range ChannelNames() {
		if accountedTypes[name] {
			continue
		}

		enabled, err := IsChannelEnabled(name, cfg)
		if err != nil {
			return err
		}
		if !enabled {
			continue
		}

		channel, err := BuildChannel(name, log, messageBus, ag, cmdRegistry, prefsMgr, toolSessionMgr, processMgr, cfg)
		if err != nil {
			log.Warn("Failed to create channel, skipping", zap.String("channel", name), zap.Error(err))
			continue
		}
		if err := manager.Register(channel); err != nil {
			return err
		}
	}

	return nil
}
