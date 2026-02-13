package providers

import (
	"go.uber.org/fx"
)

// Module provides provider client for fx.
var Module = fx.Module("providers",
	fx.Provide(NewClient),
	fx.Provide(NewLoadBalancer),
)
