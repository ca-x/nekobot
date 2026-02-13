// Package googlechat provides Google Chat channel implementation.
package googlechat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/api/chat/v1"
	"google.golang.org/api/option"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Channel implements Google Chat channel.
type Channel struct {
	log      *logger.Logger
	config   config.GoogleChatConfig
	bus      bus.Bus
	commands *commands.Registry

	service    *chat.Service
	httpClient *http.Client
	mu         sync.RWMutex
	running    bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewChannel creates a new Google Chat channel.
func NewChannel(
	log *logger.Logger,
	cfg config.GoogleChatConfig,
	b bus.Bus,
	cmdRegistry *commands.Registry,
) (*Channel, error) {
	if cfg.ProjectID == "" {
		return nil, fmt.Errorf("googlechat project_id is required")
	}

	return &Channel{
		log:      log,
		config:   cfg,
		bus:      b,
		commands: cmdRegistry,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		running: false,
	}, nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "googlechat"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "GoogleChat"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// Start starts the Google Chat channel.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting Google Chat channel",
		zap.String("project_id", c.config.ProjectID))

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Register outbound message handler
	c.bus.RegisterHandler("googlechat", c.handleOutbound)

	// Initialize Google Chat service if credentials are provided
	if c.config.CredentialsFile != "" {
		if err := c.initService(); err != nil {
			c.log.Warn("Failed to initialize Google Chat service, webhook mode only", zap.Error(err))
			// Don't return error, allow webhook-only mode
		}
	}

	c.running = true
	c.log.Info("Google Chat channel started")
	return nil
}

// Stop stops the Google Chat channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping Google Chat channel")

	c.running = false

	if c.cancel != nil {
		c.cancel()
	}

	// Unregister handler
	c.bus.UnregisterHandlers("googlechat")

	c.mu.Lock()
	c.service = nil
	c.mu.Unlock()

	c.log.Info("Google Chat channel stopped")
	return nil
}

// initService initializes the Google Chat API service.
func (c *Channel) initService() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.service != nil {
		return nil
	}

	// Read credentials file
	credBytes, err := io.ReadFile(c.config.CredentialsFile)
	if err != nil {
		return fmt.Errorf("reading credentials file: %w", err)
	}

	service, err := chat.NewService(c.ctx, option.WithCredentialsJSON(credBytes))
	if err != nil {
		return fmt.Errorf("creating chat service: %w", err)
	}

	c.service = service
	c.log.Info("Google Chat service initialized")
	return nil
}

// HandleWebhook handles incoming webhook events from Google Chat.
// This should be called from an external HTTP endpoint.
func (c *Channel) HandleWebhook(ctx context.Context, event *chat.DeprecatedEvent) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}

	senderID := event.User.Name
	spaceName := event.Space.Name
	content := event.Message.Text

	// Check authorization
	if !c.isAllowed(senderID) {
		c.log.Warn("Unauthorized sender",
			zap.String("sender_name", senderID))
		return nil
	}

	c.log.Info("Google Chat message received",
		zap.String("sender", event.User.DisplayName),
		zap.String("space", spaceName))

	// Check for slash commands
	if c.commands.IsCommand(content) {
		c.handleCommand(senderID, event.User.DisplayName, spaceName, content, event)
		return nil
	}

	// Create bus message
	msg := &bus.Message{
		ID:        fmt.Sprintf("googlechat:%s", event.Message.Name),
		ChannelID: "googlechat",
		SessionID: fmt.Sprintf("googlechat:%s", spaceName),
		UserID:    senderID,
		Username:  event.User.DisplayName,
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

// handleCommand processes a command message.
func (c *Channel) handleCommand(senderID, senderName, spaceName, content string, event *chat.DeprecatedEvent) {
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
		Channel:  "googlechat",
		ChatID:   spaceName,
		UserID:   senderID,
		Username: senderName,
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
		// Try to extract webhook URL from event metadata
		// For now, just log the error
		return
	}

	// Send response
	// Try to send via API if service is available, otherwise log
	outMsg := &bus.Message{
		ChannelID: "googlechat",
		SessionID: fmt.Sprintf("googlechat:%s", spaceName),
		Content:   resp.Content,
		Timestamp: time.Now(),
	}
	c.SendMessage(context.Background(), outMsg)
}

// handleOutbound handles outbound messages from the bus.
func (c *Channel) handleOutbound(ctx context.Context, msg *bus.Message) error {
	return c.SendMessage(ctx, msg)
}

// SendMessage sends a message to Google Chat.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	// Extract space name from session ID (format: "googlechat:space_name")
	spaceName := msg.SessionID
	if len(spaceName) > 11 && spaceName[:11] == "googlechat:" {
		spaceName = spaceName[11:]
	}

	// Try to use API first if service is available
	c.mu.RLock()
	service := c.service
	c.mu.RUnlock()

	if service != nil {
		return c.sendViaAPI(spaceName, msg.Content)
	}

	// If no API service, we need a webhook URL
	// This should be stored in message metadata
	return fmt.Errorf("google chat service not initialized and no webhook URL provided")
}

// sendViaAPI sends a message using the Google Chat API.
func (c *Channel) sendViaAPI(spaceName, content string) error {
	c.mu.RLock()
	service := c.service
	c.mu.RUnlock()

	if service == nil {
		return fmt.Errorf("google chat service not initialized")
	}

	chatMsg := &chat.Message{
		Text: content,
	}

	_, err := service.Spaces.Messages.Create(spaceName, chatMsg).Do()
	if err != nil {
		return fmt.Errorf("sending message: %w", err)
	}

	c.log.Debug("Sent Google Chat message via API", zap.String("space", spaceName))
	return nil
}

// SendViaWebhook sends a message using a webhook URL.
func (c *Channel) SendViaWebhook(webhookURL, content string) error {
	payload := map[string]interface{}{
		"text": content,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	c.log.Debug("Sent Google Chat message via webhook")
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
