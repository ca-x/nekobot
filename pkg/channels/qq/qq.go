// Package qq provides QQ Bot channel implementation.
package qq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/openapi"
	"github.com/tencent-connect/botgo/token"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Channel implements QQ Bot channel using official botgo SDK.
type Channel struct {
	log      *logger.Logger
	config   config.QQConfig
	bus      bus.Bus
	commands *commands.Registry

	api            openapi.OpenAPI
	tokenSource    oauth2.TokenSource
	sessionManager botgo.SessionManager
	processedIDs   map[string]bool
	mu             sync.RWMutex
	running        bool
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewChannel creates a new QQ channel.
func NewChannel(
	log *logger.Logger,
	cfg config.QQConfig,
	b bus.Bus,
	cmdRegistry *commands.Registry,
) (*Channel, error) {
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return nil, fmt.Errorf("qq app_id and app_secret are required")
	}

	return &Channel{
		log:          log,
		config:       cfg,
		bus:          b,
		commands:     cmdRegistry,
		processedIDs: make(map[string]bool),
		running:      false,
	}, nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "qq"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "QQ"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// Start starts the QQ Bot channel with WebSocket connection.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting QQ Bot channel (WebSocket mode)")

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Register outbound message handler
	c.bus.RegisterHandler("qq", c.handleOutbound)

	// Create token source
	credentials := &token.QQBotCredentials{
		AppID:     c.config.AppID,
		AppSecret: c.config.AppSecret,
	}
	c.tokenSource = token.NewQQBotTokenSource(credentials)

	// Start token refresh
	if err := token.StartRefreshAccessToken(c.ctx, c.tokenSource); err != nil {
		return fmt.Errorf("starting token refresh: %w", err)
	}

	// Initialize OpenAPI client
	c.api = botgo.NewOpenAPI(c.config.AppID, c.tokenSource).WithTimeout(5 * time.Second)

	// Register event handlers
	intent := event.RegisterHandlers(
		c.handleC2CMessage(),
		c.handleGroupATMessage(),
	)

	// Get WebSocket info
	wsInfo, err := c.api.WS(c.ctx, nil, "")
	if err != nil {
		return fmt.Errorf("getting websocket info: %w", err)
	}

	c.log.Info("Got WebSocket info",
		zap.Int("shards", int(wsInfo.Shards)))

	// Create session manager
	c.sessionManager = botgo.NewSessionManager()

	// Start WebSocket in goroutine
	go func() {
		c.log.Info("Starting QQ WebSocket session")
		if err := c.sessionManager.Start(wsInfo, c.tokenSource, &intent); err != nil {
			c.log.Error("QQ WebSocket session error", zap.Error(err))
			c.running = false
		}
	}()

	c.running = true
	c.log.Info("QQ Bot channel started")
	return nil
}

// Stop stops the QQ Bot channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping QQ Bot channel")

	c.running = false

	if c.cancel != nil {
		c.cancel()
	}

	// Unregister handler
	c.bus.UnregisterHandlers("qq")

	c.log.Info("QQ Bot channel stopped")
	return nil
}

// handleC2CMessage handles QQ private messages.
func (c *Channel) handleC2CMessage() event.C2CMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSC2CMessageData) error {
		// Deduplication check
		if c.isDuplicate(data.ID) {
			return nil
		}

		// Extract sender info
		var senderID string
		if data.Author != nil && data.Author.ID != "" {
			senderID = data.Author.ID
		} else {
			c.log.Warn("Received message with no sender ID")
			return nil
		}

		// Extract content
		content := data.Content
		if content == "" {
			return nil
		}

		// Check authorization
		if !c.isAllowed(senderID) {
			c.log.Debug("Unauthorized sender", zap.String("sender_id", senderID))
			return nil
		}

		c.log.Info("Received QQ C2C message",
			zap.String("sender", senderID),
			zap.Int("length", len(content)))

		// Check for slash commands
		if c.commands.IsCommand(content) {
			c.handleCommand(senderID, senderID, content, data.ID)
			return nil
		}

		// Create bus message
		msg := &bus.Message{
			ID:        fmt.Sprintf("qq:%s", data.ID),
			ChannelID: "qq",
			SessionID: fmt.Sprintf("qq:%s", senderID),
			UserID:    senderID,
			Username:  senderID,
			Type:      bus.MessageTypeText,
			Content:   content,
			Timestamp: time.Now(),
		}

		// Send to bus
		if err := c.bus.SendInbound(msg); err != nil {
			c.log.Error("Failed to send inbound message", zap.Error(err))
		}

		return nil
	}
}

