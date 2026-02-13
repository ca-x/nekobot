package commands

import (
	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/logger"
)

// Module provides the commands system.
var Module = fx.Module("commands",
	fx.Provide(NewRegistry),
	fx.Invoke(registerBuiltins),
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
