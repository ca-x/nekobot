package prompts

import (
	"context"

	"go.uber.org/fx"

	"nekobot/pkg/logger"
)

// Module provides prompt storage and resolution.
var Module = fx.Module("prompts",
	fx.Provide(NewManager),
	fx.Invoke(registerLifecycle),
)

func registerLifecycle(lc fx.Lifecycle, mgr *Manager, log *logger.Logger) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			log.Info("Prompt storage shutting down")
			return mgr.Close()
		},
	})
}
