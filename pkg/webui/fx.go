package webui

import (
	"context"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Module provides the WebUI server for fx dependency injection.
var Module = fx.Module("webui",
	fx.Provide(NewServer),
	fx.Invoke(registerLifecycle),
)

func registerLifecycle(lc fx.Lifecycle, s *Server, cfg *config.Config, log *logger.Logger) {
	if !cfg.WebUI.Enabled {
		log.Info("WebUI disabled in config")
		return
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("Starting WebUI dashboard",
				zap.Int("port", s.port),
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
