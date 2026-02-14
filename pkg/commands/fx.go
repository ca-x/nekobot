package commands

import (
	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/skills"
	"nekobot/pkg/userprefs"
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
	p struct {
		fx.In

		Registry   *Registry
		Log        *logger.Logger
		Config     *config.Config
		Agent      *agent.Agent
		Skills     *skills.Manager    `optional:"true"`
		ChannelMgr ChannelManager     `optional:"true"`
		UserPrefs  *userprefs.Manager `optional:"true"`
	},
) error {
	deps := Dependencies{
		Config:         p.Config,
		Agent:          p.Agent,
		SkillsManager:  p.Skills,
		ChannelManager: p.ChannelMgr,
		UserPrefs:      p.UserPrefs,
	}

	if err := RegisterAdvancedCommands(p.Registry, deps); err != nil {
		p.Log.Error("Failed to register advanced commands", zap.Error(err))
		return err
	}

	p.Log.Info("Registered advanced commands",
		zap.Int("total_commands", len(p.Registry.List())))
	return nil
}
