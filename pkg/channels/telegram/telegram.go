// Package telegram provides Telegram bot integration.
package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
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

	bot      *tgbotapi.BotAPI
	stopOnce sync.Once
	ctx      context.Context
	cancel   context.CancelFunc
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

	// Keep HTTP timeout longer than long-poll timeout to avoid periodic forced reconnects.
	httpClient := &http.Client{Timeout: 75 * time.Second}
	if c.config.Proxy != "" {
		proxyURL, err := url.Parse(c.config.Proxy)
		if err != nil {
			return fmt.Errorf("parsing telegram proxy: %w", err)
		}
		httpClient.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		c.log.Info("Telegram proxy enabled", zap.String("proxy", proxyURL.String()))
	}

	// Create bot
	bot, err := tgbotapi.NewBotAPIWithClient(c.config.Token, tgbotapi.APIEndpoint, httpClient)
	if err != nil {
		return fmt.Errorf("creating telegram bot: %w", err)
	}

	c.bot = bot
	c.stopOnce = sync.Once{}
	c.bot.Debug = false

	c.log.Info("Telegram bot connected",
		zap.String("username", bot.Self.UserName))
	c.syncSlashCommands()

	// Setup updates
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 50

	updates := bot.GetUpdatesChan(u)

	// Process updates
	for {
		select {
		case update := <-updates:
			c.handleUpdate(update)

		case <-ctx.Done():
			c.log.Info("Telegram channel stopping")
			c.stopReceivingUpdates()
			return nil

		case <-c.ctx.Done():
			c.log.Info("Telegram channel stopping")
			c.stopReceivingUpdates()
			return nil
		}
	}
}

// Stop stops the Telegram channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping Telegram channel")
	c.cancel()
	c.stopReceivingUpdates()

	return nil
}

func (c *Channel) stopReceivingUpdates() {
	if c.bot == nil {
		return
	}
	c.stopOnce.Do(func() {
		c.bot.StopReceivingUpdates()
	})
}

func (c *Channel) requestTimeout() time.Duration {
	if c.config.TimeoutSeconds > 0 {
		return time.Duration(c.config.TimeoutSeconds) * time.Second
	}
	return 60 * time.Second
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
		msg := *update.Message
		go c.handleMessage(&msg)
		return
	}

	// Handle other update types as needed
}

func (c *Channel) syncSlashCommands() {
	if c.bot == nil || c.commands == nil {
		return
	}

	cmds := c.commands.List()
	telegramCmds := make([]tgbotapi.BotCommand, 0, len(cmds))
	seen := make(map[string]struct{})

	for _, cmd := range cmds {
		name := sanitizeTelegramCommandName(cmd.Name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}

		desc := strings.TrimSpace(cmd.Description)
		if desc == "" {
			desc = strings.TrimSpace(cmd.Usage)
		}
		if desc == "" {
			desc = "Command"
		}
		if len(desc) > 256 {
			desc = desc[:256]
		}

		telegramCmds = append(telegramCmds, tgbotapi.BotCommand{
			Command:     name,
			Description: desc,
		})
	}

	if len(telegramCmds) == 0 {
		return
	}

	// Telegram supports at most 100 commands.
	sort.Slice(telegramCmds, func(i, j int) bool {
		return telegramCmds[i].Command < telegramCmds[j].Command
	})
	if len(telegramCmds) > 100 {
		telegramCmds = telegramCmds[:100]
	}

	if _, err := c.bot.Request(tgbotapi.NewSetMyCommands(telegramCmds...)); err != nil {
		c.log.Warn("Failed to sync Telegram slash commands (default scope)", zap.Error(err))
		return
	}
	if _, err := c.bot.Request(
		tgbotapi.NewSetMyCommandsWithScope(
			tgbotapi.NewBotCommandScopeAllPrivateChats(),
			telegramCmds...,
		),
	); err != nil {
		c.log.Warn("Failed to sync Telegram slash commands (private scope)", zap.Error(err))
	}

	c.log.Info("Synced Telegram slash commands", zap.Int("count", len(telegramCmds)))
}

func sanitizeTelegramCommandName(name string) string {
	normalized := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(name), "/"))
	if normalized == "" {
		return ""
	}

	var b strings.Builder
	lastUnderscore := false
	for _, r := range normalized {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastUnderscore = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		case r == '-' || r == '_':
			if b.Len() > 0 && !lastUnderscore {
				b.WriteRune('_')
				lastUnderscore = true
			}
		default:
			// Ignore unsupported characters.
		}

		if b.Len() >= 32 {
			break
		}
	}

	result := strings.Trim(b.String(), "_")
	if result == "" {
		return ""
	}
	return result
}

// handleMessage processes an incoming message.
func (c *Channel) handleMessage(message *tgbotapi.Message) {
	// Check if user is allowed
	if !c.isUserAllowed(message.From.ID, message.Chat.ID, message.From.UserName) {
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
	ctx, cancel := context.WithTimeout(context.Background(), c.requestTimeout())
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

	ctx, cancel := context.WithTimeout(context.Background(), c.requestTimeout())
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

	ctx, cancel := context.WithTimeout(context.Background(), c.requestTimeout())
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

	if _, err := c.bot.Send(reply); err != nil {
		c.log.Error("Failed to send command response", zap.Error(err))
	}
}

// isUserAllowed checks if a user is allowed to use the bot.
func (c *Channel) isUserAllowed(userID, chatID int64, username string) bool {
	// If no allow list is configured, allow all
	if len(c.config.AllowFrom) == 0 {
		return true
	}

	// Check allow list
	userIDStr := fmt.Sprintf("%d", userID)
	chatIDStr := fmt.Sprintf("%d", chatID)
	username = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(username)), "@")

	for _, allowed := range c.config.AllowFrom {
		normalizedAllowed := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(allowed)), "@")
		if normalizedAllowed == userIDStr || normalizedAllowed == chatIDStr {
			return true
		}
		if username != "" && normalizedAllowed == username {
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
