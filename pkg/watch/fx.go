package watch

import (
	"context"

	"go.uber.org/fx"
	"nekobot/pkg/audit"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Module provides file watching for fx.
var Module = fx.Module("watch",
	fx.Provide(provideWatcher),
	fx.Invoke(registerLifecycle),
)

type watcherParams struct {
	fx.In

	Config *config.Config
	Log    *logger.Logger
	Audit  *audit.Logger `optional:"true"`
}

func provideWatcher(p watcherParams) (*Watcher, error) {
	return New(p.Config, p.Log, p.Audit)
}

func registerLifecycle(lc fx.Lifecycle, watcher *Watcher, cfg *config.Config) {
	if watcher == nil || cfg == nil || !cfg.Watch.Enabled {
		return
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			return watcher.Start()
		},
		OnStop: func(context.Context) error {
			return watcher.Stop()
		},
	})
}
