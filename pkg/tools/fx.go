package tools

import (
	"go.uber.org/fx"
)

// Module provides tools registry for fx.
// Note: Tools are registered in agent module.
var Module = fx.Module("tools")
