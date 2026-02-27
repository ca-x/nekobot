package cron

import (
	"context"

	"go.uber.org/fx"

	"nekobot/pkg/agent"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
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
	client *ent.Client,
) *Manager {
	manager := New(log, ag, client)

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
