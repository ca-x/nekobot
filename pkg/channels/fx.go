package channels

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/channels/dingtalk"
	"nekobot/pkg/channels/discord"
	"nekobot/pkg/channels/feishu"
	"nekobot/pkg/channels/googlechat"
	"nekobot/pkg/channels/infoflow"
	"nekobot/pkg/channels/maixcam"
	"nekobot/pkg/channels/qq"
	"nekobot/pkg/channels/serverchan"
	"nekobot/pkg/channels/slack"
	"nekobot/pkg/channels/teams"
	"nekobot/pkg/channels/telegram"
	"nekobot/pkg/channels/wework"
	"nekobot/pkg/channels/whatsapp"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/transcription"
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
	prefsMgr *userprefs.Manager,
) error {
	transcriber := transcription.NewFromConfig(log, cfg)

	// Register Telegram channel
	if cfg.Channels.Telegram.Enabled {
		telegramCfg := cfg.Channels.Telegram
		if telegramCfg.TimeoutSeconds <= 0 {
			telegramCfg.TimeoutSeconds = cfg.Channels.TimeoutSeconds
		}
		tgChannel, err := telegram.New(log, messageBus, ag, cmdRegistry, &telegramCfg, transcriber, prefsMgr)
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
		discordChannel, err := discord.NewChannel(log, cfg.Channels.Discord, messageBus, cmdRegistry, transcriber)
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
		slackChannel, err := slack.NewChannel(log, cfg.Channels.Slack, messageBus, cmdRegistry, transcriber)
		if err != nil {
			log.Warn("Failed to create Slack channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(slackChannel); err != nil {
				return err
			}
		}
	}

	// Register WhatsApp channel
	if cfg.Channels.WhatsApp.Enabled {
		whatsappChannel, err := whatsapp.NewChannel(log, cfg.Channels.WhatsApp, messageBus, cmdRegistry)
		if err != nil {
			log.Warn("Failed to create WhatsApp channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(whatsappChannel); err != nil {
				return err
			}
		}
	}

	// Register Feishu channel
	if cfg.Channels.Feishu.Enabled {
		feishuChannel, err := feishu.NewChannel(log, cfg.Channels.Feishu, messageBus, cmdRegistry)
		if err != nil {
			log.Warn("Failed to create Feishu channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(feishuChannel); err != nil {
				return err
			}
		}
	}

	// Register DingTalk channel
	if cfg.Channels.DingTalk.Enabled {
		dingtalkChannel, err := dingtalk.NewChannel(log, cfg.Channels.DingTalk, messageBus, cmdRegistry)
		if err != nil {
			log.Warn("Failed to create DingTalk channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(dingtalkChannel); err != nil {
				return err
			}
		}
	}

	// Register MaixCAM channel
	if cfg.Channels.MaixCam.Enabled {
		maixcamChannel, err := maixcam.NewChannel(log, cfg.Channels.MaixCam, messageBus, cmdRegistry)
		if err != nil {
			log.Warn("Failed to create MaixCAM channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(maixcamChannel); err != nil {
				return err
			}
		}
	}

	// Register ServerChan channel
	if cfg.Channels.ServerChan.Enabled {
		serverchanChannel, err := serverchan.NewChannel(log, cfg.Channels.ServerChan, ag, messageBus, cmdRegistry)
		if err != nil {
			log.Warn("Failed to create ServerChan channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(serverchanChannel); err != nil {
				return err
			}
		}
	}

	// Register WeWork channel
	if cfg.Channels.WeWork.Enabled {
		weworkChannel, err := wework.NewChannel(log, cfg.Channels.WeWork, messageBus, cmdRegistry)
		if err != nil {
			log.Warn("Failed to create WeWork channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(weworkChannel); err != nil {
				return err
			}
		}
	}

	// Register GoogleChat channel
	if cfg.Channels.GoogleChat.Enabled {
		googlechatChannel, err := googlechat.NewChannel(log, cfg.Channels.GoogleChat, messageBus, cmdRegistry)
		if err != nil {
			log.Warn("Failed to create GoogleChat channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(googlechatChannel); err != nil {
				return err
			}
		}
	}

	// Register QQ channel
	if cfg.Channels.QQ.Enabled {
		qqChannel, err := qq.NewChannel(log, cfg.Channels.QQ, messageBus, cmdRegistry)
		if err != nil {
			log.Warn("Failed to create QQ channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(qqChannel); err != nil {
				return err
			}
		}
	}

	// Register Teams channel
	if cfg.Channels.Teams.Enabled {
		teamsChannel, err := teams.NewChannel(log, cfg.Channels.Teams, messageBus, cmdRegistry)
		if err != nil {
			log.Warn("Failed to create Teams channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(teamsChannel); err != nil {
				return err
			}
		}
	}

	// Register Infoflow channel
	if cfg.Channels.Infoflow.Enabled {
		infoflowChannel, err := infoflow.NewChannel(log, cfg.Channels.Infoflow, messageBus, cmdRegistry)
		if err != nil {
			log.Warn("Failed to create Infoflow channel, skipping", zap.Error(err))
		} else {
			if err := manager.Register(infoflowChannel); err != nil {
				return err
			}
		}
	}

	return nil
}
