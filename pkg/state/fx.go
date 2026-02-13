package state

import (
	"context"
	"path/filepath"

	"go.uber.org/fx"

	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Module is the fx module for state management.
var Module = fx.Module("state",
	fx.Provide(NewKVStore),
)

// NewKVStore creates a new KV store for fx.
func NewKVStore(
	lc fx.Lifecycle,
	log *logger.Logger,
	cfg *config.Config,
) (KV, error) {
	// Determine state configuration
	stateConfig := &Config{
		Backend:  BackendFile, // Default to file backend
		FilePath: filepath.Join(cfg.Agents.Defaults.Workspace, "state.json"),
		AutoSave: true,
		SaveIntervalS: 5,
	}

	// Override with config if state settings exist
	if cfg.State.Backend != "" {
		stateConfig.Backend = BackendType(cfg.State.Backend)
	}
	if cfg.State.FilePath != "" {
		stateConfig.FilePath = cfg.State.FilePath
	}
	if cfg.State.RedisAddr != "" {
		stateConfig.RedisAddr = cfg.State.RedisAddr
		stateConfig.RedisPassword = cfg.State.RedisPassword
		stateConfig.RedisDB = cfg.State.RedisDB
		stateConfig.RedisPrefix = cfg.State.RedisPrefix
	}

	store, err := NewKV(log, stateConfig)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("State store initialized")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return store.Close()
		},
	})

	return store, nil
}
