// Package dingtalk provides DingTalk channel implementation.
package dingtalk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Channel implements DingTalk channel as a Stream client.
type Channel struct {
	log      *logger.Logger
	config   config.DingTalkConfig
	bus      bus.Bus
	commands *commands.Registry

	streamClient *client.StreamClient
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc

	// Store session webhooks for replying
	sessionWebhooks sync.Map // chatID -> sessionWebhook
}

// NewChannel creates a new DingTalk channel.
func NewChannel(
	log *logger.Logger,
	cfg config.DingTalkConfig,
	b bus.Bus,
	cmdRegistry *commands.Registry,
) (*Channel, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("dingtalk client_id and client_secret are required")
	}

	return &Channel{
		log:      log,
		config:   cfg,
		bus:      b,
		commands: cmdRegistry,
		running:  false,
	}, nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "dingtalk"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "DingTalk"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// Start starts the DingTalk channel in Stream mode.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting DingTalk channel (Stream Mode)")

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Register outbound message handler
	c.bus.RegisterHandler("dingtalk", c.handleOutbound)

	// Create credential config
	cred := client.NewAppCredentialConfig(c.config.ClientID, c.config.ClientSecret)

	// Create stream client
	c.streamClient = client.NewStreamClient(
		client.WithAppCredential(cred),
		client.WithAutoReconnect(true),
	)

	// Register chatbot callback handler
	c.streamClient.RegisterChatBotCallbackRouter(c.onMessageReceived)

	// Start stream client
	go func() {
		c.log.Info("DingTalk stream client starting")
		if err := c.streamClient.Start(c.ctx); err != nil {
			c.log.Error("DingTalk stream client stopped", zap.Error(err))
		}
	}()

	c.running = true
	c.log.Info("DingTalk channel started")
	return nil
}

// Stop stops the DingTalk channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping DingTalk channel")

	c.running = false

	if c.cancel != nil {
		c.cancel()
	}

	// Unregister handler
	c.bus.UnregisterHandlers("dingtalk")

	if c.streamClient != nil {
		c.streamClient.Close()
	}

	c.log.Info("DingTalk channel stopped")
	return nil
}

// onMessageReceived handles incoming messages from DingTalk.
func (c *Channel) onMessageReceived(ctx context.Context, data *chatbot.BotCallbackDataModel) ([]byte, error) {
	// Extract message content
	content := data.Text.Content
	if content == "" {
		// Try from Content map
		if contentMap, ok := data.Content.(map[string]interface{}); ok {
			if textContent, ok := contentMap["content"].(string); ok {
				content = textContent
			}
		}
	}

	if content == "" {
		return nil, nil // Ignore empty messages
	}

	senderID := data.SenderStaffId
	senderNick := data.SenderNick
	chatID := senderID

	// For group chats
	if data.ConversationType != "1" {
		chatID = data.ConversationId
	}

	// Check authorization
	if !c.isAllowed(senderID) {
		c.log.Debug("Unauthorized sender",
			zap.String("sender_id", senderID),
			zap.String("sender_nick", senderNick))
		return nil, nil
	}

	// Store session webhook for replies
	c.sessionWebhooks.Store(chatID, data.SessionWebhook)

	c.log.Info("DingTalk message received",
		zap.String("sender_nick", senderNick),
		zap.String("sender_id", senderID),
		zap.String("chat_id", chatID))

	// Check for slash commands
	if c.commands.IsCommand(content) {
		c.handleCommand(senderID, senderNick, chatID, content, data.SessionWebhook)
		return nil, nil
	}

	// Create bus message
	msg := &bus.Message{
		ID:        fmt.Sprintf("dingtalk:%s", chatID),
		ChannelID: "dingtalk",
		SessionID: fmt.Sprintf("dingtalk:%s", chatID),
		UserID:    senderID,
		Username:  senderNick,
		Type:      bus.MessageTypeText,
		Content:   content,
		Timestamp: time.Now(),
	}

	// Send to bus
	if err := c.bus.SendInbound(msg); err != nil {
		c.log.Error("Failed to send inbound message", zap.Error(err))
	}

	return nil, nil
}

// handleCommand processes a command message.
func (c *Channel) handleCommand(senderID, senderNick, chatID, content, sessionWebhook string) {
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
		Channel:  "dingtalk",
		ChatID:   chatID,
		UserID:   senderID,
		Username: senderNick,
		Command:  cmdName,
		Args:     args,
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
		c.sendReply(sessionWebhook, "❌ Command failed: "+err.Error())
		return
	}

	// Send response
	c.sendReply(sessionWebhook, resp.Content)
}

// handleOutbound handles outbound messages from the bus.
func (c *Channel) handleOutbound(ctx context.Context, msg *bus.Message) error {
	return c.SendMessage(ctx, msg)
}

// SendMessage sends a message to DingTalk.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	// Extract chat ID from session ID (format: "dingtalk:chat_id")
	chatID := msg.SessionID
	if len(chatID) > 9 && chatID[:9] == "dingtalk:" {
		chatID = chatID[9:]
	}

	// Get session webhook
	sessionWebhookRaw, ok := c.sessionWebhooks.Load(chatID)
	if !ok {
		return fmt.Errorf("no session_webhook found for chat %s", chatID)
	}

	sessionWebhook, ok := sessionWebhookRaw.(string)
	if !ok {
		return fmt.Errorf("invalid session_webhook type for chat %s", chatID)
	}

	return c.sendReply(sessionWebhook, msg.Content)
}

// sendReply sends a reply using session webhook.
func (c *Channel) sendReply(sessionWebhook, content string) error {
	replier := chatbot.NewChatbotReplier()

	// Send markdown formatted reply
	err := replier.SimpleReplyMarkdown(
		context.Background(),
		sessionWebhook,
		[]byte("nanobot"),
		[]byte(content),
	)

	if err != nil {
		c.log.Error("Failed to send DingTalk reply", zap.Error(err))
		return fmt.Errorf("sending reply: %w", err)
	}

	c.log.Debug("Sent DingTalk message", zap.String("webhook", sessionWebhook[:20]+"..."))
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
