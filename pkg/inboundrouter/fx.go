package inboundrouter

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/channelaccounts"
	"nekobot/pkg/logger"
)

// Module provides the unified inbound router.
var Module = fx.Module("inboundrouter",
	fx.Provide(New),
	fx.Invoke(registerLifecycle),
)

func registerLifecycle(lc fx.Lifecycle, router *Router, accounts *channelaccounts.Manager, log *logger.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			accountList, err := accounts.List(ctx)
			if err != nil {
				return err
			}
			for _, account := range accountList {
				if !account.Enabled {
					continue
				}
				router.RegisterChannel(account.ChannelType + ":" + account.AccountKey)
			}
			router.RegisterChannel("websocket")
			log.Info("Inbound router registered",
				zap.Int("channel_count", len(accountList)+1))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			router.UnregisterAll()
			return nil
		},
	})
}
