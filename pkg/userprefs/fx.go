package userprefs

import (
	"context"
	"path/filepath"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/state"
)

// Module provides user preferences manager.
var Module = fx.Module("userprefs",
	fx.Provide(provideManager),
)

func provideManager(lc fx.Lifecycle, log *logger.Logger, cfg *config.Config) (*Manager, error) {
	storePath := filepath.Join(cfg.WorkspacePath(), "userprefs.json")

	store, err := state.NewFileStore(log, &state.FileStoreConfig{
		FilePath:     storePath,
		AutoSave:     true,
		SaveInterval: 2 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("User preferences store initialized", zap.String("path", storePath))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return store.Close()
		},
	})

	return New(store), nil
}
