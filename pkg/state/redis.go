package state

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"nekobot/pkg/logger"
)

// RedisStore is a Redis-based key-value store.
type RedisStore struct {
	log    *logger.Logger
	client *redis.Client
	prefix string
}

// RedisStoreConfig configures the Redis store.
type RedisStoreConfig struct {
	Addr     string // Redis address (host:port)
	Password string // Redis password
	DB       int    // Redis database number
	Prefix   string // Key prefix for namespacing
}

// NewRedisStore creates a new Redis-based state store.
func NewRedisStore(log *logger.Logger, cfg *RedisStoreConfig) (*RedisStore, error) {
	if cfg.Prefix == "" {
		cfg.Prefix = "nekobot:"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connecting to Redis: %w", err)
	}

	s := &RedisStore{
		log:    log,
		client: client,
		prefix: cfg.Prefix,
	}

	log.Info("Connected to Redis",
		zap.String("addr", cfg.Addr),
		zap.Int("db", cfg.DB),
		zap.String("prefix", cfg.Prefix))

	return s, nil
}

// prefixKey adds the namespace prefix to a key.
func (s *RedisStore) prefixKey(key string) string {
	return s.prefix + key
}

// unprefixKey removes the namespace prefix from a key.
func (s *RedisStore) unprefixKey(key string) string {
	return strings.TrimPrefix(key, s.prefix)
}

// Get retrieves a value from the store.
func (s *RedisStore) Get(ctx context.Context, key string) (interface{}, bool, error) {
	val, err := s.client.Get(ctx, s.prefixKey(key)).Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis get: %w", err)
	}

	// Try to unmarshal as JSON
	var result interface{}
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		// If unmarshal fails, return as string
		return val, true, nil
	}

	return result, true, nil
}

// GetString retrieves a string value.
func (s *RedisStore) GetString(ctx context.Context, key string) (string, bool, error) {
	value, exists, err := s.Get(ctx, key)
	if err != nil || !exists {
		return "", false, err
	}

	str, ok := value.(string)
	return str, ok, nil
}

// GetInt retrieves an integer value.
func (s *RedisStore) GetInt(ctx context.Context, key string) (int, bool, error) {
	value, exists, err := s.Get(ctx, key)
	if err != nil || !exists {
		return 0, false, err
	}

	// JSON unmarshaling converts numbers to float64
	if f, ok := value.(float64); ok {
		return int(f), true, nil
	}

	i, ok := value.(int)
	return i, ok, nil
}

// GetBool retrieves a boolean value.
func (s *RedisStore) GetBool(ctx context.Context, key string) (bool, bool, error) {
	value, exists, err := s.Get(ctx, key)
	if err != nil || !exists {
		return false, false, err
	}

	b, ok := value.(bool)
	return b, ok, nil
}

// GetMap retrieves a map value.
func (s *RedisStore) GetMap(ctx context.Context, key string) (map[string]interface{}, bool, error) {
	value, exists, err := s.Get(ctx, key)
	if err != nil || !exists {
		return nil, false, err
	}

	m, ok := value.(map[string]interface{})
	return m, ok, nil
}

// Set stores a value.
func (s *RedisStore) Set(ctx context.Context, key string, value interface{}) error {
	// Marshal to JSON
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshaling value: %w", err)
	}

	if err := s.client.Set(ctx, s.prefixKey(key), data, 0).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}

// Delete removes a value.
func (s *RedisStore) Delete(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, s.prefixKey(key)).Err(); err != nil {
		return fmt.Errorf("redis del: %w", err)
	}
	return nil
}

// Keys returns all keys in the store.
func (s *RedisStore) Keys(ctx context.Context) ([]string, error) {
	pattern := s.prefix + "*"
	keys, err := s.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("redis keys: %w", err)
	}

	// Remove prefix from keys
	result := make([]string, len(keys))
	for i, key := range keys {
		result[i] = s.unprefixKey(key)
	}

	return result, nil
}

// Exists checks if a key exists.
func (s *RedisStore) Exists(ctx context.Context, key string) (bool, error) {
	count, err := s.client.Exists(ctx, s.prefixKey(key)).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists: %w", err)
	}
	return count > 0, nil
}

// Clear removes all data from the store.
func (s *RedisStore) Clear(ctx context.Context) error {
	// Get all keys with prefix
	pattern := s.prefix + "*"
	keys, err := s.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("redis keys: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	// Delete all keys
	if err := s.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis del: %w", err)
	}

	s.log.Info("Cleared Redis state", zap.Int("keys", len(keys)))
	return nil
}

// GetAll returns a copy of all data.
func (s *RedisStore) GetAll(ctx context.Context) (map[string]interface{}, error) {
	keys, err := s.Keys(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{}, len(keys))
	for _, key := range keys {
		value, exists, err := s.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if exists {
			result[key] = value
		}
	}

	return result, nil
}

// UpdateFunc atomically updates a value using a function.
func (s *RedisStore) UpdateFunc(ctx context.Context, key string, updateFn func(current interface{}) interface{}) error {
	prefixedKey := s.prefixKey(key)

	// Use Redis transaction for atomicity
	txf := func(tx *redis.Tx) error {
		// Get current value
		val, err := tx.Get(ctx, prefixedKey).Result()
		var current interface{}
		if err != redis.Nil {
			if err != nil {
				return err
			}
			// Unmarshal current value
			if err := json.Unmarshal([]byte(val), &current); err != nil {
				current = val
			}
		}

		// Apply update function
		newValue := updateFn(current)

		// Marshal new value
		data, err := json.Marshal(newValue)
		if err != nil {
			return fmt.Errorf("marshaling value: %w", err)
		}

		// Set new value
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, prefixedKey, data, 0)
			return nil
		})
		return err
	}

	// Execute transaction with watch
	if err := s.client.Watch(ctx, txf, prefixedKey); err != nil {
		return fmt.Errorf("redis transaction: %w", err)
	}

	return nil
}

// Close closes the Redis connection.
func (s *RedisStore) Close() error {
	return s.client.Close()
}
