package heartbeat

import (
	"context"

	"go.uber.org/fx"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/session"
)

// Module is the fx module for heartbeat.
var Module = fx.Module("heartbeat",
	fx.Provide(NewService),
	fx.Invoke(StartHeartbeat),
)

// StartHeartbeat registers the heartbeat service lifecycle hooks.
func StartHeartbeat(
	lc fx.Lifecycle,
	service *Service,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return service.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return service.Stop(ctx)
		},
	})
}
