// Package discord provides Discord channel implementation.
package discord

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/commands"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Channel implements Discord channel.
type Channel struct {
	log      *logger.Logger
	config   config.DiscordConfig
	bus      bus.Bus
	commands *commands.Registry
	session  *discordgo.Session
	running  bool
}

// NewChannel creates a new Discord channel.
func NewChannel(log *logger.Logger, cfg config.DiscordConfig, b bus.Bus, cmdRegistry *commands.Registry) (*Channel, error) {
	// Create Discord session
	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("creating discord session: %w", err)
	}

	return &Channel{
		log:      log,
		config:   cfg,
		bus:      b,
		commands: cmdRegistry,
		session:  session,
		running:  false,
	}, nil
}

// ID returns the channel identifier.
func (c *Channel) ID() string {
	return "discord"
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "Discord"
}

// IsEnabled returns whether the channel is enabled.
func (c *Channel) IsEnabled() bool {
	return c.config.Enabled
}

// Start starts the Discord bot.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting Discord channel")

	// Register message handler
	c.session.AddHandler(c.handleMessage)

	// Register outbound message handler
	c.bus.RegisterHandler("discord", c.handleOutbound)

	// Set intents
	c.session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	// Open WebSocket connection
	if err := c.session.Open(); err != nil {
		return fmt.Errorf("opening discord connection: %w", err)
	}

	c.running = true

	// Get bot user info
	botUser, err := c.session.User("@me")
	if err != nil {
		c.log.Warn("Failed to get bot user", zap.Error(err))
	} else {
		c.log.Info("Discord bot connected",
			zap.String("username", botUser.Username),
			zap.String("user_id", botUser.ID))
	}

	return nil
}

// Stop stops the Discord bot.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping Discord channel")
	c.running = false

	// Unregister handler
	c.bus.UnregisterHandlers("discord")

	if c.session != nil {
		if err := c.session.Close(); err != nil {
			return fmt.Errorf("closing discord session: %w", err)
		}
	}

	return nil
}

// handleMessage handles incoming Discord messages.
func (c *Channel) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore bot's own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check if user is allowed
	if !c.isAllowed(m.Author.ID) {
		c.log.Warn("Unauthorized user",
			zap.String("user_id", m.Author.ID),
			zap.String("username", m.Author.Username))
		return
	}

	// Check if it's a command
	if c.commands.IsCommand(m.Content) {
		c.handleCommand(s, m)
		return
	}

	// Create inbound message
	msg := &bus.Message{
		ID:        fmt.Sprintf("discord:%s", m.ID),
		ChannelID: "discord",
		SessionID: fmt.Sprintf("discord:%s", m.ChannelID),
		UserID:    m.Author.ID,
		Username:  m.Author.Username,
		Type:      bus.MessageTypeText,
		Content:   m.Content,
		Timestamp: time.Now(),
	}

	// Send to bus
	if err := c.bus.SendInbound(msg); err != nil {
		c.log.Error("Failed to send inbound message", zap.Error(err))
	}
}

// handleCommand processes a command message.
func (c *Channel) handleCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	cmdName, args := c.commands.Parse(m.Content)

	cmd, exists := c.commands.Get(cmdName)
	if !exists {
		c.log.Debug("Unknown command", zap.String("command", cmdName))
		return
	}

	c.log.Info("Executing command",
		zap.String("command", cmdName),
		zap.String("user", m.Author.Username))

	// Create command request
	req := commands.CommandRequest{
		Channel:  "discord",
		ChatID:   m.ChannelID,
		UserID:   m.Author.ID,
		Username: m.Author.Username,
		Command:  cmdName,
		Args:     args,
		Metadata: map[string]string{
			"message_id": m.ID,
			"guild_id":   m.GuildID,
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

		s.ChannelMessageSend(m.ChannelID, "❌ Command failed: "+err.Error())
		return
	}

	// Send response
	if _, err := s.ChannelMessageSend(m.ChannelID, resp.Content); err != nil {
		c.log.Error("Failed to send command response", zap.Error(err))
	}
}

// handleOutbound handles outbound messages from the bus.
func (c *Channel) handleOutbound(ctx context.Context, msg *bus.Message) error {
	return c.SendMessage(ctx, msg)
}

// SendMessage sends a message to Discord.
func (c *Channel) SendMessage(ctx context.Context, msg *bus.Message) error {
	if c.session == nil {
		return fmt.Errorf("session not initialized")
	}

	// Extract channel ID from session ID (format: "discord:channel_id")
	channelID := msg.SessionID
	if len(channelID) > 8 && channelID[:8] == "discord:" {
		channelID = channelID[8:]
	}

	// Send message
	_, err := c.session.ChannelMessageSend(channelID, msg.Content)
	if err != nil {
		return fmt.Errorf("sending discord message: %w", err)
	}

	c.log.Debug("Sent Discord message",
		zap.String("channel_id", channelID),
		zap.Int("length", len(msg.Content)))

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
