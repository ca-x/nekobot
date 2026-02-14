// Package serverchan provides ServerChan Bot channel implementation.
package serverchan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

const (
	baseURL        = "https://bot-go.apijia.cn"
	pollTimeout    = 5  // seconds
	pollInterval   = 1  // seconds between polls
	requestTimeout = 10 // seconds for HTTP requests
)

// Update represents an update from ServerChan Bot.
type Update struct {
	UpdateID int     `json:"update_id"`
	Message  Message `json:"message"`
}

// Message represents a message in an update.
type Message struct {
	MessageID int         `json:"message_id"`
	ChatID    int64       `json:"chat_id,omitempty"` // legacy flat payload
	Text      string      `json:"text"`
	Chat      MessageChat `json:"chat,omitempty"`
	From      MessageFrom `json:"from,omitempty"`
	Date      int64       `json:"date,omitempty"`
}

type MessageChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type,omitempty"`
}

type MessageFrom struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// Response represents a ServerChan API response.
type Response struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

// Channel implements ServerChan Bot channel.
type Channel struct {
	log      *logger.Logger
	config   config.ServerChanConfig
	bus      bus.Bus
	commands *commands.Registry

	client  *http.Client
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex

	lastUpdateID int
}

// NewChannel creates a new ServerChan channel.
func NewChannel(
	log *logger.Logger,
	cfg config.ServerChanConfig,
	b bus.Bus,
	cmdRegistry *commands.Registry,
) (*Channel, error) {
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("serverchan bot_token is required")
	}

	return &Channel{
		log:      log,
		config:   cfg,
		bus:      b,
		commands: cmdRegistry,
		client: &http.Client{
			Timeout: requestTimeout * time.Second,
		},
		running:      false,
		lastUpdateID: 0,
	}, nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "serverchan"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "ServerChan"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// Start starts the ServerChan channel.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting ServerChan channel")

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Register outbound message handler
	c.bus.RegisterHandler("serverchan", c.handleOutbound)

	// Verify bot
	if err := c.verifyBot(); err != nil {
		return fmt.Errorf("verifying bot: %w", err)
	}

	c.mu.Lock()
	c.running = true
	c.mu.Unlock()

	// Start polling for updates
	go c.pollUpdates()

	c.log.Info("ServerChan channel started")
	return nil
}

// Stop stops the ServerChan channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping ServerChan channel")

	c.mu.Lock()
	c.running = false
	c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	// Unregister handler
	c.bus.UnregisterHandlers("serverchan")

	c.log.Info("ServerChan channel stopped")
	return nil
}

// verifyBot verifies the bot token by calling getMe.
func (c *Channel) verifyBot() error {
	url := fmt.Sprintf("%s/bot%s/getMe", baseURL, c.config.BotToken)

	resp, err := c.client.Get(url)
	if err != nil {
		return fmt.Errorf("calling getMe: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("getMe returned status %d", resp.StatusCode)
	}

	c.log.Info("ServerChan bot verified")
	return nil
}

// pollUpdates polls for new updates from ServerChan.
func (c *Channel) pollUpdates() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.mu.Lock()
			running := c.running
			c.mu.Unlock()

			if !running {
				return
			}

			if err := c.getUpdates(); err != nil {
				c.log.Error("Failed to get updates", zap.Error(err))
			}

			time.Sleep(pollInterval * time.Second)
		}
	}
}

