package providerstore

import (
	"context"

	"go.uber.org/fx"

	"nekobot/pkg/logger"
)

// Module provides database-backed provider storage.
var Module = fx.Module("providerstore",
	fx.Provide(NewManager),
	fx.Invoke(registerLifecycle),
)

func registerLifecycle(lc fx.Lifecycle, mgr *Manager, log *logger.Logger) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			log.Info("Provider storage shutting down")
			return mgr.Close()
		},
	})
}
