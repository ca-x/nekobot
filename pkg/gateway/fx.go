package gateway

import (
	"context"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Module provides the gateway server for fx dependency injection.
var Module = fx.Module("gateway",
	fx.Provide(NewServer),
	fx.Provide(provideGatewayController),
	fx.Invoke(registerLifecycle),
)

func provideGatewayController(cfg *config.Config, loader *config.Loader, log *logger.Logger) commands.GatewayController {
	return NewController(cfg, loader, log)
}

func registerLifecycle(lc fx.Lifecycle, s *Server, cfg *config.Config, log *logger.Logger) {
	if cfg.Gateway.Port == 0 {
		log.Info("Gateway server disabled (port not configured)")
		return
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("Starting WebSocket gateway",
				zap.String("host", cfg.Gateway.Host),
				zap.Int("port", cfg.Gateway.Port),
			)
			return s.Start()
		},
		OnStop: func(ctx context.Context) error {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return s.Stop(shutdownCtx)
		},
	})
}
