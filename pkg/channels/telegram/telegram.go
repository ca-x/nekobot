// Package telegram provides Telegram bot integration.
package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"nekobot/pkg/agent"
	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
	"nekobot/pkg/transcription"
)

// simpleSession is a simple session implementation for telegram messages.
type simpleSession struct {
	messages []agent.Message
	mu       sync.RWMutex
}

func (s *simpleSession) GetMessages() []agent.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.messages
}

func (s *simpleSession) AddMessage(msg agent.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}

// Channel implements the Telegram channel.
type Channel struct {
	log         *logger.Logger
	bus         bus.Bus // Use interface, not pointer to interface
	agent       *agent.Agent
	commands    *commands.Registry
	config      *config.TelegramConfig
	transcriber transcription.Transcriber

	bot    *tgbotapi.BotAPI
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new Telegram channel.
func New(
	log *logger.Logger,
	messageBus bus.Bus,
	ag *agent.Agent,
	cmdRegistry *commands.Registry,
	cfg *config.TelegramConfig,
	transcriber transcription.Transcriber,
) (*Channel, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram token is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Channel{
		log:         log,
		bus:         messageBus,
		agent:       ag,
		commands:    cmdRegistry,
		config:      cfg,
		transcriber: transcriber,
		ctx:         ctx,
		cancel:      cancel,
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

	content := strings.TrimSpace(message.Text)
	msgType := bus.MessageTypeText

	// Support voice/audio messages via Whisper transcription.
	if content == "" && c.transcriber != nil {
		transcribed, ok := c.tryTranscribeAudio(message)
		if ok {
			content = transcribed
			msgType = bus.MessageTypeAudio
		}
	}
	if content == "" {
		return
	}

	c.log.Info("Received Telegram message",
		zap.Int64("chat_id", message.Chat.ID),
		zap.String("from", message.From.UserName),
		zap.String("text", content))

	// Check if it's a command
	if c.commands.IsCommand(content) {
		if msgType == bus.MessageTypeText {
			c.handleCommand(message)
			return
		}

		// For transcribed voice command text, run command path with synthetic message text.
		clone := *message
		clone.Text = content
		c.handleCommand(&clone)
		return
	}

	// Create bus message
	busMsg := &bus.Message{
		ID:        fmt.Sprintf("telegram:%d", message.MessageID),
		ChannelID: c.ID(),
		SessionID: fmt.Sprintf("telegram:%d", message.Chat.ID),
		UserID:    fmt.Sprintf("%d", message.From.ID),
		Username:  message.From.UserName,
		Type:      msgType,
		Content:   content,
		Timestamp: time.Unix(int64(message.Date), 0),
	}

	if message.ReplyToMessage != nil {
		busMsg.ReplyTo = fmt.Sprintf("telegram:%d", message.ReplyToMessage.MessageID)
	}

	// Send to bus for processing
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create a simple session for this message
	sess := &simpleSession{
		messages: make([]agent.Message, 0),
	}

	// Process with agent
	response, err := c.agent.Chat(ctx, sess, content)
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

func (c *Channel) tryTranscribeAudio(message *tgbotapi.Message) (string, bool) {
	fileID := ""
	filename := "voice.ogg"
	switch {
	case message.Voice != nil:
		fileID = message.Voice.FileID
		filename = "voice.ogg"
	case message.Audio != nil:
		fileID = message.Audio.FileID
		if message.Audio.FileName != "" {
			filename = message.Audio.FileName
		}
	case message.Document != nil:
		fileID = message.Document.FileID
		if message.Document.FileName != "" {
			filename = message.Document.FileName
		}
	default:
		return "", false
	}
	if fileID == "" {
		return "", false
	}

	audioBytes, err := c.downloadFile(fileID)
	if err != nil {
		c.log.Warn("Failed to download Telegram audio for transcription", zap.Error(err))
		return "", false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	text, err := c.transcriber.Transcribe(ctx, audioBytes, filename)
	if err != nil {
		c.log.Warn("Telegram audio transcription failed", zap.Error(err))
		return "", false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	return text, true
}

func (c *Channel) downloadFile(fileID string) ([]byte, error) {
	url, err := c.bot.GetFileDirectURL(fileID)
	if err != nil {
		return nil, fmt.Errorf("resolving file URL: %w", err)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("downloading file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 20*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("reading file body: %w", err)
	}
	return data, nil
}

// handleCommand processes a command message.
func (c *Channel) handleCommand(message *tgbotapi.Message) {
	cmdName, args := c.commands.Parse(message.Text)

	cmd, exists := c.commands.Get(cmdName)
	if !exists {
		c.log.Debug("Unknown command", zap.String("command", cmdName))
		return
	}

	c.log.Info("Executing command",
		zap.String("command", cmdName),
		zap.String("user", message.From.UserName))

	// Create command request
	req := commands.CommandRequest{
		Channel:  c.ID(),
		ChatID:   fmt.Sprintf("%d", message.Chat.ID),
		UserID:   fmt.Sprintf("%d", message.From.ID),
		Username: message.From.UserName,
		Command:  cmdName,
		Args:     args,
		Metadata: map[string]string{
			"message_id": fmt.Sprintf("%d", message.MessageID),
			"chat_type":  message.Chat.Type,
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

		reply := tgbotapi.NewMessage(message.Chat.ID, "❌ Command failed: "+err.Error())
		c.bot.Send(reply)
		return
	}

	// Send response
	reply := tgbotapi.NewMessage(message.Chat.ID, resp.Content)
	reply.ParseMode = "Markdown"

	if _, err := c.bot.Send(reply); err != nil {
		c.log.Error("Failed to send command response", zap.Error(err))
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
