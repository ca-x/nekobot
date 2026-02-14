package channels

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/logger"
)

// Manager manages all communication channels.
type Manager struct {
	log      *logger.Logger
	bus      bus.Bus // Use interface directly, not pointer to interface
	channels map[string]Channel
	mu       sync.RWMutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates a new channel manager.
func NewManager(log *logger.Logger, messageBus bus.Bus) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		log:      log,
		bus:      messageBus,
		channels: make(map[string]Channel),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Register registers a channel with the manager.
func (m *Manager) Register(channel Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := channel.ID()
	if _, exists := m.channels[id]; exists {
		return fmt.Errorf("channel %s already registered", id)
	}

	m.channels[id] = channel
	m.log.Info("Registered channel",
		zap.String("id", id),
		zap.String("name", channel.Name()))

	return nil
}

// Unregister removes a channel from the manager.
func (m *Manager) Unregister(channelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.channels[channelID]; !exists {
		return fmt.Errorf("channel %s not found", channelID)
	}

	delete(m.channels, channelID)
	m.log.Info("Unregistered channel", zap.String("id", channelID))

	return nil
}

// Start starts all enabled channels.
func (m *Manager) Start() error {
	m.log.Info("Starting channel manager")

	m.mu.RLock()
	channels := make([]Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		if ch.IsEnabled() {
			channels = append(channels, ch)
		}
	}
	m.mu.RUnlock()

	// Start each enabled channel
	for _, ch := range channels {
		channel := ch // Capture for goroutine

		// Register message handler for this channel
		m.bus.RegisterHandler(channel.ID(), func(ctx context.Context, msg *bus.Message) error {
			return channel.SendMessage(ctx, msg)
		})

		// Start channel
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()

			m.log.Info("Starting channel",
				zap.String("id", channel.ID()),
				zap.String("name", channel.Name()))

			if err := channel.Start(m.ctx); err != nil {
				m.log.Error("Channel start failed",
					zap.String("channel", channel.ID()),
					zap.Error(err))
			}
		}()
	}

	if len(channels) == 0 {
		m.log.Warn("No channels enabled")
	} else {
		m.log.Info("Started channels", zap.Int("count", len(channels)))
	}

	return nil
}

// Stop stops all channels gracefully.
func (m *Manager) Stop() error {
	m.log.Info("Stopping channel manager")

	// Cancel context to signal all channels to stop
	m.cancel()

	// Stop each channel
	m.mu.RLock()
	channels := make([]Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		channels = append(channels, ch)
	}
	m.mu.RUnlock()

	// Stop all channels
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, ch := range channels {
		if err := ch.Stop(ctx); err != nil {
			m.log.Error("Error stopping channel",
				zap.String("channel", ch.ID()),
				zap.Error(err))
		}

		// Unregister handler from bus
		m.bus.UnregisterHandlers(ch.ID())
	}

	// Wait for all channel goroutines to finish
	m.wg.Wait()

	m.log.Info("Channel manager stopped")
	return nil
}

// StopChannel stops and unregisters a specific channel.
func (m *Manager) StopChannel(channelID string) error {
	m.mu.RLock()
	ch, exists := m.channels[channelID]
	m.mu.RUnlock()
	if !exists {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := ch.Stop(ctx); err != nil {
		m.log.Error("Error stopping channel",
			zap.String("channel", channelID),
			zap.Error(err))
	}

	m.bus.UnregisterHandlers(channelID)

	m.mu.Lock()
	delete(m.channels, channelID)
	m.mu.Unlock()

	m.log.Info("Stopped channel", zap.String("id", channelID))
	return nil
}

// ReloadChannel replaces an existing channel and starts the new one if enabled.
func (m *Manager) ReloadChannel(channel Channel) error {
	if channel == nil {
		return fmt.Errorf("channel cannot be nil")
	}

	id := channel.ID()
	if err := m.StopChannel(id); err != nil {
		return err
	}

	m.mu.Lock()
	m.channels[id] = channel
	m.mu.Unlock()

	m.log.Info("Reloaded channel",
		zap.String("id", channel.ID()),
		zap.String("name", channel.Name()),
		zap.Bool("enabled", channel.IsEnabled()))

	if !channel.IsEnabled() {
		return nil
	}

	m.bus.RegisterHandler(channel.ID(), func(ctx context.Context, msg *bus.Message) error {
		return channel.SendMessage(ctx, msg)
	})

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		if err := channel.Start(m.ctx); err != nil {
			m.log.Error("Channel start failed after reload",
				zap.String("channel", channel.ID()),
				zap.Error(err))
		}
	}()

	return nil
}

// GetChannel returns a channel by ID.
func (m *Manager) GetChannel(channelID string) (Channel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channel, exists := m.channels[channelID]
	if !exists {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	return channel, nil
}

// ListChannels returns all registered channels.
func (m *Manager) ListChannels() []Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := make([]Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		channels = append(channels, ch)
	}

	return channels
}

// GetEnabledChannels returns all enabled channels.
func (m *Manager) GetEnabledChannels() []Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := make([]Channel, 0)
	for _, ch := range m.channels {
		if ch.IsEnabled() {
			channels = append(channels, ch)
		}
	}

	return channels
}
