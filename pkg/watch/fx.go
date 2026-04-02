package watch

import (
	"context"

	"go.uber.org/fx"
	"nekobot/pkg/agent"
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
	Agent  *agent.Agent  `optional:"true"`
}

func provideWatcher(p watcherParams) (*Watcher, error) {
	watcher, err := New(p.Config, p.Log, p.Audit)
	if err != nil {
		return nil, err
	}
	if p.Agent != nil {
		watcher.SetTaskService(p.Agent.TaskService())
	}
	return watcher, nil
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
