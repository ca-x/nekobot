package channels

import (
	"encoding/json"
	"testing"

	"nekobot/pkg/channelaccounts"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

func TestApplyChannelConfigAndListChannelConfigsIncludeWechat(t *testing.T) {
	cfg := config.DefaultConfig()

	raw := json.RawMessage(`{"enabled":true,"poll_interval_seconds":9,"allow_from":["u1"]}`)
	if err := ApplyChannelConfig(cfg, "wechat", raw); err != nil {
		t.Fatalf("ApplyChannelConfig failed: %v", err)
	}

	if !cfg.Channels.WeChat.Enabled || cfg.Channels.WeChat.PollIntervalSeconds != 9 {
		t.Fatalf("unexpected wechat config after apply: %+v", cfg.Channels.WeChat)
	}

	configs := ListChannelConfigs(cfg)
	value, ok := configs["wechat"]
	if !ok {
		t.Fatalf("expected wechat in config list")
	}

	wechatCfg, ok := value.(config.WeChatConfig)
	if !ok {
		t.Fatalf("expected WeChatConfig type, got %T", value)
	}
	if !wechatCfg.Enabled || wechatCfg.PollIntervalSeconds != 9 {
		t.Fatalf("unexpected listed wechat config: %+v", wechatCfg)
	}
}

func TestApplyChannelConfigAndListChannelConfigsIncludeGotify(t *testing.T) {
	cfg := config.DefaultConfig()

	raw := json.RawMessage(`{"enabled":true,"server_url":"https://gotify.example.com","app_token":"token","priority":7}`)
	if err := ApplyChannelConfig(cfg, "gotify", raw); err != nil {
		t.Fatalf("ApplyChannelConfig failed: %v", err)
	}

	if !cfg.Channels.Gotify.Enabled || cfg.Channels.Gotify.Priority != 7 {
		t.Fatalf("unexpected gotify config after apply: %+v", cfg.Channels.Gotify)
	}

	configs := ListChannelConfigs(cfg)
	value, ok := configs["gotify"]
	if !ok {
		t.Fatalf("expected gotify in config list")
	}

	gotifyCfg, ok := value.(config.GotifyConfig)
	if !ok {
		t.Fatalf("expected GotifyConfig type, got %T", value)
	}
	if !gotifyCfg.Enabled || gotifyCfg.ServerURL != "https://gotify.example.com" || gotifyCfg.Priority != 7 {
		t.Fatalf("unexpected listed gotify config: %+v", gotifyCfg)
	}
}

func TestBuildChannelFromAccount_Gotify(t *testing.T) {
	cfg := config.DefaultConfig()
	log := newRegistryTestLogger(t)

	account := channelaccounts.ChannelAccount{
		ChannelType: "gotify",
		AccountKey:  "alerts-a",
		DisplayName: "Alerts A",
		Config: map[string]interface{}{
			"enabled":    true,
			"server_url": "https://gotify.example.com",
			"app_token":  "token-1",
			"priority":   6,
		},
	}

	channel, err := BuildChannelFromAccount(account, log, nil, nil, nil, nil, nil, nil, cfg)
	if err != nil {
		t.Fatalf("BuildChannelFromAccount failed: %v", err)
	}
	if channel.ID() != "gotify:alerts-a" {
		t.Fatalf("unexpected account channel id: %s", channel.ID())
	}
	if typed, ok := channel.(TypedChannel); !ok || typed.ChannelType() != "gotify" {
		t.Fatalf("expected typed gotify channel, got %T", channel)
	}
	if channel.Name() != "Alerts A" {
		t.Fatalf("unexpected account channel name: %s", channel.Name())
	}
}

func TestBuildChannelFromAccount_Telegram(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Channels.TimeoutSeconds = 41
	log := newRegistryTestLogger(t)

	account := channelaccounts.ChannelAccount{
		ChannelType: "telegram",
		AccountKey:  "bot-a",
		DisplayName: "Telegram Bot A",
		Config: map[string]interface{}{
			"enabled": true,
			"token":   "telegram-token",
		},
	}

	channel, err := BuildChannelFromAccount(account, log, nil, nil, nil, nil, nil, nil, cfg)
	if err != nil {
		t.Fatalf("BuildChannelFromAccount failed: %v", err)
	}
	if channel.ID() != "telegram:bot-a" {
		t.Fatalf("unexpected telegram account channel id: %s", channel.ID())
	}
	if typed, ok := channel.(TypedChannel); !ok || typed.ChannelType() != "telegram" {
		t.Fatalf("expected typed telegram channel, got %T", channel)
	}
	if channel.Name() != "Telegram Bot A" {
		t.Fatalf("unexpected telegram account channel name: %s", channel.Name())
	}
}

func TestBuildChannelFromAccount_Slack(t *testing.T) {
	cfg := config.DefaultConfig()
	log := newRegistryTestLogger(t)

	account := channelaccounts.ChannelAccount{
		ChannelType: "slack",
		AccountKey:  "team-a",
		DisplayName: "Slack Team A",
		Config: map[string]interface{}{
			"enabled":   true,
			"bot_token": "xoxb-test",
			"app_token": "xapp-test",
		},
	}

	channel, err := BuildChannelFromAccount(account, log, nil, nil, nil, nil, nil, nil, cfg)
	if err != nil {
		t.Fatalf("BuildChannelFromAccount failed: %v", err)
	}
	if channel.ID() != "slack:team-a" {
		t.Fatalf("unexpected slack account channel id: %s", channel.ID())
	}
	if typed, ok := channel.(TypedChannel); !ok || typed.ChannelType() != "slack" {
		t.Fatalf("expected typed slack channel, got %T", channel)
	}
	if channel.Name() != "Slack Team A" {
		t.Fatalf("unexpected slack account channel name: %s", channel.Name())
	}
}

func TestBuildChannelFromAccount_Wechat(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Storage.DBDir = t.TempDir()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	log := newRegistryTestLogger(t)

	account := channelaccounts.ChannelAccount{
		ChannelType: "wechat",
		AccountKey:  "bot-a@im.wechat",
		DisplayName: "WeChat Bot A",
		Config: map[string]interface{}{
			"enabled":               true,
			"poll_interval_seconds": 9,
		},
	}

	channel, err := BuildChannelFromAccount(account, log, nil, nil, nil, nil, nil, nil, cfg)
	if err != nil {
		t.Fatalf("BuildChannelFromAccount failed: %v", err)
	}
	if channel.ID() != "wechat:bot-a@im.wechat" {
		t.Fatalf("unexpected wechat account channel id: %s", channel.ID())
	}
	if typed, ok := channel.(TypedChannel); !ok || typed.ChannelType() != "wechat" {
		t.Fatalf("expected typed wechat channel, got %T", channel)
	}
	if channel.Name() != "WeChat Bot A" {
		t.Fatalf("unexpected wechat account channel name: %s", channel.Name())
	}
}

func newRegistryTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	cfg := logger.DefaultConfig()
	cfg.OutputPath = ""
	cfg.Development = true
	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	return log
}