// handleGroupATMessage handles QQ group @ messages.
func (c *Channel) handleGroupATMessage() event.GroupATMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSGroupATMessageData) error {
		// Deduplication check
		if c.isDuplicate(data.ID) {
			return nil
		}

		// Extract sender info
		var senderID string
		if data.Author != nil && data.Author.ID != "" {
			senderID = data.Author.ID
		} else {
			c.log.Warn("Received group message with no sender ID")
			return nil
		}

		// Extract content
		content := data.Content
		if content == "" {
			return nil
		}

		groupID := data.GroupID

		// Check authorization
		if !c.isAllowed(senderID) && !c.isAllowed(groupID) {
			c.log.Debug("Unauthorized sender/group",
				zap.String("sender_id", senderID),
				zap.String("group_id", groupID))
			return nil
		}

		c.log.Info("Received QQ group AT message",
			zap.String("sender", senderID),
			zap.String("group", groupID),
			zap.Int("length", len(content)))

		// Check for slash commands
		if c.commands.IsCommand(content) {
			c.handleCommand(senderID, groupID, content, data.ID)
			return nil
		}

		// Create bus message (use groupID as chatID)
		msg := &bus.Message{
			ID:        fmt.Sprintf("qq:%s", data.ID),
			ChannelID: "qq",
			SessionID: fmt.Sprintf("qq:%s", groupID),
			UserID:    senderID,
			Username:  senderID,
			Type:      bus.MessageTypeText,
			Content:   content,
			Timestamp: time.Now(),
		}

		// Send to bus
		if err := c.bus.SendInbound(msg); err != nil {
			c.log.Error("Failed to send inbound message", zap.Error(err))
		}

		return nil
	}
}

// handleCommand processes a command message.
func (c *Channel) handleCommand(senderID, chatID, content, messageID string) {
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
		Channel:  "qq",
		ChatID:   chatID,
		UserID:   senderID,
		Username: senderID,
		Command:  cmdName,
		Args:     args,
		Metadata: map[string]string{
			"message_id": messageID,
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
		c.sendMessage(chatID, "❌ Command failed: "+err.Error())
		return
	}

	// Send response
	c.sendMessage(chatID, resp.Content)
}

// handleOutbound handles outbound messages from the bus.
func (c *Channel) handleOutbound(ctx context.Context, msg *bus.Message) error {
	return c.SendMessage(ctx, msg)
}

// SendMessage sends a message to QQ.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	// Extract chat ID from session ID (format: "qq:chat_id")
	chatID := msg.SessionID
	if len(chatID) > 3 && chatID[:3] == "qq:" {
		chatID = chatID[3:]
	}

	return c.sendMessage(chatID, msg.Content)
}

// sendMessage sends a message to a specific chat.
func (c *Channel) sendMessage(chatID, content string) error {
	if !c.running {
		return fmt.Errorf("qq bot not running")
	}

	// Create message
	msgToCreate := &dto.MessageToCreate{
		Content: content,
	}

	// Send C2C message
	_, err := c.api.PostC2CMessage(context.Background(), chatID, msgToCreate)
	if err != nil {
		c.log.Error("Failed to send C2C message", zap.Error(err))
		return fmt.Errorf("sending message: %w", err)
	}

	c.log.Debug("Sent QQ message", zap.String("chat_id", chatID))
	return nil
}

// isDuplicate checks if a message has been processed.
func (c *Channel) isDuplicate(messageID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.processedIDs[messageID] {
		return true
	}

	c.processedIDs[messageID] = true

	// Simple cleanup: limit map size
	if len(c.processedIDs) > 10000 {
		// Clear half
		count := 0
		for id := range c.processedIDs {
			if count >= 5000 {
				break
			}
			delete(c.processedIDs, id)
			count++
		}
	}

	return false
}

// isAllowed checks if a user/group is allowed to use the bot.
func (c *Channel) isAllowed(id string) bool {
	if len(c.config.AllowFrom) == 0 {
		return true
	}

	for _, allowed := range c.config.AllowFrom {
		if allowed == id || allowed == "*" {
			return true
		}
	}

	return false
}
