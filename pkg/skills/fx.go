package skills

import (
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Module provides skills functionality.
var Module = fx.Module("skills",
	fx.Provide(
		ProvideManager,
	),
)

// ProvideManager creates a skills manager from configuration.
func ProvideManager(log *logger.Logger, cfg *config.Config) (*Manager, error) {
	// Determine skills directory
	skillsDir := cfg.Agents.Defaults.SkillsDir
	if skillsDir == "" {
		// Default to ~/.nekobot/skills
		homeDir, err := os.UserHomeDir()
		if err != nil {
			skillsDir = "./skills"
		} else {
			skillsDir = filepath.Join(homeDir, ".nekobot", "skills")
		}
	}

	// Expand ~ in path
	if len(skillsDir) > 0 && skillsDir[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			skillsDir = filepath.Join(homeDir, skillsDir[1:])
		}
	}

	autoReload := cfg.Agents.Defaults.SkillsAutoReload
	manager := NewManagerWithRuntimeOptions(
		log,
		skillsDir,
		autoReload,
		cfg.Agents.Defaults.SkillsProxy,
		SnapshotRetentionConfig{
			AutoPrune: cfg.WebUI.SkillSnapshots.AutoPrune,
			MaxCount:  cfg.WebUI.SkillSnapshots.MaxCount,
		},
		VersionRetentionConfig{
			Enabled:  cfg.WebUI.SkillVersions.Enabled,
			MaxCount: cfg.WebUI.SkillVersions.MaxCount,
		},
	)
	manager.eligibilityCheck.SetConfigPathExists(func(path string) bool {
		return hasConfigPath(cfg, path)
	})

	// Discover skills on startup
	if err := manager.Discover(); err != nil {
		log.Warn("Failed to discover skills during initialization",
			zap.Error(err))
	}

	return manager, nil
}

func hasConfigPath(cfg *config.Config, path string) bool {
	if cfg == nil {
		return false
	}

	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return false
	}

	switch trimmed {
	case "channels.discord":
		return cfg.Channels.Discord.Enabled
	case "channels.telegram":
		return cfg.Channels.Telegram.Enabled
	case "channels.wechat":
		return cfg.Channels.WeChat.Enabled
	case "channels.wework":
		return cfg.Channels.WeWork.Enabled
	case "channels.slack":
		return cfg.Channels.Slack.Enabled
	case "channels.whatsapp":
		return cfg.Channels.WhatsApp.Enabled
	case "channels.feishu":
		return cfg.Channels.Feishu.Enabled
	case "channels.qq":
		return cfg.Channels.QQ.Enabled
	case "channels.dingtalk":
		return cfg.Channels.DingTalk.Enabled
	case "channels.googlechat":
		return cfg.Channels.GoogleChat.Enabled
	case "channels.teams":
		return cfg.Channels.Teams.Enabled
	case "channels.infoflow":
		return cfg.Channels.Infoflow.Enabled
	case "channels.gotify":
		return cfg.Channels.Gotify.Enabled
	default:
		return false
	}
}
