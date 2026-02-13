// Package state provides persistent key-value storage with multiple backend support.
package state

import (
	"context"
)

// KV is the interface for key-value storage backends.
type KV interface {
	// Get retrieves a value from the store.
	Get(ctx context.Context, key string) (interface{}, bool, error)

	// GetString retrieves a string value.
	GetString(ctx context.Context, key string) (string, bool, error)

	// GetInt retrieves an integer value.
	GetInt(ctx context.Context, key string) (int, bool, error)

	// GetBool retrieves a boolean value.
	GetBool(ctx context.Context, key string) (bool, bool, error)

	// GetMap retrieves a map value.
	GetMap(ctx context.Context, key string) (map[string]interface{}, bool, error)

	// Set stores a value.
	Set(ctx context.Context, key string, value interface{}) error

	// Delete removes a value.
	Delete(ctx context.Context, key string) error

	// Keys returns all keys in the store.
	Keys(ctx context.Context) ([]string, error)

	// Exists checks if a key exists.
	Exists(ctx context.Context, key string) (bool, error)

	// Clear removes all data from the store.
	Clear(ctx context.Context) error

	// GetAll returns a copy of all data.
	GetAll(ctx context.Context) (map[string]interface{}, error)

	// UpdateFunc atomically updates a value using a function.
	UpdateFunc(ctx context.Context, key string, updateFn func(current interface{}) interface{}) error

	// Close closes the store and performs cleanup.
	Close() error
}

// BackendType represents the storage backend type.
type BackendType string

const (
	BackendFile  BackendType = "file"
	BackendRedis BackendType = "redis"
)

// Config configures the state store.
type Config struct {
	Backend BackendType // Storage backend (file or redis)

	// File backend config
	FilePath     string // Path to state file

	// Redis backend config
	RedisAddr     string // Redis address (host:port)
	RedisPassword string // Redis password
	RedisDB       int    // Redis database number
	RedisPrefix   string // Key prefix for namespacing

	// Common config
	AutoSave      bool   // Enable auto-save (file backend only)
	SaveIntervalS int    // Auto-save interval in seconds (file backend only)
}
