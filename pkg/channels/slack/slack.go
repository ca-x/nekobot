// Package slack provides Slack channel implementation.
package slack

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Channel implements Slack channel using Socket Mode.
type Channel struct {
	log          *logger.Logger
	config       config.SlackConfig
	bus          bus.Bus
	commands     *commands.Registry
	api          *slack.Client
	socketClient *socketmode.Client
	botUserID    string
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
	pendingAcks  sync.Map // chat_id -> message reference
}

// NewChannel creates a new Slack channel.
func NewChannel(log *logger.Logger, cfg config.SlackConfig, b bus.Bus, cmdRegistry *commands.Registry) (*Channel, error) {
	if cfg.BotToken == "" || cfg.AppToken == "" {
		return nil, fmt.Errorf("slack bot_token and app_token are required")
	}

	// Create API client
	api := slack.New(
		cfg.BotToken,
		slack.OptionAppLevelToken(cfg.AppToken),
	)

	// Create Socket Mode client
	socketClient := socketmode.New(api)

	return &Channel{
		log:          log,
		config:       cfg,
		bus:          b,
		commands:     cmdRegistry,
		api:          api,
		socketClient: socketClient,
		running:      false,
	}, nil
}

// Start starts the Slack bot.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting Slack channel (Socket Mode)")

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Test authentication
	authResp, err := c.api.AuthTest()
	if err != nil {
		return fmt.Errorf("slack auth test failed: %w", err)
	}
	c.botUserID = authResp.UserID

	c.log.Info("Slack bot connected",
		zap.String("bot_user_id", c.botUserID),
		zap.String("team", authResp.Team))

	// Register outbound message handler
	c.bus.RegisterHandler("slack", c.handleOutbound)

	// Start event loop
	go c.eventLoop()

	// Run Socket Mode client
	go func() {
		if err := c.socketClient.RunContext(c.ctx); err != nil {
			if c.ctx.Err() == nil {
				c.log.Error("Socket Mode connection error", zap.Error(err))
			}
		}
	}()

	c.running = true
	c.log.Info("Slack channel started")

	return nil
}

// Stop stops the Slack bot.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping Slack channel")

	if c.cancel != nil {
		c.cancel()
	}

	// Unregister handler
	c.bus.UnregisterHandlers("slack")

	c.running = false
	c.log.Info("Slack channel stopped")
	return nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "slack"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "Slack"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// IsRunning returns whether the channel is running.
func (c *Channel) IsRunning() bool {
	return c.running
}

// eventLoop processes Socket Mode events.
func (c *Channel) eventLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case evt := <-c.socketClient.Events:
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				c.handleEventsAPI(evt)
			case socketmode.EventTypeSlashCommand:
				c.handleSlashCommand(evt)
			case socketmode.EventTypeInteractive:
				c.handleInteractive(evt)
			}
		}
	}
}

// handleEventsAPI handles Events API events.
func (c *Channel) handleEventsAPI(evt socketmode.Event) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		c.log.Warn("Failed to parse Events API event")
		c.socketClient.Ack(*evt.Request)
		return
	}

	// Acknowledge the event
	c.socketClient.Ack(*evt.Request)

	// Handle inner event
	switch ev := eventsAPIEvent.InnerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		c.handleMessageEvent(ev)
	case *slackevents.AppMentionEvent:
		c.handleAppMentionEvent(ev)
	}
}

// handleMessageEvent handles message events.
func (c *Channel) handleMessageEvent(ev *slackevents.MessageEvent) {
	// Ignore bot messages
	if ev.BotID != "" || ev.User == c.botUserID {
		return
	}

	// Check if user is allowed
	if !c.isAllowed(ev.User) {
		c.log.Warn("Unauthorized user",
			zap.String("user_id", ev.User))
		return
	}

	// Determine chat ID (channel_id or channel_id:thread_ts)
	sessionID := fmt.Sprintf("slack:%s", ev.Channel)
	if ev.ThreadTimeStamp != "" {
		sessionID = fmt.Sprintf("slack:%s:%s", ev.Channel, ev.ThreadTimeStamp)
	}

	// Create inbound message
	msg := &bus.Message{
		ID:        fmt.Sprintf("slack:%s", ev.TimeStamp),
		ChannelID: "slack",
		SessionID: sessionID,
		UserID:    ev.User,
		Username:  ev.User, // Slack uses user ID
		Type:      bus.MessageTypeText,
		Content:   ev.Text,
		Timestamp: time.Now(),
	}

	// Send to bus
	if err := c.bus.SendInbound(msg); err != nil {
		c.log.Error("Failed to send inbound message", zap.Error(err))
	}
}

