// Package qmd provides the main QMD manager.
package qmd

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

// Manager manages QMD collections and operations.
type Manager struct {
	log      *logger.Logger
	config   Config
	executor *ProcessExecutor
	collections map[string]*Collection
	mu       sync.RWMutex
	available bool
	version  string
}

// NewManager creates a new QMD manager.
func NewManager(log *logger.Logger, config Config) *Manager {
	// Parse timeout
	timeout := 30 * time.Second
	if config.Update.CommandTimeout != "" {
		if d, err := time.ParseDuration(config.Update.CommandTimeout); err == nil {
			timeout = d
		}
	}

	executor := NewProcessExecutor(log, config.Command, timeout)

	mgr := &Manager{
		log:         log,
		config:      config,
		executor:    executor,
		collections: make(map[string]*Collection),
	}

	// Check QMD availability
	mgr.checkAvailability()

	return mgr
}

// checkAvailability checks if QMD is installed.
func (m *Manager) checkAvailability() {
	m.available, m.version, _ = m.executor.CheckAvailable()
	if !m.available {
		m.log.Warn("QMD not available, semantic search disabled")
		return
	}

	m.log.Info("QMD available",
		zap.String("version", m.version))
}

// Initialize initializes QMD collections.
func (m *Manager) Initialize(ctx context.Context, workspaceDir string) error {
	if !m.config.Enabled {
		m.log.Debug("QMD integration disabled")
		return nil
	}

	if !m.available {
		m.log.Warn("QMD not available, skipping initialization")
		return nil
	}

	// Initialize default workspace collection
	if m.config.IncludeDefault {
		memoryPath := filepath.Join(workspaceDir, "memory")
		if err := m.EnsureCollection(ctx, "default", memoryPath, "**/*.md"); err != nil {
			m.log.Warn("Failed to initialize default collection", zap.Error(err))
		}
	}

	// Initialize custom collections
	for _, pathCfg := range m.config.Paths {
		// Expand path
		path := os.ExpandEnv(pathCfg.Path)
		path = expandHome(path)

		if err := m.EnsureCollection(ctx, pathCfg.Name, path, pathCfg.Pattern); err != nil {
			m.log.Warn("Failed to initialize collection",
				zap.String("name", pathCfg.Name),
				zap.Error(err))
		}
	}

	// Initialize sessions collection
	if m.config.Sessions.Enabled {
		sessionsPath := m.config.Sessions.ExportDir
		if sessionsPath != "" {
			sessionsPath = os.ExpandEnv(sessionsPath)
			sessionsPath = expandHome(sessionsPath)

			if err := m.EnsureCollection(ctx, "sessions", sessionsPath, "**/*.md"); err != nil {
				m.log.Warn("Failed to initialize sessions collection", zap.Error(err))
			}
		}
	}

	m.log.Info("QMD initialized",
		zap.Int("collections", len(m.collections)))

	return nil
}

// EnsureCollection ensures a collection exists, creating it if necessary.
func (m *Manager) EnsureCollection(ctx context.Context, name, path, pattern string) error {
	if !m.available {
		return fmt.Errorf("qmd not available")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if collection already exists
	if _, exists := m.collections[name]; exists {
		return nil
	}

	// Create the collection
	if err := m.executor.CreateCollection(ctx, name, path, pattern); err != nil {
		return err
	}

	// Add to our tracking
	m.collections[name] = &Collection{
		Name:         name,
		Path:         path,
		Pattern:      pattern,
		LastUpdated:  time.Now(),
	}

	return nil
}

// UpdateCollection updates a specific collection.
func (m *Manager) UpdateCollection(ctx context.Context, name string) error {
	if !m.available {
		return fmt.Errorf("qmd not available")
	}

	if err := m.executor.UpdateCollection(ctx, name); err != nil {
		return err
	}

	m.mu.Lock()
	if coll, exists := m.collections[name]; exists {
		coll.LastUpdated = time.Now()
	}
	m.mu.Unlock()

	return nil
}

// UpdateAll updates all collections.
func (m *Manager) UpdateAll(ctx context.Context) error {
	if !m.available {
		return fmt.Errorf("qmd not available")
	}

	m.mu.RLock()
	names := make([]string, 0, len(m.collections))
	for name := range m.collections {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		if err := m.UpdateCollection(ctx, name); err != nil {
			m.log.Warn("Failed to update collection",
				zap.String("name", name),
				zap.Error(err))
		}
	}

	m.log.Info("Updated all QMD collections", zap.Int("count", len(names)))
	return nil
}

// Search performs a semantic search across collections.
func (m *Manager) Search(ctx context.Context, collectionName, query string, limit int) ([]SearchResult, error) {
	if !m.available {
		return nil, fmt.Errorf("qmd not available")
	}

	output, err := m.executor.Search(ctx, collectionName, query, limit)
	if err != nil {
		return nil, err
	}

	// Parse JSON output
	var results []SearchResult
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("parsing search results: %w", err)
	}

	return results, nil
}

// DeleteCollection deletes a collection.
func (m *Manager) DeleteCollection(ctx context.Context, name string) error {
	if !m.available {
		return fmt.Errorf("qmd not available")
	}

	if err := m.executor.DeleteCollection(ctx, name); err != nil {
		return err
	}

	m.mu.Lock()
	delete(m.collections, name)
	m.mu.Unlock()

	return nil
}

// GetStatus returns the current QMD status.
func (m *Manager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	collections := make([]Collection, 0, len(m.collections))
	var lastUpdate time.Time

	for _, coll := range m.collections {
		collections = append(collections, *coll)
		if coll.LastUpdated.After(lastUpdate) {
			lastUpdate = coll.LastUpdated
		}
	}

	status := Status{
		Available:   m.available,
		Version:     m.version,
		Collections: collections,
		LastUpdate:  lastUpdate,
	}

	if !m.available {
		status.Error = "QMD not installed or not found in PATH"
	}

	return status
}

// IsAvailable returns whether QMD is available.
func (m *Manager) IsAvailable() bool {
	return m.available
}

// expandHome expands ~ to home directory.
func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}
	return path
}
