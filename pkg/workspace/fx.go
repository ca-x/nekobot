package workspace

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Module provides workspace functionality.
var Module = fx.Module("workspace",
	fx.Provide(ProvideManager),
	fx.Invoke(InitializeWorkspace),
)

// ProvideManager creates a workspace manager from configuration.
func ProvideManager(cfg *config.Config, log *logger.Logger) (*Manager, error) {
	workspaceDir := cfg.WorkspacePath()
	return NewManager(workspaceDir, log), nil
}

// InitializeWorkspace ensures workspace is initialized on startup.
func InitializeWorkspace(
	lc fx.Lifecycle,
	manager *Manager,
	log *logger.Logger,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Ensure workspace exists
			if err := manager.Ensure(); err != nil {
				log.Warn("Failed to initialize workspace, continuing anyway",
					zap.Error(err))
			} else {
				log.Info("Workspace initialized",
					zap.String("path", manager.GetWorkspaceDir()))
			}
			return nil
		},
	})
}
