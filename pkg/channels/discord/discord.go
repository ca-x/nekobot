// Package discord provides Discord channel implementation.
package discord

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"

	"nekobot/pkg/bus"
	"nekobot/pkg/config"
	"nekobot/pkg/logger"
)

// Channel implements Discord channel.
type Channel struct {
	log     *logger.Logger
	config  config.DiscordConfig
	bus     bus.Bus
	session *discordgo.Session
	running bool
}

// NewChannel creates a new Discord channel.
func NewChannel(log *logger.Logger, cfg config.DiscordConfig, b bus.Bus) (*Channel, error) {
	// Create Discord session
	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("creating discord session: %w", err)
	}

	return &Channel{
		log:     log,
		config:  cfg,
		bus:     b,
		session: session,
		running: false,
	}, nil
}

// Start starts the Discord bot.
func (c *Channel) Start(ctx context.Context) error {
	c.log.Info("Starting Discord channel")

	// Register message handler
	c.session.AddHandler(c.handleMessage)

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

	// Listen for outbound messages
	go c.listenForOutbound(ctx)

	return nil
}

// Stop stops the Discord bot.
func (c *Channel) Stop(ctx context.Context) error {
	c.log.Info("Stopping Discord channel")
	c.running = false

	if c.session != nil {
		if err := c.session.Close(); err != nil {
			return fmt.Errorf("closing discord session: %w", err)
		}
	}

	return nil
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return "discord"
}

// IsRunning returns whether the channel is running.
func (c *Channel) IsRunning() bool {
	return c.running
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

	// Create inbound message
	msg := bus.InboundMessage{
		Channel:   "discord",
		ChatID:    m.ChannelID,
		UserID:    m.Author.ID,
		Username:  m.Author.Username,
		Content:   m.Content,
		Timestamp: time.Now(),
	}

	// Send to bus
	ctx := context.Background()
	if err := c.bus.SendInbound(ctx, msg); err != nil {
		c.log.Error("Failed to send inbound message",
			zap.Error(err))
	}
}

// listenForOutbound listens for outbound messages from the bus.
func (c *Channel) listenForOutbound(ctx context.Context) {
	outboundChan, err := c.bus.SubscribeOutbound(ctx, "discord")
	if err != nil {
		c.log.Error("Failed to subscribe to outbound", zap.Error(err))
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-outboundChan:
			if !ok {
				return
			}

			if err := c.sendMessage(ctx, msg); err != nil {
				c.log.Error("Failed to send message",
					zap.String("channel_id", msg.ChatID),
					zap.Error(err))
			}
		}
	}
}

// sendMessage sends a message to Discord.
func (c *Channel) sendMessage(ctx context.Context, msg bus.OutboundMessage) error {
	if c.session == nil {
		return fmt.Errorf("session not initialized")
	}

	// Send message
	_, err := c.session.ChannelMessageSend(msg.ChatID, msg.Content)
	if err != nil {
		return fmt.Errorf("sending discord message: %w", err)
	}

	c.log.Debug("Sent Discord message",
		zap.String("channel_id", msg.ChatID),
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
