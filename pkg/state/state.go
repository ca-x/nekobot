// Package state provides persistent state management with atomic operations.
package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"

	"nekobot/pkg/logger"
)

// FileStore is a file-based key-value store with atomic operations.
type FileStore struct {
	log      *logger.Logger
	filePath string
	data     map[string]interface{}
	mu       sync.RWMutex

	// Auto-save configuration
	autoSave      bool
	saveInterval  time.Duration
	saveTicker    *time.Ticker
	stopSave      chan struct{}
	pendingWrites bool
}

// FileStoreConfig configures the file store.
type FileStoreConfig struct {
	FilePath     string        // Path to state file
	AutoSave     bool          // Enable auto-save
	SaveInterval time.Duration // Auto-save interval (default: 5s)
}

// NewFileStore creates a new file-based state store.
func NewFileStore(log *logger.Logger, cfg *FileStoreConfig) (*FileStore, error) {
	if cfg.SaveInterval == 0 {
		cfg.SaveInterval = 5 * time.Second
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(cfg.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating state directory: %w", err)
	}

	s := &FileStore{
		log:          log,
		filePath:     cfg.FilePath,
		data:         make(map[string]interface{}),
		autoSave:     cfg.AutoSave,
		saveInterval: cfg.SaveInterval,
		stopSave:     make(chan struct{}),
	}

	// Load existing state
	if err := s.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("loading state: %w", err)
	}

	// Start auto-save if enabled
	if s.autoSave {
		s.startAutoSave()
	}

	return s, nil
}

// Get retrieves a value from the store.
func (s *FileStore) Get(ctx context.Context, key string) (interface{}, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.data[key]
	return value, exists, nil
}

// GetString retrieves a string value.
func (s *FileStore) GetString(ctx context.Context, key string) (string, bool, error) {
	value, exists, err := s.Get(ctx, key)
	if err != nil || !exists {
		return "", false, err
	}

	str, ok := value.(string)
	return str, ok, nil
}

// GetInt retrieves an integer value.
func (s *FileStore) GetInt(ctx context.Context, key string) (int, bool, error) {
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
func (s *FileStore) GetBool(ctx context.Context, key string) (bool, bool, error) {
	value, exists, err := s.Get(ctx, key)
	if err != nil || !exists {
		return false, false, err
	}

	b, ok := value.(bool)
	return b, ok, nil
}

// GetMap retrieves a map value.
func (s *FileStore) GetMap(ctx context.Context, key string) (map[string]interface{}, bool, error) {
	value, exists, err := s.Get(ctx, key)
	if err != nil || !exists {
		return nil, false, err
	}

	m, ok := value.(map[string]interface{})
	return m, ok, nil
}

// Set stores a value.
func (s *FileStore) Set(ctx context.Context, key string, value interface{}) error {
	s.mu.Lock()
	s.data[key] = value
	s.pendingWrites = true
	s.mu.Unlock()

	if !s.autoSave {
		return s.Save() // Immediate save if auto-save is disabled
	}
	return nil
}

// Delete removes a value.
func (s *FileStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	delete(s.data, key)
	s.pendingWrites = true
	s.mu.Unlock()

	if !s.autoSave {
		return s.Save()
	}
	return nil
}

// Keys returns all keys in the store.
func (s *FileStore) Keys(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys, nil
}

// Exists checks if a key exists.
func (s *FileStore) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.data[key]
	return exists, nil
}

// Clear removes all data from the store.
func (s *FileStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	s.data = make(map[string]interface{})
	s.pendingWrites = true
	s.mu.Unlock()

	return s.Save()
}

// Load loads state from disk.
func (s *FileStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &s.data); err != nil {
		return fmt.Errorf("unmarshaling state: %w", err)
	}

	s.log.Info("Loaded state", zap.String("file", s.filePath), zap.Int("keys", len(s.data)))
	return nil
}

// Save persists state to disk.
func (s *FileStore) Save() error {
	s.mu.RLock()
	// Quick check if there are pending writes
	if !s.pendingWrites && s.autoSave {
		s.mu.RUnlock()
		return nil
	}

	// Marshal data while holding read lock
	data, err := json.MarshalIndent(s.data, "", "  ")
	keyCount := len(s.data)
	s.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	// Write to temp file first for atomic save
	tempFile := s.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("writing temp state file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, s.filePath); err != nil {
		return fmt.Errorf("renaming temp state file: %w", err)
	}

	// Clear pending writes flag
	s.mu.Lock()
	s.pendingWrites = false
	s.mu.Unlock()

	s.log.Debug("Saved state", zap.String("file", s.filePath), zap.Int("keys", keyCount))
	return nil
}

// startAutoSave starts the auto-save goroutine.
func (s *FileStore) startAutoSave() {
	s.saveTicker = time.NewTicker(s.saveInterval)

	go func() {
		for {
			select {
			case <-s.saveTicker.C:
				if err := s.Save(); err != nil {
					s.log.Error("Auto-save failed", zap.Error(err))
				}
			case <-s.stopSave:
				return
			}
		}
	}()

	s.log.Info("Started auto-save", zap.Duration("interval", s.saveInterval))
}

// Close stops auto-save and performs a final save.
func (s *FileStore) Close() error {
	if s.autoSave && s.saveTicker != nil {
		s.saveTicker.Stop()
		close(s.stopSave)
	}

	// Final save
	return s.Save()
}

// GetAll returns a copy of all data.
func (s *FileStore) GetAll(ctx context.Context) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := make(map[string]interface{}, len(s.data))
	for k, v := range s.data {
		copy[k] = v
	}
	return copy, nil
}

// UpdateFunc atomically updates a value using a function.
func (s *FileStore) UpdateFunc(ctx context.Context, key string, updateFn func(current interface{}) interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.data[key]
	s.data[key] = updateFn(current)
	s.pendingWrites = true

	if !s.autoSave {
		return s.Save()
	}
	return nil
}
