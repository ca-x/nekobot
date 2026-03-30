package runtimeagents

import (
	"context"

	"go.uber.org/fx"

	"nekobot/pkg/logger"
)

// Module provides runtime agent storage.
var Module = fx.Module("runtimeagents",
	fx.Provide(NewManager),
	fx.Invoke(registerLifecycle),
)

func registerLifecycle(lc fx.Lifecycle, mgr *Manager, log *logger.Logger) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			log.Info("Agent runtime storage shutting down")
			return mgr.Close()
		},
	})
}