// getUpdates retrieves new updates from ServerChan.
func (c *Channel) getUpdates() error {
	url := fmt.Sprintf("%s/bot%s/getUpdates?timeout=%d&offset=%d",
		baseURL, c.config.BotToken, pollTimeout, c.lastUpdateID+1)

	resp, err := c.client.Get(url)
	if err != nil {
		return fmt.Errorf("getting updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("getUpdates returned status %d", resp.StatusCode)
	}

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if !response.OK {
		return fmt.Errorf("api returned ok=false")
	}

	// Process updates
	for _, update := range response.Result {
		c.processUpdate(update)
		if update.UpdateID > c.lastUpdateID {
			c.lastUpdateID = update.UpdateID
		}
	}

	return nil
}

// processUpdate processes a single update.
func (c *Channel) processUpdate(update Update) {
	msg := update.Message

	chatIDNum := msg.Chat.ID
	if chatIDNum == 0 {
		chatIDNum = msg.ChatID
	}
	if chatIDNum == 0 {
		c.log.Warn("Invalid ServerChan update: missing chat ID",
			zap.Int("update_id", update.UpdateID),
			zap.Int("message_id", msg.MessageID))
		return
	}

	userIDNum := msg.From.ID
	if userIDNum == 0 {
		userIDNum = chatIDNum
	}

	chatID := fmt.Sprintf("%d", chatIDNum)
	userID := fmt.Sprintf("%d", userIDNum)
	username := strings.TrimSpace(msg.From.Username)
	if username == "" {
		username = strings.TrimSpace(strings.TrimSpace(msg.From.FirstName + " " + msg.From.LastName))
	}
	if username == "" {
		username = "User" + userID
	}

	// Check authorization
	if !c.isAllowed(userID, chatID) {
		c.log.Warn("Unauthorized user",
			zap.String("user_id", userID),
			zap.String("chat_id", chatID),
			zap.String("username", username))
		_ = c.sendMessage(chatID, "❌ 你不在 allow_from 白名单中，暂时不能使用这个 agent。", false)
		return
	}

	content := msg.Text
	if content == "" {
		return
	}

	c.log.Info("ServerChan message received",
		zap.String("user_id", userID),
		zap.String("chat_id", chatID),
		zap.String("username", username))

	// Check for slash commands
	if c.commands.IsCommand(content) {
		c.handleCommand(userID, username, chatID, content)
		return
	}

	// Create bus message
	busMsg := &bus.Message{
		ID:        fmt.Sprintf("serverchan:%d", msg.MessageID),
		ChannelID: "serverchan",
		SessionID: fmt.Sprintf("serverchan:%s", chatID),
		UserID:    userID,
		Username:  username,
		Type:      bus.MessageTypeText,
		Content:   content,
		Timestamp: time.Now(),
	}

	// Send to bus
	if err := c.bus.SendInbound(busMsg); err != nil {
		c.log.Error("Failed to send inbound message", zap.Error(err))
	}
}

// handleCommand processes a command message.
func (c *Channel) handleCommand(userID, username, chatID, content string) {
	cmdName, args := c.commands.Parse(content)

	cmd, exists := c.commands.Get(cmdName)
	if !exists {
		c.log.Debug("Unknown command", zap.String("command", cmdName))
		return
	}

	c.log.Info("Executing command",
		zap.String("command", cmdName),
		zap.String("user", userID))

	// Create command request
	req := commands.CommandRequest{
		Channel:  "serverchan",
		ChatID:   chatID,
		UserID:   userID,
		Username: username,
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
		c.sendMessage(chatID, "❌ Command failed: "+err.Error(), false)
		return
	}

	// Send response
	c.sendMessage(chatID, resp.Content, false)
}

// handleOutbound handles outbound messages from the bus.
func (c *Channel) handleOutbound(ctx context.Context, msg *bus.Message) error {
	return c.SendMessage(ctx, msg)
}

// SendMessage sends a message to ServerChan.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	// Extract chat ID from session ID (format: "serverchan:chat_id")
	chatID := msg.SessionID
	if len(chatID) > 11 && chatID[:11] == "serverchan:" {
		chatID = chatID[11:]
	}

	return c.sendMessage(chatID, msg.Content, false)
}

// sendMessage sends a message to a specific chat.
func (c *Channel) sendMessage(chatID, text string, silent bool) error {
	if chatID == "" {
		return fmt.Errorf("chat ID is empty")
	}
	chatIDNum, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID %q: %w", chatID, err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", baseURL, c.config.BotToken)

	// Build request payload
	payload := map[string]interface{}{
		"chat_id":    chatIDNum,
		"text":       text,
		"parse_mode": "markdown",
		"silent":     silent,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	// Send POST request
	resp, err := c.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("sending message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sendMessage returned status %d: %s", resp.StatusCode, string(body))
	}

	c.log.Debug("Sent ServerChan message", zap.String("chat_id", chatID))
	return nil
}

// isAllowed checks if a user is allowed to use the bot.
func (c *Channel) isAllowed(userID, chatID string) bool {
	if len(c.config.AllowFrom) == 0 {
		return true
	}

	for _, allowed := range c.config.AllowFrom {
		if allowed == userID || allowed == chatID || allowed == "*" {
			return true
		}
	}

	return false
}
