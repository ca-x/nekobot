package session

import (
	"go.uber.org/fx"

	"nekobot/pkg/config"
)

// Module provides session management for fx.
var Module = fx.Module("session",
	fx.Provide(func(cfg *config.Config) *Manager {
		return NewManager(cfg.WorkspacePath() + "/sessions")
	}),
)
