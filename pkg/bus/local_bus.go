// Package bus provides a message bus for routing messages between channels and agents.
package bus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"nekobot/pkg/logger"
)

// LocalBus is a local in-process message bus using Go channels.
type LocalBus struct {
	log      *logger.Logger
	handlers map[string][]Handler // Channel ID -> handlers
	mu       sync.RWMutex

	// Channels for message flow
	inbound  chan *Message
	outbound chan *Message

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	messagesIn  uint64
	messagesOut uint64
	errors      uint64
	metricsLock sync.RWMutex
}

// NewLocalBus creates a new local message bus.
func NewLocalBus(log *logger.Logger, bufferSize int) *LocalBus {
	if bufferSize <= 0 {
		bufferSize = 100
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &LocalBus{
		log:      log,
		handlers: make(map[string][]Handler),
		inbound:  make(chan *Message, bufferSize),
		outbound: make(chan *Message, bufferSize),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start starts the message bus processing loops.
func (b *LocalBus) Start() error {
	b.log.Info("Starting message bus")

	// Start inbound processor
	b.wg.Add(1)
	go b.processInbound()

	// Start outbound processor
	b.wg.Add(1)
	go b.processOutbound()

	return nil
}

// Stop stops the message bus and waits for all processing to complete.
func (b *LocalBus) Stop() error {
	b.log.Info("Stopping message bus")

	// Cancel context to signal shutdown
	b.cancel()

	// Close channels
	close(b.inbound)
	close(b.outbound)

	// Wait for processors to finish
	b.wg.Wait()

	b.log.Info("Message bus stopped")
	return nil
}

// RegisterHandler registers a handler for a specific channel.
// Multiple handlers can be registered for the same channel.
func (b *LocalBus) RegisterHandler(channelID string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[channelID] = append(b.handlers[channelID], handler)
	b.log.Info("Registered handler", zap.String("channel", channelID))
}

// UnregisterHandlers removes all handlers for a channel.
func (b *LocalBus) UnregisterHandlers(channelID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.handlers, channelID)
	b.log.Info("Unregistered handlers", zap.String("channel", channelID))
}

// SendInbound sends an inbound message (from channel to agent).
func (b *LocalBus) SendInbound(msg *Message) error {
	select {
	case b.inbound <- msg:
		b.incrementMessagesIn()
		return nil
	case <-b.ctx.Done():
		return fmt.Errorf("bus is shutting down")
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending inbound message")
	}
}

// SendOutbound sends an outbound message (from agent to channel).
func (b *LocalBus) SendOutbound(msg *Message) error {
	select {
	case b.outbound <- msg:
		b.incrementMessagesOut()
		return nil
	case <-b.ctx.Done():
		return fmt.Errorf("bus is shutting down")
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending outbound message")
	}
}

// processInbound processes inbound messages.
func (b *LocalBus) processInbound() {
	defer b.wg.Done()

	for {
		select {
		case msg, ok := <-b.inbound:
			if !ok {
				return
			}

			b.handleMessage(msg, "inbound")

		case <-b.ctx.Done():
			return
		}
	}
}

// processOutbound processes outbound messages.
func (b *LocalBus) processOutbound() {
	defer b.wg.Done()

	for {
		select {
		case msg, ok := <-b.outbound:
			if !ok {
				return
			}

			b.handleMessage(msg, "outbound")

		case <-b.ctx.Done():
			return
		}
	}
}

// handleMessage dispatches a message to registered handlers.
func (b *LocalBus) handleMessage(msg *Message, direction string) {
	b.mu.RLock()
	handlers := b.handlers[msg.ChannelID]
	b.mu.RUnlock()

	if len(handlers) == 0 {
		b.log.Warn("No handlers registered for channel",
			zap.String("channel", msg.ChannelID),
			zap.String("direction", direction),
			zap.String("message_id", msg.ID))
		return
	}

	b.log.Debug("Processing message",
		zap.String("channel", msg.ChannelID),
		zap.String("direction", direction),
		zap.String("message_id", msg.ID),
		zap.String("session", msg.SessionID))

	// Execute handlers
	for _, handler := range handlers {
		if err := handler(b.ctx, msg); err != nil {
			b.incrementErrors()
			b.log.Error("Handler error",
				zap.String("channel", msg.ChannelID),
				zap.String("message_id", msg.ID),
				zap.Error(err))
		}
	}
}

// GetMetrics returns current bus metrics.
func (b *LocalBus) GetMetrics() map[string]uint64 {
	b.metricsLock.RLock()
	defer b.metricsLock.RUnlock()

	return map[string]uint64{
		"messages_in":  b.messagesIn,
		"messages_out": b.messagesOut,
		"errors":       b.errors,
	}
}

func (b *LocalBus) incrementMessagesIn() {
	b.metricsLock.Lock()
	b.messagesIn++
	b.metricsLock.Unlock()
}

func (b *LocalBus) incrementMessagesOut() {
	b.metricsLock.Lock()
	b.messagesOut++
	b.metricsLock.Unlock()
}

func (b *LocalBus) incrementErrors() {
	b.metricsLock.Lock()
	b.errors++
	b.metricsLock.Unlock()
}
