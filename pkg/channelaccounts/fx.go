package channelaccounts

import (
	"context"

	"go.uber.org/fx"

	"nekobot/pkg/logger"
)

// Module provides channel account storage.
var Module = fx.Module("channelaccounts",
	fx.Provide(NewManager),
	fx.Invoke(registerLifecycle),
)

func registerLifecycle(lc fx.Lifecycle, mgr *Manager, log *logger.Logger) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			log.Info("Channel account storage shutting down")
			return mgr.Close()
		},
	})
}
