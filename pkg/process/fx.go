package process

import (
	"go.uber.org/fx"
)

// Module provides process manager for fx.
var Module = fx.Module("process",
	fx.Provide(NewManager),
)
