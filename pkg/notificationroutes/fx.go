package notificationroutes

import (
	"go.uber.org/fx"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/storage/ent"
)

// Module provides the notification routes manager via Uber FX.
var Module = fx.Module("notificationroutes",
	fx.Provide(NewManager),
)

// Dependencies for NewManager when used outside FX.
type Dependencies struct {
	fx.In

	Cfg    *config.Config
	Log    *logger.Logger
	Client *ent.Client
}
