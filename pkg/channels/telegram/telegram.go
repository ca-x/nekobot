// Package telegram provides Telegram bot integration.
package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Channel implements the Telegram channel.
type Channel struct {
	log    *logger.Logger
	bus    bus.Bus // Use interface, not pointer to interface
	agent  *agent.Agent
	config *config.TelegramConfig

	bot    *tgbotapi.BotAPI
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new Telegram channel.
func New(log *logger.Logger, messageBus bus.Bus, ag *agent.Agent, cfg *config.TelegramConfig) (*Channel, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram token is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Channel{
		log:    log,
		bus:    messageBus,
		agent:  ag,
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "telegram"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "Telegram"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// Start starts the Telegram bot and begins listening for messages.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting Telegram channel")

	// Create bot
	bot, err := tgbotapi.NewBotAPI(c.config.Token)
	if err != nil {
		return fmt.Errorf("creating telegram bot: %w", err)
	}

	c.bot = bot
	c.bot.Debug = false

	c.log.Info("Telegram bot connected",
		zap.String("username", bot.Self.UserName))

	// Setup updates
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// Process updates
	for {
		select {
		case update := <-updates:
			c.handleUpdate(update)

		case <-ctx.Done():
			c.log.Info("Telegram channel stopping")
			bot.StopReceivingUpdates()
			return nil

		case <-c.ctx.Done():
			c.log.Info("Telegram channel stopping")
			bot.StopReceivingUpdates()
			return nil
		}
	}
}

// Stop stops the Telegram channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping Telegram channel")
	c.cancel()

	if c.bot != nil {
		c.bot.StopReceivingUpdates()
	}

	return nil
}

// SendMessage sends a message through Telegram.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	if c.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}

	// Parse chat ID from session ID (format: "telegram:123456")
	chatID, err := c.extractChatID(msg.SessionID)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}

	// Create message
	reply := tgbotapi.NewMessage(chatID, msg.Content)

	// Handle reply
	if msg.ReplyTo != "" {
		// Parse message ID
		if msgID, err := c.extractMessageID(msg.ReplyTo); err == nil {
			reply.ReplyToMessageID = msgID
		}
	}

	// Send message
	if _, err := c.bot.Send(reply); err != nil {
		return fmt.Errorf("sending telegram message: %w", err)
	}

	return nil
}

// handleUpdate processes a Telegram update.
func (c *Channel) handleUpdate(update tgbotapi.Update) {
	// Handle messages
	if update.Message != nil {
		c.handleMessage(update.Message)
		return
	}

	// Handle other update types as needed
}

// handleMessage processes an incoming message.
func (c *Channel) handleMessage(message *tgbotapi.Message) {
	// Check if user is allowed
	if !c.isUserAllowed(message.From.ID, message.Chat.ID) {
		c.log.Warn("Unauthorized access attempt",
			zap.Int64("user_id", message.From.ID),
			zap.String("username", message.From.UserName))
		return
	}

	// Ignore non-text messages for now
	if message.Text == "" {
		return
	}

	c.log.Info("Received Telegram message",
		zap.Int64("chat_id", message.Chat.ID),
		zap.String("from", message.From.UserName),
		zap.String("text", message.Text))

	// Create bus message
	busMsg := &bus.Message{
		ID:        fmt.Sprintf("telegram:%d", message.MessageID),
		ChannelID: c.ID(),
		SessionID: fmt.Sprintf("telegram:%d", message.Chat.ID),
		UserID:    fmt.Sprintf("%d", message.From.ID),
		Username:  message.From.UserName,
		Type:      bus.MessageTypeText,
		Content:   message.Text,
		Timestamp: time.Unix(int64(message.Date), 0),
	}

	if message.ReplyToMessage != nil {
		busMsg.ReplyTo = fmt.Sprintf("telegram:%d", message.ReplyToMessage.MessageID)
	}

	// Send to bus for processing
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Process with agent
	response, err := c.agent.Chat(ctx, message.Text)
	if err != nil {
		c.log.Error("Agent chat failed", zap.Error(err))

		// Send error message
		reply := tgbotapi.NewMessage(message.Chat.ID, "Sorry, I encountered an error processing your message.")
		c.bot.Send(reply)
		return
	}

	// Send response back
	reply := tgbotapi.NewMessage(message.Chat.ID, response)
	reply.ReplyToMessageID = message.MessageID

	if _, err := c.bot.Send(reply); err != nil {
		c.log.Error("Failed to send reply", zap.Error(err))
	}
}

// isUserAllowed checks if a user is allowed to use the bot.
func (c *Channel) isUserAllowed(userID, chatID int64) bool {
	// If no allow list is configured, allow all
	if len(c.config.AllowFrom) == 0 {
		return true
	}

	// Check allow list
	userIDStr := fmt.Sprintf("%d", userID)
	chatIDStr := fmt.Sprintf("%d", chatID)

	for _, allowed := range c.config.AllowFrom {
		if allowed == userIDStr || allowed == chatIDStr {
			return true
		}
	}

	return false
}

// extractChatID extracts the chat ID from a session ID.
func (c *Channel) extractChatID(sessionID string) (int64, error) {
	// Format: "telegram:123456"
	parts := strings.Split(sessionID, ":")
	if len(parts) != 2 || parts[0] != "telegram" {
		return 0, fmt.Errorf("invalid telegram session ID format")
	}

	var chatID int64
	if _, err := fmt.Sscanf(parts[1], "%d", &chatID); err != nil {
		return 0, fmt.Errorf("invalid chat ID: %w", err)
	}

	return chatID, nil
}

// extractMessageID extracts the message ID from a message reference.
func (c *Channel) extractMessageID(msgRef string) (int, error) {
	// Format: "telegram:123456"
	parts := strings.Split(msgRef, ":")
	if len(parts) != 2 || parts[0] != "telegram" {
		return 0, fmt.Errorf("invalid telegram message reference format")
	}

	var msgID int
	if _, err := fmt.Sscanf(parts[1], "%d", &msgID); err != nil {
		return 0, fmt.Errorf("invalid message ID: %w", err)
	}

	return msgID, nil
}
