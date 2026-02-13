package commands

import (
	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/skills"
)

// Module provides the commands system.
var Module = fx.Module("commands",
	fx.Provide(NewRegistry),
	fx.Invoke(registerBuiltins),
	fx.Invoke(registerAdvanced),
)

// registerBuiltins registers built-in commands on startup.
func registerBuiltins(registry *Registry, log *logger.Logger) error {
	if err := RegisterBuiltinCommands(registry); err != nil {
		log.Error("Failed to register builtin commands", zap.Error(err))
		return err
	}

	log.Info("Registered builtin commands", zap.Int("count", len(registry.List())))
	return nil
}

// registerAdvanced registers advanced commands with dependencies.
func registerAdvanced(
	registry *Registry,
	log *logger.Logger,
	cfg *config.Config,
	ag *agent.Agent,
	skillsMgr *skills.Manager,
	channelMgr ChannelManager, // Use interface instead of concrete type
) error {
	deps := Dependencies{
		Config:         cfg,
		Agent:          ag,
		SkillsManager:  skillsMgr,
		ChannelManager: channelMgr,
	}

	if err := RegisterAdvancedCommands(registry, deps); err != nil {
		log.Error("Failed to register advanced commands", zap.Error(err))
		return err
	}

	log.Info("Registered advanced commands",
		zap.Int("total_commands", len(registry.List())))
	return nil
}
