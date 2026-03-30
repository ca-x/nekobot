package accountbindings

import (
	"context"

	"go.uber.org/fx"

	"nekobot/pkg/logger"
)

// Module provides account binding storage.
var Module = fx.Module("accountbindings",
	fx.Provide(NewManager),
	fx.Invoke(registerLifecycle),
)

func registerLifecycle(lc fx.Lifecycle, mgr *Manager, log *logger.Logger) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			log.Info("Account binding storage shutting down")
			return mgr.Close()
		},
	})
}
