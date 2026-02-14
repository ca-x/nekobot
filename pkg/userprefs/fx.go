package userprefs

import (
	"go.uber.org/fx"

	"nekobot/pkg/state"
)

// Module provides user preferences manager.
var Module = fx.Module("userprefs",
	fx.Provide(provideManager),
)

type managerParams struct {
	fx.In

	Store state.KV `optional:"true"`
}

func provideManager(p managerParams) *Manager {
	if p.Store == nil {
		return nil
	}
	return New(p.Store)
}
