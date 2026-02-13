package skills

import (
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"path/filepath"

	"go.uber.org/fx"
)

// Module provides skills functionality.
var Module = fx.Module("skills",
	fx.Provide(
		NewManager,
	),
)

// NewManager creates a skills manager from configuration.
func NewManager(log *logger.Logger, cfg *config.Config) (*Manager, error) {
	// Determine skills directory
	skillsDir := cfg.Agents.Defaults.SkillsDir
	if skillsDir == "" {
		// Default to ~/.nekobot/skills
		homeDir, err := cfg.GetExpandedWorkspace()
		if err != nil {
			skillsDir = "./skills"
		} else {
			skillsDir = filepath.Join(filepath.Dir(homeDir), "skills")
		}
	}

	// Expand ~ in path
	if len(skillsDir) > 0 && skillsDir[0] == '~' {
		homeDir, err := cfg.GetExpandedWorkspace()
		if err == nil {
			skillsDir = filepath.Join(filepath.Dir(homeDir), skillsDir[1:])
		}
	}

	autoReload := cfg.Agents.Defaults.SkillsAutoReload
	manager := NewManager(log, skillsDir, autoReload)

	// Discover skills on startup
	if err := manager.Discover(); err != nil {
		log.Warn("Failed to discover skills during initialization",
			logger.Error(err))
	}

	return manager, nil
}
