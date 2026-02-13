package heartbeat

import (
	"context"
	"time"

	"go.uber.org/fx"

	"nekobot/pkg/agent"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/state"
)

// Module is the fx module for heartbeat.
var Module = fx.Module("heartbeat",
	fx.Provide(NewHeartbeat),
)

// NewHeartbeat creates a new heartbeat system for fx.
func NewHeartbeat(
	lc fx.Lifecycle,
	log *logger.Logger,
	ag *agent.Agent,
	st state.KV,
	cfg *config.Config,
) *Heartbeat {
	// Parse interval from config
	interval := 1 * time.Hour
	if cfg.Heartbeat.IntervalMinutes > 0 {
		interval = time.Duration(cfg.Heartbeat.IntervalMinutes) * time.Minute
	}

	hbConfig := &Config{
		Enabled:   cfg.Heartbeat.Enabled,
		Interval:  interval,
		Workspace: cfg.Agents.Defaults.Workspace,
	}

	hb := New(log, ag, st, hbConfig)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return hb.Start()
		},
		OnStop: func(ctx context.Context) error {
			return hb.Stop()
		},
	})

	return hb
}
