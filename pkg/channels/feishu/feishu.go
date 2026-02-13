// Package feishu provides Feishu (Lark) channel implementation.
package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Channel implements Feishu channel as a WebSocket client.
type Channel struct {
	log      *logger.Logger
	config   config.FeishuConfig
	bus      bus.Bus
	commands *commands.Registry

	client   *lark.Client
	wsClient *larkws.Client
	mu       sync.Mutex
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewChannel creates a new Feishu channel.
func NewChannel(
	log *logger.Logger,
	cfg config.FeishuConfig,
	b bus.Bus,
	cmdRegistry *commands.Registry,
) (*Channel, error) {
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return nil, fmt.Errorf("feishu app_id and app_secret are required")
	}

	return &Channel{
		log:      log,
		config:   cfg,
		bus:      b,
		commands: cmdRegistry,
		client:   lark.NewClient(cfg.AppID, cfg.AppSecret),
		running:  false,
	}, nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "feishu"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "Feishu"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// Start starts the Feishu channel and connects to WebSocket.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting Feishu channel")

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Register outbound message handler
	c.bus.RegisterHandler("feishu", c.handleOutbound)

	// Create event dispatcher
	dispatcher := larkdispatcher.NewEventDispatcher(
		c.config.VerificationToken,
		c.config.EncryptKey,
	).OnP2MessageReceiveV1(c.handleMessageReceive)

	// Create WebSocket client
	c.mu.Lock()
	c.wsClient = larkws.NewClient(
		c.config.AppID,
		c.config.AppSecret,
		larkws.WithEventHandler(dispatcher),
	)
	wsClient := c.wsClient
	c.mu.Unlock()

	c.running = true

	// Start WebSocket in goroutine
	go func() {
		c.log.Info("Feishu WebSocket client starting")
		if err := wsClient.Start(c.ctx); err != nil {
			c.log.Error("Feishu WebSocket stopped", zap.Error(err))
		}
	}()

	c.log.Info("Feishu channel started")
	return nil
}

// Stop stops the Feishu channel.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping Feishu channel")

	c.running = false

	if c.cancel != nil {
		c.cancel()
	}

	// Unregister handler
	c.bus.UnregisterHandlers("feishu")

	c.mu.Lock()
	c.wsClient = nil
	c.mu.Unlock()

	c.log.Info("Feishu channel stopped")
	return nil
}

// handleMessageReceive handles incoming messages from Feishu.
func (c *Channel) handleMessageReceive(_ context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil
	}

	message := event.Event.Message
	sender := event.Event.Sender

	chatID := stringValue(message.ChatId)
	if chatID == "" {
		return nil
	}

	senderID := extractSenderID(sender)
	if senderID == "" {
		senderID = "unknown"
	}

	// Check authorization
	if !c.isAllowed(senderID) {
		c.log.Warn("Unauthorized user",
			zap.String("user_id", senderID))
		return nil
	}

	content := extractMessageContent(message)
	if content == "" {
		return nil
	}

	c.log.Info("Feishu message received",
		zap.String("sender_id", senderID),
		zap.String("chat_id", chatID))

	// Check for slash commands
	if c.commands.IsCommand(content) {
		c.handleCommand(message, sender, senderID, chatID, content)
		return nil
	}

	// Create bus message
	msg := &bus.Message{
		ID:        fmt.Sprintf("feishu:%s", stringValue(message.MessageId)),
		ChannelID: "feishu",
		SessionID: fmt.Sprintf("feishu:%s", chatID),
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

// handleCommand processes a command message.
func (c *Channel) handleCommand(message *larkim.EventMessage, sender *larkim.EventSender, senderID, chatID, content string) {
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
		Channel:  "feishu",
		ChatID:   chatID,
		UserID:   senderID,
		Username: senderID,
		Command:  cmdName,
		Args:     args,
		Metadata: map[string]string{
			"message_id": stringValue(message.MessageId),
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

// SendMessage sends a message to Feishu.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	// Extract chat ID from session ID (format: "feishu:chat_id")
	chatID := msg.SessionID
	if len(chatID) > 7 && chatID[:7] == "feishu:" {
		chatID = chatID[7:]
	}

	return c.sendMessage(chatID, msg.Content)
}

// sendMessage sends a message to a specific chat.
func (c *Channel) sendMessage(chatID, content string) error {
	if chatID == "" {
		return fmt.Errorf("chat ID is empty")
	}

	// Prepare text payload
	payload, err := json.Marshal(map[string]string{"text": content})
	if err != nil {
		return fmt.Errorf("marshaling content: %w", err)
	}

	// Build request
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(larkim.MsgTypeText).
			Content(string(payload)).
			Uuid(fmt.Sprintf("nekobot-%d", time.Now().UnixNano())).
			Build()).
		Build()

	// Send message
	resp, err := c.client.Im.V1.Message.Create(context.Background(), req)
	if err != nil {
		return fmt.Errorf("sending message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu api error: code=%d msg=%s", resp.Code, resp.Msg)
	}

	c.log.Debug("Sent Feishu message", zap.String("chat_id", chatID))
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

// extractSenderID extracts sender ID from event sender.
func extractSenderID(sender *larkim.EventSender) string {
	if sender == nil || sender.SenderId == nil {
		return ""
	}

	if sender.SenderId.UserId != nil && *sender.SenderId.UserId != "" {
		return *sender.SenderId.UserId
	}
	if sender.SenderId.OpenId != nil && *sender.SenderId.OpenId != "" {
		return *sender.SenderId.OpenId
	}
	if sender.SenderId.UnionId != nil && *sender.SenderId.UnionId != "" {
		return *sender.SenderId.UnionId
	}

	return ""
}

// extractMessageContent extracts text content from message.
func extractMessageContent(message *larkim.EventMessage) string {
	if message == nil || message.Content == nil || *message.Content == "" {
		return ""
	}

	if message.MessageType != nil && *message.MessageType == larkim.MsgTypeText {
		var textPayload struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(*message.Content), &textPayload); err == nil {
			return textPayload.Text
		}
	}

	return *message.Content
}

// stringValue safely extracts string from pointer.
func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
