package channels

import (
	"encoding/json"
	"testing"

	"nekobot/pkg/config"
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
