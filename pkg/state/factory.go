package state

import (
	"fmt"
	"time"

	"nekobot/pkg/logger"
)

// NewKV creates a new KV store based on configuration.
func NewKV(log *logger.Logger, cfg *Config) (KV, error) {
	switch cfg.Backend {
	case BackendFile:
		saveInterval := time.Duration(cfg.SaveIntervalS) * time.Second
		if saveInterval == 0 {
			saveInterval = 5 * time.Second
		}

		return NewFileStore(log, &FileStoreConfig{
			FilePath:     cfg.FilePath,
			AutoSave:     cfg.AutoSave,
			SaveInterval: saveInterval,
		})

	case BackendRedis:
		if cfg.RedisAddr == "" {
			return nil, fmt.Errorf("redis address is required")
		}

		return NewRedisStore(log, &RedisStoreConfig{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
			Prefix:   cfg.RedisPrefix,
		})

	default:
		return nil, fmt.Errorf("unknown backend type: %s", cfg.Backend)
	}
}