// handleAppMentionEvent handles app mention events.
func (c *Channel) handleAppMentionEvent(ev *slackevents.AppMentionEvent) {
	// Check if user is allowed
	if !c.isAllowed(ev.User) {
		c.log.Warn("Unauthorized user",
			zap.String("user_id", ev.User))
		return
	}

	// Determine session ID
	sessionID := fmt.Sprintf("slack:%s", ev.Channel)
	if ev.ThreadTimeStamp != "" {
		sessionID = fmt.Sprintf("slack:%s:%s", ev.Channel, ev.ThreadTimeStamp)
	}

	// Remove bot mention from text
	text := strings.TrimSpace(strings.Replace(ev.Text, fmt.Sprintf("<@%s>", c.botUserID), "", 1))

	// Create inbound message
	msg := &bus.Message{
		ID:        fmt.Sprintf("slack:%s", ev.TimeStamp),
		ChannelID: "slack",
		SessionID: sessionID,
		UserID:    ev.User,
		Username:  ev.User,
		Type:      bus.MessageTypeText,
		Content:   text,
		Timestamp: time.Now(),
	}

	// Send to bus
	if err := c.bus.SendInbound(msg); err != nil {
		c.log.Error("Failed to send inbound message", zap.Error(err))
	}
}

// handleSlashCommand handles slash commands.
func (c *Channel) handleSlashCommand(evt socketmode.Event) {
	cmd, ok := evt.Data.(slack.SlashCommand)
	if !ok {
		c.socketClient.Ack(*evt.Request)
		return
	}

	// Acknowledge the event
	c.socketClient.Ack(*evt.Request)

	c.log.Debug("Received slash command",
		zap.String("command", cmd.Command),
		zap.String("text", cmd.Text))

	// Remove leading / from command name
	cmdName := strings.TrimPrefix(cmd.Command, "/")

	// Get command from registry
	command, exists := c.commands.Get(cmdName)
	if !exists {
		c.log.Debug("Unknown slash command", zap.String("command", cmdName))
		return
	}

	// Create command request
	req := commands.CommandRequest{
		Channel:  "slack",
		ChatID:   cmd.ChannelID,
		UserID:   cmd.UserID,
		Username: cmd.UserName,
		Command:  cmdName,
		Args:     cmd.Text,
		Metadata: map[string]string{
			"channel_name": cmd.ChannelName,
			"team_id":      cmd.TeamID,
			"team_domain":  cmd.TeamDomain,
			"trigger_id":   cmd.TriggerID,
		},
	}

	// Execute command
	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	resp, err := command.Handler(ctx, req)
	if err != nil {
		c.log.Error("Slash command execution failed",
			zap.String("command", cmdName),
			zap.Error(err))

		// Send error as ephemeral message
		c.api.PostEphemeral(cmd.ChannelID, cmd.UserID,
			slack.MsgOptionText("❌ Command failed: "+err.Error(), false))
		return
	}

	// Send response
	opts := []slack.MsgOption{
		slack.MsgOptionText(resp.Content, false),
	}

	if resp.Ephemeral {
		// Send as ephemeral message (only visible to user)
		c.api.PostEphemeral(cmd.ChannelID, cmd.UserID, opts...)
	} else {
		// Send as regular message
		c.api.PostMessage(cmd.ChannelID, opts...)
	}
}

// handleInteractive handles interactive components.
func (c *Channel) handleInteractive(evt socketmode.Event) {
	callback, ok := evt.Data.(slack.InteractionCallback)
	if !ok {
		c.socketClient.Ack(*evt.Request)
		return
	}

	c.socketClient.Ack(*evt.Request)

	c.log.Debug("Received interactive callback",
		zap.String("type", string(callback.Type)))

	// TODO: Handle interactive components
}

// handleOutbound handles outbound messages from the bus.
func (c *Channel) handleOutbound(ctx context.Context, msg *bus.Message) error {
	return c.SendMessage(ctx, msg)
}

// SendMessage sends a message to Slack.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	// Parse session ID (format: "slack:channel_id" or "slack:channel_id:thread_ts")
	channelID, threadTS := c.parseSessionID(msg.SessionID)
	if channelID == "" {
		return fmt.Errorf("invalid session ID: %s", msg.SessionID)
	}

	// Build message options
	opts := []slack.MsgOption{
		slack.MsgOptionText(msg.Content, false),
	}

	if threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(threadTS))
	}

	// Send message
	_, _, err := c.api.PostMessageContext(ctx, channelID, opts...)
	if err != nil {
		return fmt.Errorf("sending slack message: %w", err)
	}

	c.log.Debug("Sent Slack message",
		zap.String("channel_id", channelID),
		zap.String("thread_ts", threadTS))

	return nil
}

// parseSessionID parses session ID into channel ID and optional thread timestamp.
func (c *Channel) parseSessionID(sessionID string) (string, string) {
	// Format: "slack:channel_id" or "slack:channel_id:thread_ts"
	if !strings.HasPrefix(sessionID, "slack:") {
		return "", ""
	}

	parts := strings.SplitN(sessionID[6:], ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return parts[0], ""
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
