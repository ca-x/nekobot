package channels

import (
	"encoding/json"
	"fmt"

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
	"nekobot/pkg/channels/wechat"
	"nekobot/pkg/channels/wework"
	"nekobot/pkg/channels/whatsapp"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/process"
	"nekobot/pkg/toolsessions"
	"nekobot/pkg/transcription"
	"nekobot/pkg/userprefs"
)

type channelDescriptor struct {
	name    string
	get     func(*config.Config) interface{}
	set     func(*config.Config, json.RawMessage) error
	enabled func(*config.Config) bool
	build   func(
		log *logger.Logger,
		messageBus bus.Bus,
		ag *agent.Agent,
		cmdRegistry *commands.Registry,
		prefsMgr *userprefs.Manager,
		toolSessionMgr *toolsessions.Manager,
		processMgr *process.Manager,
		cfg *config.Config,
	) (Channel, error)
}

var channelDescriptors = []channelDescriptor{
	{
		name: "telegram",
		get:  func(cfg *config.Config) interface{} { return cfg.Channels.Telegram },
		set: func(cfg *config.Config, data json.RawMessage) error {
			return json.Unmarshal(data, &cfg.Channels.Telegram)
		},
		enabled: func(cfg *config.Config) bool { return cfg.Channels.Telegram.Enabled },
		build: func(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cmdRegistry *commands.Registry, prefsMgr *userprefs.Manager, toolSessionMgr *toolsessions.Manager, processMgr *process.Manager, cfg *config.Config) (Channel, error) {
			telegramCfg := cfg.Channels.Telegram
			if telegramCfg.TimeoutSeconds <= 0 {
				telegramCfg.TimeoutSeconds = cfg.Channels.TimeoutSeconds
			}
			transcriber := transcription.NewFromConfig(log, cfg)
			return telegram.New(log, messageBus, ag, cmdRegistry, &telegramCfg, transcriber, prefsMgr)
		},
	},
	{
		name: "discord",
		get:  func(cfg *config.Config) interface{} { return cfg.Channels.Discord },
		set: func(cfg *config.Config, data json.RawMessage) error {
			return json.Unmarshal(data, &cfg.Channels.Discord)
		},
		enabled: func(cfg *config.Config) bool { return cfg.Channels.Discord.Enabled },
		build: func(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cmdRegistry *commands.Registry, prefsMgr *userprefs.Manager, toolSessionMgr *toolsessions.Manager, processMgr *process.Manager, cfg *config.Config) (Channel, error) {
			transcriber := transcription.NewFromConfig(log, cfg)
			return discord.NewChannel(log, cfg.Channels.Discord, messageBus, cmdRegistry, transcriber)
		},
	},
	{
		name:    "slack",
		get:     func(cfg *config.Config) interface{} { return cfg.Channels.Slack },
		set:     func(cfg *config.Config, data json.RawMessage) error { return json.Unmarshal(data, &cfg.Channels.Slack) },
		enabled: func(cfg *config.Config) bool { return cfg.Channels.Slack.Enabled },
		build: func(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cmdRegistry *commands.Registry, prefsMgr *userprefs.Manager, toolSessionMgr *toolsessions.Manager, processMgr *process.Manager, cfg *config.Config) (Channel, error) {
			transcriber := transcription.NewFromConfig(log, cfg)
			return slack.NewChannel(log, cfg.Channels.Slack, messageBus, cmdRegistry, transcriber)
		},
	},
	{
		name: "whatsapp",
		get:  func(cfg *config.Config) interface{} { return cfg.Channels.WhatsApp },
		set: func(cfg *config.Config, data json.RawMessage) error {
			return json.Unmarshal(data, &cfg.Channels.WhatsApp)
		},
		enabled: func(cfg *config.Config) bool { return cfg.Channels.WhatsApp.Enabled },
		build: func(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cmdRegistry *commands.Registry, prefsMgr *userprefs.Manager, toolSessionMgr *toolsessions.Manager, processMgr *process.Manager, cfg *config.Config) (Channel, error) {
			return whatsapp.NewChannel(log, cfg.Channels.WhatsApp, messageBus, cmdRegistry)
		},
	},
	{
		name: "wechat",
		get:  func(cfg *config.Config) interface{} { return cfg.Channels.WeChat },
		set: func(cfg *config.Config, data json.RawMessage) error {
			return json.Unmarshal(data, &cfg.Channels.WeChat)
		},
		enabled: func(cfg *config.Config) bool { return cfg.Channels.WeChat.Enabled },
		build: func(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cmdRegistry *commands.Registry, prefsMgr *userprefs.Manager, toolSessionMgr *toolsessions.Manager, processMgr *process.Manager, cfg *config.Config) (Channel, error) {
			store, err := wechat.NewCredentialStore(cfg)
			if err != nil {
				return nil, err
			}
			transcriber := transcription.NewFromConfig(log, cfg)
			return wechat.NewChannel(log, cfg.Channels.WeChat, messageBus, ag, cmdRegistry, store, toolSessionMgr, processMgr, cfg, transcriber)
		},
	},
	{
		name: "feishu",
		get:  func(cfg *config.Config) interface{} { return cfg.Channels.Feishu },
		set: func(cfg *config.Config, data json.RawMessage) error {
			return json.Unmarshal(data, &cfg.Channels.Feishu)
		},
		enabled: func(cfg *config.Config) bool { return cfg.Channels.Feishu.Enabled },
		build: func(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cmdRegistry *commands.Registry, prefsMgr *userprefs.Manager, toolSessionMgr *toolsessions.Manager, processMgr *process.Manager, cfg *config.Config) (Channel, error) {
			return feishu.NewChannel(log, cfg.Channels.Feishu, messageBus, cmdRegistry)
		},
	},
	{
		name: "dingtalk",
		get:  func(cfg *config.Config) interface{} { return cfg.Channels.DingTalk },
		set: func(cfg *config.Config, data json.RawMessage) error {
			return json.Unmarshal(data, &cfg.Channels.DingTalk)
		},
		enabled: func(cfg *config.Config) bool { return cfg.Channels.DingTalk.Enabled },
		build: func(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cmdRegistry *commands.Registry, prefsMgr *userprefs.Manager, toolSessionMgr *toolsessions.Manager, processMgr *process.Manager, cfg *config.Config) (Channel, error) {
			return dingtalk.NewChannel(log, cfg.Channels.DingTalk, messageBus, cmdRegistry)
		},
	},
	{
		name:    "qq",
		get:     func(cfg *config.Config) interface{} { return cfg.Channels.QQ },
		set:     func(cfg *config.Config, data json.RawMessage) error { return json.Unmarshal(data, &cfg.Channels.QQ) },
		enabled: func(cfg *config.Config) bool { return cfg.Channels.QQ.Enabled },
		build: func(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cmdRegistry *commands.Registry, prefsMgr *userprefs.Manager, toolSessionMgr *toolsessions.Manager, processMgr *process.Manager, cfg *config.Config) (Channel, error) {
			return qq.NewChannel(log, cfg.Channels.QQ, messageBus, cmdRegistry)
		},
	},
	{
		name: "wework",
		get:  func(cfg *config.Config) interface{} { return cfg.Channels.WeWork },
		set: func(cfg *config.Config, data json.RawMessage) error {
			return json.Unmarshal(data, &cfg.Channels.WeWork)
		},
		enabled: func(cfg *config.Config) bool { return cfg.Channels.WeWork.Enabled },
		build: func(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cmdRegistry *commands.Registry, prefsMgr *userprefs.Manager, toolSessionMgr *toolsessions.Manager, processMgr *process.Manager, cfg *config.Config) (Channel, error) {
			return wework.NewChannel(log, cfg.Channels.WeWork, messageBus, cmdRegistry)
		},
	},
	{
		name: "serverchan",
		get:  func(cfg *config.Config) interface{} { return cfg.Channels.ServerChan },
		set: func(cfg *config.Config, data json.RawMessage) error {
			return json.Unmarshal(data, &cfg.Channels.ServerChan)
		},
		enabled: func(cfg *config.Config) bool { return cfg.Channels.ServerChan.Enabled },
		build: func(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cmdRegistry *commands.Registry, prefsMgr *userprefs.Manager, toolSessionMgr *toolsessions.Manager, processMgr *process.Manager, cfg *config.Config) (Channel, error) {
			return serverchan.NewChannel(log, cfg.Channels.ServerChan, ag, messageBus, cmdRegistry)
		},
	},
	{
		name: "googlechat",
		get:  func(cfg *config.Config) interface{} { return cfg.Channels.GoogleChat },
		set: func(cfg *config.Config, data json.RawMessage) error {
			return json.Unmarshal(data, &cfg.Channels.GoogleChat)
		},
		enabled: func(cfg *config.Config) bool { return cfg.Channels.GoogleChat.Enabled },
		build: func(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cmdRegistry *commands.Registry, prefsMgr *userprefs.Manager, toolSessionMgr *toolsessions.Manager, processMgr *process.Manager, cfg *config.Config) (Channel, error) {
			return googlechat.NewChannel(log, cfg.Channels.GoogleChat, messageBus, cmdRegistry)
		},
	},
	{
		name: "maixcam",
		get:  func(cfg *config.Config) interface{} { return cfg.Channels.MaixCam },
		set: func(cfg *config.Config, data json.RawMessage) error {
			return json.Unmarshal(data, &cfg.Channels.MaixCam)
		},
		enabled: func(cfg *config.Config) bool { return cfg.Channels.MaixCam.Enabled },
		build: func(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cmdRegistry *commands.Registry, prefsMgr *userprefs.Manager, toolSessionMgr *toolsessions.Manager, processMgr *process.Manager, cfg *config.Config) (Channel, error) {
			return maixcam.NewChannel(log, cfg.Channels.MaixCam, messageBus, cmdRegistry)
		},
	},
	{
		name:    "teams",
		get:     func(cfg *config.Config) interface{} { return cfg.Channels.Teams },
		set:     func(cfg *config.Config, data json.RawMessage) error { return json.Unmarshal(data, &cfg.Channels.Teams) },
		enabled: func(cfg *config.Config) bool { return cfg.Channels.Teams.Enabled },
		build: func(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cmdRegistry *commands.Registry, prefsMgr *userprefs.Manager, toolSessionMgr *toolsessions.Manager, processMgr *process.Manager, cfg *config.Config) (Channel, error) {
			return teams.NewChannel(log, cfg.Channels.Teams, messageBus, cmdRegistry)
		},
	},
	{
		name: "infoflow",
		get:  func(cfg *config.Config) interface{} { return cfg.Channels.Infoflow },
		set: func(cfg *config.Config, data json.RawMessage) error {
			return json.Unmarshal(data, &cfg.Channels.Infoflow)
		},
		enabled: func(cfg *config.Config) bool { return cfg.Channels.Infoflow.Enabled },
		build: func(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cmdRegistry *commands.Registry, prefsMgr *userprefs.Manager, toolSessionMgr *toolsessions.Manager, processMgr *process.Manager, cfg *config.Config) (Channel, error) {
			return infoflow.NewChannel(log, cfg.Channels.Infoflow, messageBus, cmdRegistry)
		},
	},
}

func getChannelDescriptor(name string) (*channelDescriptor, error) {
	for i := range channelDescriptors {
		if channelDescriptors[i].name == name {
			return &channelDescriptors[i], nil
		}
	}
	return nil, fmt.Errorf("unknown channel: %s", name)
}

// ChannelNames returns all registered channel names in stable order.
func ChannelNames() []string {
	names := make([]string, 0, len(channelDescriptors))
	for _, descriptor := range channelDescriptors {
		names = append(names, descriptor.name)
	}
	return names
}

// ListChannelConfigs returns editable channel configs keyed by channel name.
func ListChannelConfigs(cfg *config.Config) map[string]interface{} {
	configs := make(map[string]interface{}, len(channelDescriptors))
	for _, descriptor := range channelDescriptors {
		configs[descriptor.name] = descriptor.get(cfg)
	}
	return configs
}

// ApplyChannelConfig decodes a specific channel config payload into runtime config.
func ApplyChannelConfig(cfg *config.Config, name string, data json.RawMessage) error {
	descriptor, err := getChannelDescriptor(name)
	if err != nil {
		return err
	}
	return descriptor.set(cfg, data)
}
