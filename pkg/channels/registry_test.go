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
