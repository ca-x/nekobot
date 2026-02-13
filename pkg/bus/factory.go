package bus

import (
	"fmt"

	"nekobot/pkg/logger"
)

// BusType represents the bus backend type.
type BusType string

const (
	BusTypeLocal BusType = "local"
	BusTypeRedis BusType = "redis"
)

// Config configures the bus.
type Config struct {
	Type       BusType // Bus type (local or redis)
	BufferSize int     // Buffer size for local bus

	// Redis config
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	RedisPrefix   string
}

// NewBus creates a new bus based on configuration.
func NewBus(log *logger.Logger, cfg *Config) (Bus, error) {
	switch cfg.Type {
	case BusTypeLocal, "":
		// Default to local bus
		return NewLocalBus(log, cfg.BufferSize), nil

	case BusTypeRedis:
		if cfg.RedisAddr == "" {
			return nil, fmt.Errorf("redis address is required for redis bus")
		}

		return NewRedisBus(log, &RedisBusConfig{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
			Prefix:   cfg.RedisPrefix,
		})

	default:
		return nil, fmt.Errorf("unknown bus type: %s", cfg.Type)
	}
}
