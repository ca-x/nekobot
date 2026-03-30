package runtimetopology

import "go.uber.org/fx"

// Module provides runtime topology aggregation.
var Module = fx.Module("runtimetopology",
	fx.Provide(NewService),
)
