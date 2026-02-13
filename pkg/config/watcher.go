package config

import (
	"fmt"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// ChangeHandler is a callback function called when configuration changes.
type ChangeHandler func(*Config) error

// Watcher monitors configuration file for changes and triggers reload.
type Watcher struct {
	loader   *Loader
	config   *Config
	handlers []ChangeHandler
	mu       sync.RWMutex
	stopCh   chan struct{}
	watching bool
}

// NewWatcher creates a new configuration watcher.
func NewWatcher(loader *Loader, config *Config) *Watcher {
	return &Watcher{
		loader:   loader,
		config:   config,
		handlers: make([]ChangeHandler, 0),
		stopCh:   make(chan struct{}),
	}
}

// AddHandler registers a handler to be called when configuration changes.
func (w *Watcher) AddHandler(handler ChangeHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers = append(w.handlers, handler)
}

// Start begins watching the configuration file for changes.
func (w *Watcher) Start() error {
	w.mu.Lock()
	if w.watching {
		w.mu.Unlock()
		return fmt.Errorf("watcher already started")
	}
	w.watching = true
	w.mu.Unlock()

	// Use viper's built-in watch functionality
	w.loader.viper.OnConfigChange(func(e fsnotify.Event) {
		// Reload configuration
		newConfig, err := w.loader.Load("")
		if err != nil {
			// Log error but continue watching
			fmt.Printf("Error reloading config: %v\n", err)
			return
		}

		// Update the config
		w.mu.Lock()
		w.config = newConfig
		w.mu.Unlock()

		// Notify handlers
		w.notifyHandlers(newConfig)
	})

	w.loader.viper.WatchConfig()

	return nil
}

// Stop stops watching the configuration file.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.watching {
		return
	}

	w.watching = false
	close(w.stopCh)
}

// GetConfig returns the current configuration (thread-safe).
func (w *Watcher) GetConfig() *Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.config
}

// notifyHandlers calls all registered handlers with the new configuration.
func (w *Watcher) notifyHandlers(config *Config) {
	w.mu.RLock()
	handlers := make([]ChangeHandler, len(w.handlers))
	copy(handlers, w.handlers)
	w.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(config); err != nil {
			// Log error but continue notifying other handlers
			fmt.Printf("Error in config change handler: %v\n", err)
		}
	}
}

// WatchOption configures the watcher behavior.
type WatchOption func(*Watcher)

// WithHandler adds a change handler to the watcher.
func WithHandler(handler ChangeHandler) WatchOption {
	return func(w *Watcher) {
		w.AddHandler(handler)
	}
}
