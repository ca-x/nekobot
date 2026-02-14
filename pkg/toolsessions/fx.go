package toolsessions

import (
	"context"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/logger"
)

// Module wires tool session persistence and lifecycle cleanup.
var Module = fx.Module("toolsessions",
	fx.Provide(NewManager),
	fx.Invoke(registerLifecycle),
)

func registerLifecycle(lc fx.Lifecycle, mgr *Manager, log *logger.Logger) {
	var cancel context.CancelFunc

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			runnerCtx, c := context.WithCancel(context.Background())
			cancel = c
			interval := mgr.Lifecycle().SweepInterval
			if interval <= 0 {
				interval = time.Minute
			}
			go mgr.cleanupLoop(runnerCtx, interval)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if cancel != nil {
				cancel()
			}
			if err := mgr.Close(); err != nil {
				log.Warn("Failed to close tool session manager", zap.Error(err))
			}
			return nil
		},
	})
}

func (m *Manager) cleanupLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := m.Cleanup(context.Background())
			if err != nil {
				m.log.Warn("Tool session cleanup failed", zap.Error(err))
				continue
			}
			if result.DetachedByIdle == 0 && result.TerminatedByTTL == 0 && result.TerminatedByLife == 0 && result.ArchivedOld == 0 {
				continue
			}
			m.log.Info("Tool session cleanup applied",
				zap.Int("detached_by_idle", result.DetachedByIdle),
				zap.Int("terminated_by_ttl", result.TerminatedByTTL),
				zap.Int("terminated_by_lifetime", result.TerminatedByLife),
				zap.Int("archived_old", result.ArchivedOld),
			)
		}
	}
}
