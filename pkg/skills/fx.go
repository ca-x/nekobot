package skills

import (
	"os"
	"path/filepath"

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
	manager := NewManager(log, skillsDir, autoReload)

	// Discover skills on startup
	if err := manager.Discover(); err != nil {
		log.Warn("Failed to discover skills during initialization",
			zap.Error(err))
	}

	return manager, nil
}
