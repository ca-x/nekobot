package cron

import (
	"context"

	"go.uber.org/fx"

	"nekobot/pkg/agent"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Module is the fx module for cron.
var Module = fx.Module("cron",
	fx.Provide(NewManager),
)

// NewManager creates a new cron manager for fx.
func NewManager(
	lc fx.Lifecycle,
	log *logger.Logger,
	ag *agent.Agent,
	cfg *config.Config,
) *Manager {
	manager := New(log, ag, cfg.Agents.Defaults.Workspace)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return manager.Start()
		},
		OnStop: func(ctx context.Context) error {
			return manager.Stop()
		},
	})

	return manager
}
