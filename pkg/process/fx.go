package process

import (
	"go.uber.org/fx"

	"nekobot/pkg/logger"
)

// Module provides process manager for fx.
var Module = fx.Module("process",
	fx.Provide(NewManager),
)
