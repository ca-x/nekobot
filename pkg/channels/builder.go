package channels

import (
	"fmt"
	"strings"

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

// BuildChannel creates a channel instance from the current config.
func BuildChannel(
	name string,
	log *logger.Logger,
	messageBus bus.Bus,
	ag *agent.Agent,
	cmdRegistry *commands.Registry,
	prefsMgr *userprefs.Manager,
	cfg *config.Config,
) (Channel, error) {
	transcriber := transcription.NewFromConfig(log, cfg)
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "telegram":
		telegramCfg := cfg.Channels.Telegram
		if telegramCfg.TimeoutSeconds <= 0 {
			telegramCfg.TimeoutSeconds = cfg.Channels.TimeoutSeconds
		}
		return telegram.New(log, messageBus, ag, cmdRegistry, &telegramCfg, transcriber, prefsMgr)
	case "discord":
		return discord.NewChannel(log, cfg.Channels.Discord, messageBus, cmdRegistry, transcriber)
	case "slack":
		return slack.NewChannel(log, cfg.Channels.Slack, messageBus, cmdRegistry, transcriber)
	case "whatsapp":
		return whatsapp.NewChannel(log, cfg.Channels.WhatsApp, messageBus, cmdRegistry)
	case "feishu":
		return feishu.NewChannel(log, cfg.Channels.Feishu, messageBus, cmdRegistry)
	case "dingtalk":
		return dingtalk.NewChannel(log, cfg.Channels.DingTalk, messageBus, cmdRegistry)
	case "qq":
		return qq.NewChannel(log, cfg.Channels.QQ, messageBus, cmdRegistry)
	case "wework":
		return wework.NewChannel(log, cfg.Channels.WeWork, messageBus, cmdRegistry)
	case "serverchan":
		return serverchan.NewChannel(log, cfg.Channels.ServerChan, messageBus, cmdRegistry)
	case "googlechat":
		return googlechat.NewChannel(log, cfg.Channels.GoogleChat, messageBus, cmdRegistry)
	case "maixcam":
		return maixcam.NewChannel(log, cfg.Channels.MaixCam, messageBus, cmdRegistry)
	case "teams":
		return teams.NewChannel(log, cfg.Channels.Teams, messageBus, cmdRegistry)
	case "infoflow":
		return infoflow.NewChannel(log, cfg.Channels.Infoflow, messageBus, cmdRegistry)
	default:
		return nil, fmt.Errorf("unknown channel: %s", name)
	}
}

// IsChannelEnabled checks whether a channel is enabled in config.
func IsChannelEnabled(name string, cfg *config.Config) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "telegram":
		return cfg.Channels.Telegram.Enabled, nil
	case "discord":
		return cfg.Channels.Discord.Enabled, nil
	case "slack":
		return cfg.Channels.Slack.Enabled, nil
	case "whatsapp":
		return cfg.Channels.WhatsApp.Enabled, nil
	case "feishu":
		return cfg.Channels.Feishu.Enabled, nil
	case "dingtalk":
		return cfg.Channels.DingTalk.Enabled, nil
	case "qq":
		return cfg.Channels.QQ.Enabled, nil
	case "wework":
		return cfg.Channels.WeWork.Enabled, nil
	case "serverchan":
		return cfg.Channels.ServerChan.Enabled, nil
	case "googlechat":
		return cfg.Channels.GoogleChat.Enabled, nil
	case "maixcam":
		return cfg.Channels.MaixCam.Enabled, nil
	case "teams":
		return cfg.Channels.Teams.Enabled, nil
	case "infoflow":
		return cfg.Channels.Infoflow.Enabled, nil
	default:
		return false, fmt.Errorf("unknown channel: %s", name)
	}
}
