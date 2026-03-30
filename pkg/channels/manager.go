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
	log            *logger.Logger
	bus            bus.Bus // Use interface directly, not pointer to interface
	channels       map[string]Channel
	channelsByType map[string][]string
	defaultByType  map[string]string
	started        bool
	mu             sync.RWMutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates a new channel manager.
func NewManager(log *logger.Logger, messageBus bus.Bus) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		log:            log,
		bus:            messageBus,
		channels:       make(map[string]Channel),
		channelsByType: make(map[string][]string),
		defaultByType:  make(map[string]string),
		ctx:            ctx,
		cancel:         cancel,
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

	channelType := channelTypeOf(channel)
	m.channels[id] = channel
	m.channelsByType[channelType] = append(m.channelsByType[channelType], id)
	if _, exists := m.defaultByType[channelType]; !exists {
		m.defaultByType[channelType] = id
	}
	m.log.Info("Registered channel",
		zap.String("id", id),
		zap.String("type", channelType),
		zap.String("name", channel.Name()))

	return nil
}

// Unregister removes a channel from the manager.
func (m *Manager) Unregister(channelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	resolvedID, channelType, ok := m.resolveRegisteredIDLocked(channelID)
	if !ok {
		return fmt.Errorf("channel %s not found", channelID)
	}

	delete(m.channels, resolvedID)
	m.removeTypeIndexLocked(channelType, resolvedID)
	m.log.Info("Unregistered channel", zap.String("id", resolvedID), zap.String("type", channelType))

	return nil
}

// Start starts all enabled channels.
func (m *Manager) Start() error {
	m.log.Info("Starting channel manager")
	m.mu.Lock()
	m.started = true
	m.mu.Unlock()

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
		if m.bus != nil {
			m.bus.RegisterOutboundHandler(channel.ID(), func(ctx context.Context, msg *bus.Message) error {
				return channel.SendMessage(ctx, msg)
			})
		}

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
	m.mu.Lock()
	m.started = false
	m.mu.Unlock()

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
		if m.bus != nil {
			m.bus.UnregisterOutboundHandlers(ch.ID())
		}
	}

	// Wait for all channel goroutines to finish
	m.wg.Wait()

	m.log.Info("Channel manager stopped")
	return nil
}

// StopChannel stops and unregisters a specific channel.
func (m *Manager) StopChannel(channelID string) error {
	m.mu.RLock()
	resolvedID, _, exists := m.resolveRegisteredIDLocked(channelID)
	var ch Channel
	if exists {
		ch = m.channels[resolvedID]
	}
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

	if m.bus != nil {
		m.bus.UnregisterOutboundHandlers(resolvedID)
	}

	m.mu.Lock()
	delete(m.channels, resolvedID)
	m.removeTypeIndexLocked(channelTypeOf(ch), resolvedID)
	m.mu.Unlock()

	m.log.Info("Stopped channel", zap.String("id", resolvedID))
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
	channelType := channelTypeOf(channel)
	ids := m.channelsByType[channelType]
	alreadyIndexed := false
	for _, existingID := range ids {
		if existingID == id {
			alreadyIndexed = true
			break
		}
	}
	if !alreadyIndexed {
		m.channelsByType[channelType] = append(ids, id)
	}
	if _, exists := m.defaultByType[channelType]; !exists {
		m.defaultByType[channelType] = id
	}
	started := m.started
	m.mu.Unlock()

	m.log.Info("Reloaded channel",
		zap.String("id", channel.ID()),
		zap.String("name", channel.Name()),
		zap.Bool("enabled", channel.IsEnabled()))

	if !started || !channel.IsEnabled() {
		return nil
	}

	if m.bus != nil {
		m.bus.RegisterOutboundHandler(channel.ID(), func(ctx context.Context, msg *bus.Message) error {
			return channel.SendMessage(ctx, msg)
		})
	}

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

	resolvedID, _, exists := m.resolveRegisteredIDLocked(channelID)
	if !exists {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	channel, exists := m.channels[resolvedID]
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

// ListChannelsByType returns all registered instances for one logical channel type.
func (m *Manager) ListChannelsByType(channelType string) []Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.channelsByType[channelType]
	result := make([]Channel, 0, len(ids))
	for _, id := range ids {
		ch, ok := m.channels[id]
		if !ok {
			continue
		}
		result = append(result, ch)
	}
	return result
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

func (m *Manager) resolveRegisteredIDLocked(key string) (string, string, bool) {
	if _, ok := m.channels[key]; ok {
		return key, channelTypeOf(m.channels[key]), true
	}
	id, ok := m.defaultByType[key]
	if !ok {
		return "", "", false
	}
	ch, exists := m.channels[id]
	if !exists {
		return "", "", false
	}
	return id, channelTypeOf(ch), true
}

func (m *Manager) removeTypeIndexLocked(channelType, channelID string) {
	ids := m.channelsByType[channelType]
	if len(ids) == 0 {
		delete(m.defaultByType, channelType)
		return
	}

	next := ids[:0]
	for _, id := range ids {
		if id == channelID {
			continue
		}
		next = append(next, id)
	}

	if len(next) == 0 {
		delete(m.channelsByType, channelType)
		delete(m.defaultByType, channelType)
		return
	}

	m.channelsByType[channelType] = next
	if m.defaultByType[channelType] == channelID {
		m.defaultByType[channelType] = next[0]
	}
}

func channelTypeOf(channel Channel) string {
	typed, ok := channel.(TypedChannel)
	if ok {
		if channelType := typed.ChannelType(); channelType != "" {
			return channelType
		}
	}
	return channel.ID()
}
