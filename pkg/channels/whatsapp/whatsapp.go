// Package whatsapp provides WhatsApp channel implementation via bridge.
package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Channel implements WhatsApp channel using a WebSocket bridge.
type Channel struct {
	log      *logger.Logger
	config   config.WhatsAppConfig
	bus      bus.Bus
	commands *commands.Registry

	conn      *websocket.Conn
	mu        sync.Mutex
	connected bool
	running   bool
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewChannel creates a new WhatsApp channel.
func NewChannel(
	log *logger.Logger,
	cfg config.WhatsAppConfig,
	b bus.Bus,
	cmdRegistry *commands.Registry,
) (*Channel, error) {
	if cfg.BridgeURL == "" {
		return nil, fmt.Errorf("whatsapp bridge_url is required")
	}

	return &Channel{
		log:       log,
		config:    cfg,
		bus:       b,
		commands:  cmdRegistry,
		connected: false,
		running:   false,
	}, nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "whatsapp"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "WhatsApp"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// Start starts the WhatsApp channel and connects to the bridge.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting WhatsApp channel", zap.String("bridge_url", c.config.BridgeURL))

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Register outbound message handler
	c.bus.RegisterHandler("whatsapp", c.handleOutbound)

	// Connect to bridge
	if err := c.connect(); err != nil {
		return fmt.Errorf("connecting to WhatsApp bridge: %w", err)
	}

	// Start listening
	go c.listen()

	c.running = true
	c.log.Info("WhatsApp channel started")

	return nil
}

// Stop stops the WhatsApp channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping WhatsApp channel")

	c.running = false

	if c.cancel != nil {
		c.cancel()
	}

	// Unregister handler
	c.bus.UnregisterHandlers("whatsapp")

	// Close connection
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.log.Warn("Error closing WhatsApp connection", zap.Error(err))
		}
		c.conn = nil
	}

	c.connected = false
	c.log.Info("WhatsApp channel stopped")

	return nil
}

// connect establishes WebSocket connection to the bridge.
func (c *Channel) connect() error {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.Dial(c.config.BridgeURL, nil)
	if err != nil {
		return fmt.Errorf("dialing bridge: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()

	c.log.Info("Connected to WhatsApp bridge")
	return nil
}

// listen listens for messages from the bridge.
func (c *Channel) listen() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.mu.Lock()
			conn := c.conn
			c.mu.Unlock()

			if conn == nil {
				if c.running {
					// Try to reconnect
					c.log.Warn("WhatsApp connection lost, reconnecting...")
					time.Sleep(2 * time.Second)
					if err := c.connect(); err != nil {
						c.log.Error("Failed to reconnect", zap.Error(err))
					}
				}
				continue
			}

			// Read message
			_, message, err := conn.ReadMessage()
			if err != nil {
				c.log.Error("WhatsApp read error", zap.Error(err))
				c.mu.Lock()
				c.connected = false
				c.conn = nil
				c.mu.Unlock()
				time.Sleep(2 * time.Second)
				continue
			}

			// Parse message
			var msg map[string]interface{}
			if err := json.Unmarshal(message, &msg); err != nil {
				c.log.Warn("Failed to unmarshal message", zap.Error(err))
				continue
			}

			// Handle message based on type
			msgType, ok := msg["type"].(string)
			if !ok {
				continue
			}

			if msgType == "message" {
				c.handleInbound(msg)
			}
		}
	}
}

// handleInbound handles incoming messages from WhatsApp.
func (c *Channel) handleInbound(rawMsg map[string]interface{}) {
	// Extract fields
	senderID, ok := rawMsg["from"].(string)
	if !ok {
		return
	}

	chatID, ok := rawMsg["chat"].(string)
	if !ok {
		chatID = senderID
	}

	content, ok := rawMsg["content"].(string)
	if !ok {
		content = ""
	}

	// Check authorization
	if !c.isAllowed(senderID) {
		c.log.Warn("Unauthorized user",
			zap.String("user_id", senderID))
		return
	}

	// Check for slash commands
	if c.commands.IsCommand(content) {
		c.handleCommand(rawMsg, senderID, chatID, content)
		return
	}

	// Create bus message
	msg := &bus.Message{
		ID:        fmt.Sprintf("whatsapp:%v", rawMsg["id"]),
		ChannelID: "whatsapp",
		SessionID: fmt.Sprintf("whatsapp:%s", chatID),
		UserID:    senderID,
		Username:  getStringOrDefault(rawMsg, "from_name", senderID),
		Type:      bus.MessageTypeText,
		Content:   content,
		Timestamp: time.Now(),
	}

	// Send to bus
	if err := c.bus.SendInbound(msg); err != nil {
		c.log.Error("Failed to send inbound message", zap.Error(err))
	}
}

// handleCommand processes a command message.
func (c *Channel) handleCommand(rawMsg map[string]interface{}, senderID, chatID, content string) {
	cmdName, args := c.commands.Parse(content)

	cmd, exists := c.commands.Get(cmdName)
	if !exists {
		c.log.Debug("Unknown command", zap.String("command", cmdName))
		return
	}

	c.log.Info("Executing command",
		zap.String("command", cmdName),
		zap.String("user", senderID))

	// Create command request
	req := commands.CommandRequest{
		Channel:  "whatsapp",
		ChatID:   chatID,
		UserID:   senderID,
		Username: getStringOrDefault(rawMsg, "from_name", senderID),
		Command:  cmdName,
		Args:     args,
		Metadata: map[string]string{
			"message_id": getStringOrDefault(rawMsg, "id", ""),
		},
	}

	// Execute command
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cmd.Handler(ctx, req)
	if err != nil {
		c.log.Error("Command execution failed",
			zap.String("command", cmdName),
			zap.Error(err))

		// Send error response
		c.sendBridgeMessage(chatID, "❌ Command failed: "+err.Error())
		return
	}

	// Send response
	c.sendBridgeMessage(chatID, resp.Content)
}

// handleOutbound handles outbound messages from the bus.
func (c *Channel) handleOutbound(ctx context.Context, msg *bus.Message) error {
	return c.SendMessage(ctx, msg)
}

// SendMessage sends a message to WhatsApp via the bridge.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	// Extract chat ID from session ID (format: "whatsapp:chat_id")
	chatID := msg.SessionID
	if len(chatID) > 9 && chatID[:9] == "whatsapp:" {
		chatID = chatID[9:]
	}

	return c.sendBridgeMessage(chatID, msg.Content)
}

// sendBridgeMessage sends a message through the WebSocket bridge.
func (c *Channel) sendBridgeMessage(chatID, content string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("whatsapp connection not established")
	}

	payload := map[string]interface{}{
		"type":    "message",
		"to":      chatID,
		"content": content,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("sending message: %w", err)
	}

	c.log.Debug("Sent WhatsApp message",
		zap.String("chat_id", chatID),
		zap.Int("length", len(content)))

	return nil
}

// isAllowed checks if a user is allowed to use the bot.
func (c *Channel) isAllowed(userID string) bool {
	if len(c.config.AllowFrom) == 0 {
		return true
	}

	for _, allowed := range c.config.AllowFrom {
		if allowed == userID || allowed == "*" {
			return true
		}
	}

	return false
}

// getStringOrDefault safely extracts a string from a map.
func getStringOrDefault(m map[string]interface{}, key, defaultValue string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return defaultValue
}
