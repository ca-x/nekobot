package approval

import (
	"go.uber.org/fx"
	"nekobot/pkg/config"
)

// Module provides the approval manager for fx dependency injection.
var Module = fx.Module("approval",
	fx.Provide(ProvideManager),
)

// ProvideManager creates an approval manager from config.
func ProvideManager(cfg *config.Config) *Manager {
	return NewManager(Config{
		Mode:      Mode(cfg.Approval.Mode),
		Allowlist: cfg.Approval.Allowlist,
		Denylist:  cfg.Approval.Denylist,
	})
}
