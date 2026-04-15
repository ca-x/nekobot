package goaldriven

import (
	"go.uber.org/fx"

	"nekobot/pkg/goaldriven/criteria"
	"nekobot/pkg/goaldriven/scope"
)

// Module provides the first GoalDriven vertical slice.
var Module = fx.Module("goaldriven",
	fx.Provide(
		NewMemoryStore,
		criteria.NewParser,
		criteria.NewSchema,
		scope.NewResolver,
		NewService,
	),
)
