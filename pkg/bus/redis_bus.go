package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"nekobot/pkg/logger"
)

// RedisBus is a Redis-based message bus using pub/sub.
type RedisBus struct {
	log    *logger.Logger
	client *redis.Client
	prefix string

	handlers map[string][]Handler // Channel ID -> handlers
	mu       sync.RWMutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Pub/Sub
	pubsub *redis.PubSub

	// Metrics
	messagesIn  uint64
	messagesOut uint64
	errors      uint64
	metricsLock sync.RWMutex
}

// RedisBusConfig configures the Redis bus.
type RedisBusConfig struct {
	Addr     string
	Password string
	DB       int
	Prefix   string
}

// NewRedisBus creates a new Redis-based message bus.
func NewRedisBus(log *logger.Logger, cfg *RedisBusConfig) (*RedisBus, error) {
	if cfg.Prefix == "" {
		cfg.Prefix = "nekobot:bus:"
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

	ctx, cancel := context.WithCancel(context.Background())

	b := &RedisBus{
		log:      log,
		client:   client,
		prefix:   cfg.Prefix,
		handlers: make(map[string][]Handler),
		ctx:      ctx,
		cancel:   cancel,
	}

	log.Info("Redis bus initialized",
		zap.String("addr", cfg.Addr),
		zap.Int("db", cfg.DB),
		zap.String("prefix", cfg.Prefix))

	return b, nil
}

// Start starts the Redis bus.
func (b *RedisBus) Start() error {
	b.log.Info("Starting Redis message bus")

	// Subscribe to inbound and outbound channels
	b.pubsub = b.client.PSubscribe(b.ctx, b.prefix+"*")

	// Start message processor
	b.wg.Add(1)
	go b.processMessages()

	return nil
}

// Stop stops the Redis bus.
func (b *RedisBus) Stop() error {
	b.log.Info("Stopping Redis message bus")

	// Cancel context
	b.cancel()

	// Close pubsub
	if b.pubsub != nil {
		b.pubsub.Close()
	}

	// Wait for processors
	b.wg.Wait()

	// Close client
	b.client.Close()

	b.log.Info("Redis message bus stopped")
	return nil
}

// RegisterHandler registers a handler for a specific channel.
func (b *RedisBus) RegisterHandler(channelID string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[channelID] = append(b.handlers[channelID], handler)
	b.log.Info("Registered handler", zap.String("channel", channelID))
}

// UnregisterHandlers removes all handlers for a channel.
func (b *RedisBus) UnregisterHandlers(channelID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.handlers, channelID)
	b.log.Info("Unregistered handlers", zap.String("channel", channelID))
}

// SendInbound sends an inbound message (from channel to agent).
func (b *RedisBus) SendInbound(msg *Message) error {
	channel := b.prefix + "inbound:" + msg.ChannelID

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	if err := b.client.Publish(b.ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("publishing to Redis: %w", err)
	}

	b.incrementMessagesIn()
	return nil
}

// SendOutbound sends an outbound message (from agent to channel).
func (b *RedisBus) SendOutbound(msg *Message) error {
	channel := b.prefix + "outbound:" + msg.ChannelID

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	if err := b.client.Publish(b.ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("publishing to Redis: %w", err)
	}

	b.incrementMessagesOut()
	return nil
}

// GetMetrics returns current bus metrics.
func (b *RedisBus) GetMetrics() map[string]uint64 {
	b.metricsLock.RLock()
	defer b.metricsLock.RUnlock()

	return map[string]uint64{
		"messages_in":  b.messagesIn,
		"messages_out": b.messagesOut,
		"errors":       b.errors,
	}
}

// processMessages processes messages from Redis pub/sub.
func (b *RedisBus) processMessages() {
	defer b.wg.Done()

	ch := b.pubsub.Channel()

	for {
		select {
		case redisMsg, ok := <-ch:
			if !ok {
				return
			}

			b.handleRedisMessage(redisMsg)

		case <-b.ctx.Done():
			return
		}
	}
}

// handleRedisMessage handles a Redis pub/sub message.
func (b *RedisBus) handleRedisMessage(redisMsg *redis.Message) {
	// Parse message
	var msg Message
	if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
		b.log.Error("Failed to unmarshal message", zap.Error(err))
		b.incrementErrors()
		return
	}

	// Determine direction from channel name
	channel := redisMsg.Channel
	var direction string
	if len(channel) > len(b.prefix+"inbound:") && channel[:len(b.prefix+"inbound:")] == b.prefix+"inbound:" {
		direction = "inbound"
	} else if len(channel) > len(b.prefix+"outbound:") && channel[:len(b.prefix+"outbound:")] == b.prefix+"outbound:" {
		direction = "outbound"
	} else {
		b.log.Warn("Unknown channel format", zap.String("channel", channel))
		return
	}

	// Dispatch to handlers
	b.mu.RLock()
	handlers := b.handlers[msg.ChannelID]
	b.mu.RUnlock()

	if len(handlers) == 0 {
		b.log.Debug("No handlers registered for channel",
			zap.String("channel", msg.ChannelID),
			zap.String("direction", direction))
		return
	}

	b.log.Debug("Processing message",
		zap.String("channel", msg.ChannelID),
		zap.String("direction", direction),
		zap.String("message_id", msg.ID))

	// Execute handlers
	for _, handler := range handlers {
		if err := handler(b.ctx, &msg); err != nil {
			b.incrementErrors()
			b.log.Error("Handler error",
				zap.String("channel", msg.ChannelID),
				zap.String("message_id", msg.ID),
				zap.Error(err))
		}
	}
}

func (b *RedisBus) incrementMessagesIn() {
	b.metricsLock.Lock()
	b.messagesIn++
	b.metricsLock.Unlock()
}

func (b *RedisBus) incrementMessagesOut() {
	b.metricsLock.Lock()
	b.messagesOut++
	b.metricsLock.Unlock()
}

func (b *RedisBus) incrementErrors() {
	b.metricsLock.Lock()
	b.errors++
	b.metricsLock.Unlock()
}
