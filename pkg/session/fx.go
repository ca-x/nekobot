package session

import (
	"context"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Module provides session management for fx.
var Module = fx.Module("session",
	fx.Provide(func(cfg *config.Config) *Manager {
		return NewManager(cfg.WorkspacePath()+"/sessions", cfg.Sessions)
	}),
	fx.Invoke(registerCleanupLifecycle),
)

func registerCleanupLifecycle(
	lc fx.Lifecycle,
	cfg *config.Config,
	log *logger.Logger,
	manager *Manager,
) {
	if !cfg.Sessions.Enabled || !cfg.Sessions.Cleanup.Enabled {
		return
	}

	cleanup := cfg.Sessions.Cleanup
	pruner := NewPruner(manager, PruneConfig{
		Strategy:      PruneStrategyTTL,
		MaxSessionAge: time.Duration(cleanup.MaxAgeDays) * 24 * time.Hour,
	})

	stop := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			runPrune := func() {
				if err := pruner.Prune(); err != nil {
					log.Warn("Session cleanup failed", zap.Error(err))
				}
			}

			runPrune()

			ticker := time.NewTicker(time.Duration(cleanup.IntervalMinutes) * time.Minute)
			go func() {
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						runPrune()
					case <-stop:
						return
					}
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			close(stop)
			return nil
		},
	})
}

// CleanupPersistedSessions runs session pruning once using current config.
func CleanupPersistedSessions(cfg *config.Config, manager *Manager) error {
	if cfg == nil || manager == nil {
		return nil
	}
	if !cfg.Sessions.Enabled || !cfg.Sessions.Cleanup.Enabled {
		return nil
	}

	cleanup := cfg.Sessions.Cleanup
	pruner := NewPruner(manager, PruneConfig{
		Strategy:      PruneStrategyTTL,
		MaxSessionAge: time.Duration(cleanup.MaxAgeDays) * 24 * time.Hour,
	})
	return pruner.Prune()
}
